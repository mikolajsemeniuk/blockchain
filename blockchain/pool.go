package blockchain

import (
	"errors"
	"sync"
)

type Pool struct {
	txs map[string]Transaction
	mu  sync.RWMutex
}

func NewPool() *Pool {
	return &Pool{txs: map[string]Transaction{}}
}

// Use for mining new block.
func (p *Pool) List() []Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txs := make([]Transaction, 0, len(p.txs))
	for _, tx := range p.txs {
		txs = append(txs, tx)
	}

	return txs
}

// Use for adding new transaction to be approved.
func (p *Pool) Add(t Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	out, err := t.Serialize()
	if err != nil {
		return err
	}

	key := string(out)
	if _, exists := p.txs[key]; exists {
		return errors.New("transaction already exists in the pool")
	}

	p.txs[key] = t

	return nil
}

// Use in case of adding new block.
func (p *Pool) Remove(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.txs, key)
}
