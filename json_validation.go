package codex

import (
	"encoding/json"
	"fmt"
)

func validateInboundObjectFields(data []byte, requiredFields []string, nonNullFields []string) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	nonNull := make(map[string]struct{}, len(nonNullFields))
	for _, field := range nonNullFields {
		nonNull[field] = struct{}{}
	}

	for _, field := range requiredFields {
		raw, ok := payload[field]
		if !ok {
			return fmt.Errorf("missing required field %q", field)
		}
		if _, mustBeNonNull := nonNull[field]; mustBeNonNull && isNullJSONValue(raw) {
			return fmt.Errorf("required field %q must not be null", field)
		}
	}

	return nil
}

func unmarshalInboundObject(data []byte, dest interface{}, requiredFields []string, nonNullFields []string) error {
	if err := validateInboundObjectFields(data, requiredFields, nonNullFields); err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}
