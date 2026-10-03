// Package jsondecode bounds native JSON numeric conversion diagnostics at
// audited SDK decoding boundaries before their callers wrap the error.
package jsondecode

import (
	"encoding"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/internal/diagnostic"
)

// Unmarshal is for SDK-owned destinations whose codecs cannot delegate to an
// application decoder. It retains ordinary decoding and short error behavior.
func Unmarshal(data []byte, dest any) error {
	return NativeError(json.Unmarshal(data, dest))
}

// NativeError also serves SDK-owned streaming decoders. It must not be applied
// to arbitrary callback errors: their concrete type does not prove provenance.
func NativeError(err error) error {
	typed, ok := err.(*json.UnmarshalTypeError) //nolint:errorlint // Only direct native errors can be copied without discarding an existing error chain.
	// The bounded 256-byte quoted preview is always below this retained-value
	// limit, even with worst-case escaping. Nested SDK decoders therefore leave
	// an already bounded error alone instead of repeatedly quoting its preview.
	if !ok || len(typed.Value) <= 2048 || !strings.HasPrefix(typed.Value, "number ") {
		return err
	}
	value := diagnostic.Display(strings.TrimPrefix(typed.Value, "number "))
	// Copy metadata without retaining or mutating the original error. Returning
	// the concrete type preserves enclosing encoding/json field propagation.
	bounded := *typed
	bounded.Value = "number " + value
	return &bounded
}

// UnmarshalStandard preserves unrestricted generic delegation. Only a static
// destination graph with no custom codecs or interface slots proves that its
// type error came from standard decoding rather than application code.
func UnmarshalStandard(data []byte, dest any) error {
	err := json.Unmarshal(data, dest)
	if _, ok := err.(*json.UnmarshalTypeError); !ok { //nolint:errorlint // Wrapped custom errors must retain their original chain and identity.
		return err
	}
	if !standardType(reflect.TypeOf(dest), make(map[reflect.Type]bool)) {
		return err
	}
	return NativeError(err)
}

var (
	unmarshalerType     = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func standardType(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nil {
		return false
	}
	if t.Implements(unmarshalerType) || t.Implements(textUnmarshalerType) || reflect.PointerTo(t).Implements(unmarshalerType) || reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return false
	}
	if seen[t] {
		return true
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Interface:
		return false
	case reflect.Pointer, reflect.Array, reflect.Slice:
		return standardType(t.Elem(), seen)
	case reflect.Map:
		return standardType(t.Key(), seen) && standardType(t.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if (field.PkgPath != "" && !field.Anonymous) || strings.Split(field.Tag.Get("json"), ",")[0] == "-" {
				continue
			}
			if !standardType(field.Type, seen) {
				return false
			}
		}
	}
	return true
}
