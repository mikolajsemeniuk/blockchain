package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

func GenerateWallet() (ecdsa.PrivateKey, ecdsa.PublicKey, error) {
	// Using P256 curve. Bitcoin uses secp256k1, but here we use P256 for simplicity.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ecdsa.PrivateKey{}, ecdsa.PublicKey{}, err
	}

	return *key, key.PublicKey, nil
}
