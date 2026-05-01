package codex

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func validateOutboundBase64Field(field, value string) error {
	if err := validateBase64Syntax(value); err != nil {
		return invalidParamsError("%s must be valid base64: %v", field, err)
	}
	return nil
}

func validateInboundBase64Field(field, value string) error {
	if err := validateBase64Syntax(value); err != nil {
		return fmt.Errorf("%s must be valid base64: %w", field, err)
	}
	return nil
}

func validateNonEmptyInboundBase64Field(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("%s must be valid base64: %w", field, err)
	}
	if len(decoded) == 0 {
		return fmt.Errorf("%s must decode to non-empty bytes", field)
	}
	return nil
}

func validateOutboundSHA256Base64URLField(field, value string) error {
	if value == "" {
		return invalidParamsError("%s must not be empty", field)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return invalidParamsError("%s must be valid unpadded base64url: %v", field, err)
	}
	if len(decoded) != sha256.Size {
		return invalidParamsError("%s must decode to a SHA-256 digest", field)
	}
	return nil
}

func validateBase64Syntax(value string) error {
	_, err := io.Copy(io.Discard, base64.NewDecoder(base64.StdEncoding, strings.NewReader(value)))
	return err
}
