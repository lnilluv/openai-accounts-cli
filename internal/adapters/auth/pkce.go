package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const PKCEChallengeMethodS256 = "S256"

type PKCEPair struct {
	Verifier  string
	Challenge string
}

func NewPKCEPair() (PKCEPair, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return PKCEPair{}, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return PKCEPair{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
