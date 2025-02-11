package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"
)

type Block struct {
	Index        int
	Timestamp    int64
	Transactions []Transaction
	PrevHash     []byte
	Hash         []byte
	Nonce        int
}

func (b *Block) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (b *Block) Deserialize(data []byte) error {
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(b); err != nil {
		return err
	}

	return nil
}

// HashTransactions is a naive approach to "Merkle root" – in practice, you would build a proper Merkle tree.
// For demonstration, we’ll just hash all transactions’ JSON together.
func (b *Block) HashTransactions() []byte {
	// TODO: Implement merkle root
	var all []byte
	for _, tx := range b.Transactions {
		out, _ := json.Marshal(tx)
		all = append(all, out...)
	}

	hash := sha256.Sum256(all)
	return hash[:]
}

func NewGenesisBlock() *Block {
	tx := Transaction{
		Version: 1,
		Inputs:  []Input{},
		Outputs: []Output{{Value: 100, PubKey: []byte("genesis-reward")}},
		Fee:     0,
	}

	block := &Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []Transaction{tx},
		PrevHash:     []byte{},
	}

	headers := bytes.Join([][]byte{
		block.PrevHash,
		block.HashTransactions(),
		[]byte(fmt.Sprintf("%d", block.Timestamp)),
		[]byte(fmt.Sprintf("%d", block.Nonce)),
	}, []byte{})

	hash := sha256.Sum256(headers)

	block.Hash = hash[:]

	return block
}
