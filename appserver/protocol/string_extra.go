package protocol

import (
	"encoding/json"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// stringExtra preserves the last successfully decoded string
// occurrence where the selected schema branch leaves the field unrestricted.
// Wrong JSON types are ignored without changing the prior successful value.
type stringExtra struct {
	Value *string
}

func (e *stringExtra) UnmarshalJSON(data []byte) error {
	var decoded *string
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		// This decode targets only a native *string. Ignore only its direct
		// type mismatch; a wrapped error must remain visible to the caller.
		if _, ok := err.(*json.UnmarshalTypeError); ok { //nolint:errorlint // jsondecode.Unmarshal into this native *string returns a direct mismatch error.
			return nil
		}
		return err
	}
	e.Value = decoded
	return nil
}
