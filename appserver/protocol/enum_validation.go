package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

func validateEnumValue[T ~string](field string, value T, allowed map[T]struct{}) error {
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s %s", field, quotedValueDiagnostic(string(value)))
}

func validateOptionalEnumValue[T ~string](field string, value *T, allowed map[T]struct{}) error {
	if value == nil {
		return nil
	}
	return validateEnumValue(field, *value, allowed)
}

// Run after the record's established checks to preserve their error precedence.
// Field matching must agree with the containing record's decoder.
// T may be an enum pointer when the record permits explicit null.
func validateEnumOccurrences[T any](data []byte, field string, aliases bool, validate func(T) error) error {
	return validateFields(data, func(_ string, raw []byte) error {
		var value T
		if err := jsondecode.Unmarshal(raw, &value); err != nil {
			return err
		}
		return validate(value)
	}, aliases, field)
}

func validateStringEnumValue(field string, value string, allowed map[string]struct{}) error {
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s %s", field, quotedValueDiagnostic(value))
}

func unmarshalEnumString[T ~string](data []byte, field string, allowed map[T]struct{}, dest *T) error {
	var value string
	if err := jsondecode.Unmarshal(data, &value); err != nil {
		return err
	}

	enumValue := T(value)
	if err := validateEnumValue(field, enumValue, allowed); err != nil {
		return err
	}

	*dest = enumValue
	return nil
}

func marshalEnumString[T ~string](field string, value T, allowed map[T]struct{}) ([]byte, error) {
	if err := validateEnumValue(field, value, allowed); err != nil {
		return nil, err
	}
	return jsonencode.Marshal(string(value))
}
