package blockchain

import (
	"fmt"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	blocksBucket  = "blocks"   // We'll store blocks by their hash
	chainstateKey = "chainTip" // We'll store the hash of the latest block
)

type Chain struct {
	tip    []byte
	blocks *leveldb.DB
	pool   *Pool
	set    *Set
}

// NewBlockchain creates a new Blockchain. If there's no existing chain in DB, it
// creates a genesis block. Otherwise, it loads the tip from DB.
func NewChain() (*Chain, error) {
	pool := NewPool()

	set, err := NewSet("utxo.db")
	if err != nil {
		return nil, err
	}

	blocks, err := leveldb.OpenFile("blockchain.db", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open blockchain db: %v", err)
	}

	// Check if we already have a tip stored
	tip, err := blocks.Get([]byte(chainstateKey), nil)
	if err == leveldb.ErrNotFound {
		// If not found, create a genesis block
		genesis := NewGenesisBlock()
		serialized, err := genesis.Serialize()
		if err != nil {
			return nil, err
		}

		// Store the genesis block with key=genesis.Hash
		if err := blocks.Put(genesis.Hash, serialized, nil); err != nil {
			return nil, err
		}

		// Update the tip
		if err := blocks.Put([]byte(chainstateKey), genesis.Hash, nil); err != nil {
			return nil, err
		}

		return &Chain{tip: tip, blocks: blocks, pool: pool, set: set}, nil
	}

	if err != nil {
		return nil, err
	}

	// If chainstateKey was found, we load it as tip
	return &Chain{tip: tip, blocks: blocks, pool: pool, set: set}, nil
}

// Close closes the underlying DB.
func (c *Chain) Close() error {
	return c.blocks.Close()
}

func (c *Chain) MineBlockFromPool(minerPubKey []byte) (*Block, error) {
	// 1. Gather transactions from the pool
	txs := c.pool.List()

	// 2. Add coinbase transaction
	coinbase := Transaction{
		Version: 1,
		Inputs:  []Input{},
		Outputs: []Output{
			// example fixed reward
			{Value: 50, PubKey: minerPubKey},
		},
		Timestamp: time.Now().Unix(),
		Fee:       0,
	}

	txs = append([]Transaction{coinbase}, txs...)

	// 3. Mine
	newBlock, err := c.MineBlock(txs)
	if err != nil {
		return nil, err
	}

	// 4. Remove from pool the transactions that were included
	//    (besides coinbase)
	for _, tx := range txs[1:] {
		key, _ := tx.Key()
		c.pool.Remove(key)
	}

	// 5. Also update your UTXO set with new outputs, etc.
	// (This is needed so that balances remain consistent.)
	for _, tx := range newBlock.Transactions {
		// Compute the transaction’s unique ID (hash)
		txID, err := tx.Key()
		if err != nil {
			return nil, fmt.Errorf("failed to compute txID: %v", err)
		}

		// (a) Add each output to UTXO
		for idx, out := range tx.Outputs {
			spent := Spend{Value: out.Value, PubKey: out.PubKey}

			if err := c.set.Add(txID, idx, spent); err != nil {
				return nil, fmt.Errorf("failed to add UTXO: %v", err)
			}
		}

		// (b) Remove inputs from UTXO (they are now spent)
		//     Coinbase TX will have no real inputs to remove, so it’s fine.
		for _, in := range tx.Inputs {
			// `PreviousKey` is presumably the TXID string
			// `OutputIndex` is the index in that previous TX
			if err := c.set.Remove(string(in.PreviousKey), in.OutputIndex); err != nil {
				// For coinbase, or if an input was previously removed, might return an error.
				// You can ignore or handle that if needed.
			}
		}
	}

	return newBlock, nil
}

func (c *Chain) MineBlock(transactions []Transaction) (*Block, error) {
	// Get the latest block
	data, err := c.blocks.Get(c.tip, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tip block: %v", err)
	}

	var block Block
	if err := block.Deserialize(data); err != nil {
		return nil, err
	}

	// Build a new block
	created := &Block{
		Index:        block.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: transactions,
		PrevHash:     block.Hash,
	}

	// Run proof of work
	pow := NewProofOfWork(created)
	hash, nonce := pow.Run()
	created.Hash = hash
	created.Nonce = nonce

	// Store it in DB
	serialized, err := created.Serialize()
	if err != nil {
		return nil, err
	}
	if err := c.blocks.Put(created.Hash, serialized, nil); err != nil {
		return nil, err
	}
	// Update the tip
	if err := c.blocks.Put([]byte(chainstateKey), created.Hash, nil); err != nil {
		return nil, err
	}

	c.tip = created.Hash
	return created, nil
}

func (c *Chain) FindTransactionByID(txid string) (*Transaction, error) {
	currentHash := c.tip

	for {
		// Fetch block by its hash
		data, err := c.blocks.Get(currentHash, nil)
		if err != nil {
			return nil, fmt.Errorf("unable to get block from DB: %v", err)
		}

		var block Block
		if err := block.Deserialize(data); err != nil {
			return nil, fmt.Errorf("failed to deserialize block: %v", err)
		}

		// Search each transaction in this block
		for _, tx := range block.Transactions {
			k, _ := tx.Key() // the Key() method returns the string representation of the TX hash
			if k == txid {
				return &tx, nil
			}
		}

		// If we’ve reached the genesis block (no prev hash), stop.
		if len(block.PrevHash) == 0 {
			break
		}
		currentHash = block.PrevHash
	}

	return nil, fmt.Errorf("transaction not found in chain")
}
