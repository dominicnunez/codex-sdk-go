// Package jsondecode bounds native JSON value and field diagnostics at
// audited SDK decoding boundaries before their callers wrap the error.
package jsondecode

import (
	"encoding"
	"encoding/json"
	jsonv2 "encoding/json/v2"
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
	if !ok {
		return err
	}
	// Native errors can include the original number or quoted string, as well
	// as dynamic map keys in Field. Copy only when bounding is necessary and
	// keep structured locations and native causes for ordinary diagnostics.
	bounded := *typed
	if len(typed.Value) > 2048 {
		for _, prefix := range []string{"number ", "string "} {
			if strings.HasPrefix(typed.Value, prefix) {
				bounded.Value = prefix + diagnostic.Display(strings.TrimPrefix(typed.Value, prefix))
				break
			}
		}
	}
	if len(typed.Field) > 2048 {
		bounded.Field = diagnostic.Display(typed.Field)
	}
	if bounded.Value == typed.Value && bounded.Field == typed.Field {
		return err
	}
	return &bounded
}

// UnmarshalStandard preserves unrestricted generic delegation. Only a static
// destination graph with no custom codecs or interface slots proves that its
// error came from standard decoding rather than application code.
func UnmarshalStandard(data []byte, dest any) error {
	err := json.Unmarshal(data, dest)
	if err == nil {
		return nil
	}
	if !standardType(reflect.TypeOf(dest), make(map[reflect.Type]bool)) {
		return err
	}
	return NativeError(err)
}

var (
	unmarshalerType      = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType  = reflect.TypeFor[encoding.TextUnmarshaler]()
	streamingDecoderType = reflect.TypeFor[jsonv2.UnmarshalerFrom]()
)

func standardType(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nil {
		return false
	}
	// Number's streaming method belongs to the standard library, rather than
	// an application callback; its malformed-literal errors are native.
	if t == reflect.TypeFor[json.Number]() || t == reflect.TypeFor[*json.Number]() {
		return true
	}
	if t.Implements(streamingDecoderType) || reflect.PointerTo(t).Implements(streamingDecoderType) {
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
			// Only the exact dash tag is ignored by encoding/json. With tag
			// options, "-" names an active field that can delegate to a codec.
			if (field.PkgPath != "" && !field.Anonymous) || field.Tag.Get("json") == "-" {
				continue
			}
			if !standardType(field.Type, seen) {
				return false
			}
		}
	}
	return true
}
