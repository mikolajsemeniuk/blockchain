package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"time"
)

type Handler struct {
	mux   *http.ServeMux
	chain *Chain
}

func NewHandler(c *Chain) *Handler {
	mux := http.NewServeMux()
	hdl := &Handler{mux: mux, chain: c}

	mux.HandleFunc("GET /", hdl.Elements)
	mux.HandleFunc("GET /docs", hdl.OpenAPI)
	mux.HandleFunc("GET /wallet", hdl.CreateWallet)
	mux.HandleFunc("POST /mine", hdl.Mine)
	mux.HandleFunc("GET /balance", hdl.FindBalance)
	mux.HandleFunc("GET /transaction", hdl.FindTransaction)
	mux.HandleFunc("POST /transaction", hdl.CreateTransaction)

	return hdl
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	prv, pub, err := GenerateWallet()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: change later
	wallet := map[string]interface{}{
		"private": hex.EncodeToString(prv.D.Bytes()),
		"public": map[string]string{
			"x": hex.EncodeToString(pub.X.Bytes()),
			"y": hex.EncodeToString(pub.Y.Bytes()),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(wallet); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	pub := r.URL.Query().Get("pub")

	key, err := hex.DecodeString(pub)
	if err != nil {
		http.Error(w, "invalid miner public key", http.StatusBadRequest)
		return
	}

	block, err := h.chain.MineBlockFromPool(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "Block mined successfully",
		"blockHash": fmt.Sprintf("%x", block.Hash),
	})
}

func (h *Handler) FindBalance(w http.ResponseWriter, r *http.Request) {
	pubKey := r.URL.Query().Get("pubkey")
	if pubKey == "" {
		http.Error(w, "Missing pubkey parameter", http.StatusBadRequest)
		return
	}

	key, err := hex.DecodeString(pubKey)
	if err != nil {
		http.Error(w, "invalid public key", http.StatusBadRequest)
		return
	}

	balance, err := h.chain.set.Balance(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := map[string]interface{}{
		"pubkey":  pubKey,
		"balance": balance,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type CreateTransactionRequest struct {
	SenderPubKey   string `json:"senderPubKey"`
	SenderPrivKey  string `json:"senderPrivKey"`
	ReceiverPubKey string `json:"receiverPubKey"`
	Amount         int    `json:"amount"`
	Fee            int    `json:"fee"`
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	senderPub, err := hex.DecodeString(req.SenderPubKey)
	if err != nil {
		http.Error(w, "invalid sender public key", http.StatusBadRequest)
		return
	}

	receiverPub, err := hex.DecodeString(req.ReceiverPubKey)
	if err != nil {
		http.Error(w, "invalid receiver public key", http.StatusBadRequest)
		return
	}

	total := req.Amount + req.Fee
	balance, err := h.chain.set.Balance(senderPub)
	if err != nil {
		http.Error(w, "error checking balance", http.StatusInternalServerError)
		return
	}

	if balance < total {
		http.Error(w, "insufficient funds", http.StatusBadRequest)
		return
	}

	if err := h.chain.set.Remove("genesis", 0); err != nil {
		http.Error(w, "failed to remove UTXO", http.StatusInternalServerError)
		return
	}

	input := Input{
		PreviousKey: []byte("genesis"),
		OutputIndex: 0,
		PubKey:      senderPub,
	}

	outputs := []Output{{Value: req.Amount, PubKey: receiverPub}}
	change := balance - total
	if change > 0 {
		outputs = append(outputs, Output{
			Value:  change,
			PubKey: senderPub,
		})
	}

	tx := Transaction{
		Version:   1,
		Inputs:    []Input{input},
		Outputs:   outputs,
		Timestamp: time.Now().Unix(),
		Fee:       req.Fee,
	}

	dBytes, err := hex.DecodeString(req.SenderPrivKey)
	if err != nil {
		http.Error(w, "invalid sender private key", http.StatusBadRequest)
		return
	}

	// Używamy krzywej P256 – można zamienić na secp256k1, jeśli dostępna.
	d := new(big.Int).SetBytes(dBytes)
	curve := elliptic.P256()
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}

	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	temp := tx
	temp.Inputs[0].Signature = nil
	data, err := json.Marshal(temp)
	if err != nil {
		http.Error(w, "failed to marshal transaction for signing", http.StatusInternalServerError)
		return
	}

	hash := sha256.Sum256(data)
	rInt, sInt, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		http.Error(w, "failed to sign transaction", http.StatusInternalServerError)
		return
	}

	signature := append(rInt.Bytes(), sInt.Bytes()...)
	tx.Inputs[0].Signature = signature

	if err := h.chain.pool.Add(tx); err != nil {
		http.Error(w, "failed to add transaction to pool", http.StatusInternalServerError)
		return
	}

	txID, err := tx.Key()
	if err != nil {
		http.Error(w, "failed to compute transaction key", http.StatusInternalServerError)
		return
	}

	for idx, out := range outputs {
		spent := Spend{Value: out.Value, PubKey: out.PubKey}
		if err := h.chain.set.Add(txID, idx, spent); err != nil {
			http.Error(w, "failed to update UTXO set", http.StatusInternalServerError)
			return
		}
	}

	res := map[string]string{
		"txid":    txID,
		"message": "Transaction created and added to pool",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) FindTransaction(w http.ResponseWriter, r *http.Request) {
	txid := r.URL.Query().Get("txid")
	if txid == "" {
		http.Error(w, "Missing txid parameter", http.StatusBadRequest)
		return
	}

	tx, err := h.chain.FindTransactionByID(txid)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tx)
		return
	}

	// 2. If not found in chain, check the pool
	for _, pendingTx := range h.chain.pool.List() {
		pendingKey, _ := pendingTx.Key()
		if pendingKey == txid {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(pendingTx)
			return
		}
	}

	// 3. Not found anywhere
	http.Error(w, "Transaction not found", http.StatusNotFound)
}

func (h *Handler) Elements(w http.ResponseWriter, _ *http.Request) {
	template.Must(template.New("elements").Parse(elements)).Execute(w, "./docs")
}

func (h *Handler) OpenAPI(w http.ResponseWriter, _ *http.Request) {
	template.Must(template.New("docs").Parse(docs)).Execute(w, nil)
}
