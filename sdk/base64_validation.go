package codex

import (
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

func validateBase64Syntax(value string) error {
	_, err := io.Copy(io.Discard, base64.NewDecoder(base64.StdEncoding, strings.NewReader(value)))
	return err
}
