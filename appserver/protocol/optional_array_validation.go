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
		// Whole-input validation already established these value boundaries.
		// Inspect only top-level elements; the wire decoder owns string types,
		// unquoting and established type-error context.
		return validateArrayElements(raw, func(index int, element []byte) error {
			if isNullJSONValue(element) {
				return fmt.Errorf("string at index %d must not be null", index)
			}
			return nil
		})
	}, names...)
}

// The caller has validated the whole JSON input and owns array type decoding.
// Iterate borrowed element spans without copying an additional array graph.
func validateArrayElements(raw []byte, validate func(int, []byte) error) error {
	if len(raw) == 0 || raw[0] != '[' {
		return nil
	}
	for start, index := jsonobject.SkipWhitespace(raw, 1), 0; start < len(raw) && raw[start] != ']'; index++ {
		end, ok := jsonobject.ValueEnd(raw, start)
		if !ok {
			return nil
		}
		if err := validate(index, raw[start:end]); err != nil {
			return err
		}
		start = jsonobject.SkipWhitespace(raw, end)
		if start < len(raw) && raw[start] == ',' {
			start = jsonobject.SkipWhitespace(raw, start+1)
		}
	}
	return nil
}

func validateOptionalArrays(data []byte, elements func([]byte) error, names ...string) error {
	return validateOptionalFields(data, func(name string, raw []byte) error {
		if isNullJSONValue(raw) {
			return responseObjectValidationErrors().null(name)
		}
		if elements != nil {
			return elements(raw)
		}
		return nil
	}, names...)
}

func validateOptionalFields(data []byte, validate func(string, []byte) error, names ...string) error {
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
			if cause := validate(name, raw); cause != nil {
				err = fmt.Errorf("%s: %w", name, cause)
			}
			return
		}
	})
	return err
}
