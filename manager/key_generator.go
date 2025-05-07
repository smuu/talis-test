package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

type keyType int

const (
	ed25519Type keyType = iota
	secp256k1Type
)

type keyPair struct {
	PrivateKey crypto.PrivKey
	PublicKey  crypto.PubKey
}

func (k *keyPair) ToJSON() (string, error) {
	keyJSON := struct {
		Type  string `json:"type"`
		Value struct {
			Address string `json:"address"`
			PubKey  struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
			PrivKey struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"priv_key"`
		} `json:"value"`
	}{
		Type: "tendermint/PrivKeyEd25519",
		Value: struct {
			Address string `json:"address"`
			PubKey  struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
			PrivKey struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"priv_key"`
		}{
			Address: k.PublicKey.Address().String(),
			PubKey: struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			}{
				Type:  "tendermint/PubKeyEd25519",
				Value: fmt.Sprintf("%X", k.PublicKey.Bytes()),
			},
			PrivKey: struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			}{
				Type:  "tendermint/PrivKeyEd25519",
				Value: fmt.Sprintf("%X", k.PrivateKey.Bytes()),
			},
		},
	}

	jsonBytes, err := json.Marshal(keyJSON)
	if err != nil {
		return "", fmt.Errorf("failed to marshal key JSON: %w", err)
	}
	return string(jsonBytes), nil
}

type keyGenerator struct {
	random *rand.Rand
}

func newKeyGenerator(seed int64) *keyGenerator {
	return &keyGenerator{
		random: rand.New(rand.NewSource(seed)), //nolint:gosec
	}
}

func (g *keyGenerator) Generate(keyType keyType) *keyPair {
	seed := make([]byte, ed25519.SeedSize)

	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}

	var privKey crypto.PrivKey
	switch keyType {
	case secp256k1Type:
		privKey = secp256k1.GenPrivKeySecp256k1(seed)
	case ed25519Type:
		privKey = ed25519.GenPrivKeyFromSecret(seed)
	default:
		panic("KeyType not supported")
	}

	return &keyPair{
		PrivateKey: privKey,
		PublicKey:  privKey.PubKey(),
	}
}
