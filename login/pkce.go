package login

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const (
	pkceVerifierBytes = 32
	stateBytes        = 16
)

type PKCE struct {
	Verifier  string
	Challenge string
}

func GeneratePKCE() (PKCE, error) {
	verifierBytes := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(verifierBytes); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}

func GenerateState() (string, error) {
	state := make([]byte, stateBytes)
	if _, err := rand.Read(state); err != nil {
		return "", err
	}
	return hex.EncodeToString(state), nil
}
