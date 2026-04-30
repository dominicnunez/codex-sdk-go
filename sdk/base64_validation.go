package codex

import (
	"encoding/base64"
	"fmt"
)

func validateOutboundBase64Field(field, value string) error {
	if _, err := base64.StdEncoding.DecodeString(value); err != nil {
		return invalidParamsError("%s must be valid base64: %v", field, err)
	}
	return nil
}

func validateInboundBase64Field(field, value string) error {
	if _, err := base64.StdEncoding.DecodeString(value); err != nil {
		return fmt.Errorf("%s must be valid base64: %w", field, err)
	}
	return nil
}
