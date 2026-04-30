package codex

import "fmt"

func validateNonEmptyStringField(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}

func validateNonEmptyStringPointerField(field string, value *string) error {
	if value == nil {
		return nil
	}
	return validateNonEmptyStringField(field, *value)
}

func validateNonEmptyStringFields(fields map[string]string) error {
	for field, value := range fields {
		if err := validateNonEmptyStringField(field, value); err != nil {
			return err
		}
	}
	return nil
}
