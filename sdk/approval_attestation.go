package codex

import "errors"

// AttestationGenerateParams are parameters for attestation/generate.
type AttestationGenerateParams struct{}

// AttestationGenerateResponse contains an opaque client attestation token.
type AttestationGenerateResponse struct {
	Token string `json:"token"`
}

func (r AttestationGenerateResponse) validate() error {
	if r.Token == "" {
		return errors.New("missing token")
	}
	return nil
}
