package codex

import "fmt"

func validateEnumValue[T ~string](field string, value T, allowed map[T]struct{}) error {
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s %q", field, value)
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
	return fmt.Errorf("invalid %s %q", field, value)
}
