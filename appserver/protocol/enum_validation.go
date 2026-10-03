package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
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
	return json.Marshal(string(value))
}
