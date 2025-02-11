package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
)

type Set struct {
	store *leveldb.DB
}

func NewSet(path string) (*Set, error) {
	store, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %v", err)
	}

	return &Set{store: store}, nil
}

func (u *Set) Close() error {
	return u.store.Close()
}

func (s *Set) Add(txID string, outputIndex int, utxo Spend) error {
	key := fmt.Sprintf("%s:%d", txID, outputIndex)
	value, err := json.Marshal(utxo)
	if err != nil {
		return fmt.Errorf("failed to marshal UTXO: %v", err)
	}

	return s.store.Put([]byte(key), value, nil)
}

func (u *Set) Find(txID string, outputIndex int) (*Spend, error) {
	key := fmt.Sprintf("%s:%d", txID, outputIndex)
	value, err := u.store.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil, fmt.Errorf("UTXO not found: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO: %v", err)
	}

	var utxo Spend
	if err := json.Unmarshal(value, &utxo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal UTXO: %v", err)
	}

	return &utxo, nil
}

func (u *Set) Remove(txID string, outputIndex int) error {
	key := fmt.Sprintf("%s:%d", txID, outputIndex)
	return u.store.Delete([]byte(key), nil)
}

func (u *Set) Balance(pubKey []byte) (int, error) {
	iter := u.store.NewIterator(nil, nil)
	defer iter.Release()

	balance := 0
	for iter.Next() {
		var utxo Spend
		if err := json.Unmarshal(iter.Value(), &utxo); err != nil {
			return 0, fmt.Errorf("failed to unmarshal UTXO: %v", err)
		}

		if string(utxo.PubKey) == string(pubKey) {
			balance += utxo.Value
		}
	}

	if err := iter.Error(); err != nil {
		return 0, fmt.Errorf("iterator error: %v", err)
	}

	return balance, nil
}
