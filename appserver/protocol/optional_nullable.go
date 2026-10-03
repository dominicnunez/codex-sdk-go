package protocol

import (
	"bytes"
	"encoding/json"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// OptionalNullable distinguishes an absent field from explicit null and a value.
// Present=false omits an object member with omitzero. Present=true and Value=nil
// emits null. A nonnil Value holds the supplied collection, including empty values.
// All state is exported so normal SDK deep copies retain independent values.
type OptionalNullable[T any] struct {
	Present bool
	Value   *T
}

func (v OptionalNullable[T]) IsZero() bool { return !v.Present }

func (v OptionalNullable[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value)
}

func (v *OptionalNullable[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*v = OptionalNullable[T]{Present: true}
		return nil
	}
	// Reuse map destinations to preserve stdlib ordered object merges, including
	// multiple occurrences of the field and decoding into populated receivers.
	target := v.Value
	if target == nil {
		target = new(T)
	}
	if strings, ok := any(target).(*[]string); ok {
		var list nonNullStringList
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		*strings = []string(list)
	} else if err := jsondecode.UnmarshalStandard(data, target); err != nil {
		return err
	}
	v.Present, v.Value = true, target
	return nil
}
