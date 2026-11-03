package protocol

import (
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// AttestationGenerateParams are parameters for attestation/generate.
type AttestationGenerateParams struct{}

// The request carrier owns object admission. Keep the public empty record
// method-free, and delegate native errors to its original representation.
type attestationParams struct {
	value AttestationGenerateParams
}

func (p *attestationParams) UnmarshalJSON(data []byte) error {
	if err := jsondecode.Unmarshal(data, &p.value); err != nil {
		return err
	}
	return validateInboundObjectFields(data, nil, nil)
}

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
