package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Validate every recognized occurrence, including the folded aliases accepted
// by encoding/json. A later duplicate cannot repair a forbidden null array or
// string element. The containing decoder owns requiredness and receiver updates.
func validateOptionalStringArrays(data []byte, names ...string) error {
	return validateOptionalArrays(data, func(raw []byte) error {
		var values []*string
		// The actual wire decoder owns established type-error context. Only
		// otherwise valid arrays need the extra null-element admission check.
		if json.Unmarshal(raw, &values) == nil {
			for i, value := range values {
				if value == nil {
					return fmt.Errorf("string at index %d must not be null", i)
				}
			}
		}
		return nil
	}, names...)
}

func validateOptionalArrays(data []byte, elements func([]byte) error, names ...string) error {
	// Preserve the containing decoder's syntax and non-object error contract.
	if !json.Valid(data) {
		return nil
	}
	var err error
	folded := make([]string, len(names))
	for i, name := range names {
		folded[i] = strings.ToLower(name)
	}
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if err != nil {
			return
		}
		for i, name := range names {
			if !jsonobject.FieldMatchesFolded(key, folded[i]) {
				continue
			}
			if isNullJSONValue(raw) {
				err = responseObjectValidationErrors().null(name)
			} else if elements != nil {
				if cause := elements(raw); cause != nil {
					err = fmt.Errorf("%s: %w", name, cause)
				}
			}
			return
		}
	})
	return err
}
