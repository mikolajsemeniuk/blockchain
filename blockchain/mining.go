package blockchain

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
)

const Difficulty = 3 // leading zeros required: e.g. "000..."

type ProofOfWork struct {
	block  *Block
	target string
}

func NewProofOfWork(b *Block) *ProofOfWork {
	// Our "target" is simply a string with Difficulty # of zeros
	target := strings.Repeat("0", Difficulty)
	return &ProofOfWork{b, target}
}

// Run loops to find a nonce that yields an acceptable hash
func (pow *ProofOfWork) Run() ([]byte, int) {
	var hash [32]byte
	nonce := 0
	for nonce < math.MaxInt32 {
		data := pow.prepareData(nonce)
		hash = sha256.Sum256(data)
		// Check if we have enough leading zeroes
		if strings.HasPrefix(fmt.Sprintf("%x", hash), pow.target) {
			break
		}
		nonce++
	}
	return hash[:], nonce
}

func (pow *ProofOfWork) prepareData(nonce int) []byte {
	b := pow.block
	return bytes.Join([][]byte{
		b.PrevHash,
		b.HashTransactions(),
		[]byte(fmt.Sprintf("%d", b.Timestamp)),
		[]byte(fmt.Sprintf("%d", nonce)),
	}, []byte{})
}
