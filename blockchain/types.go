package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
)

type Transaction struct {
	Version   int
	Inputs    []Input
	Outputs   []Output
	Timestamp int64
	Fee       int
}

func (t *Transaction) Key() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	key := hex.EncodeToString(hash[:])

	return key, nil
}

func (t *Transaction) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(t); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t *Transaction) Deserialize(data []byte) error {
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(t); err != nil {
		return err
	}

	return nil
}

type Input struct {
	PreviousKey []byte
	OutputIndex int
	Signature   []byte
	PubKey      []byte
}

type Output struct {
	Value  int
	PubKey []byte
}

type Spend struct {
	Value  int
	PubKey []byte
}
