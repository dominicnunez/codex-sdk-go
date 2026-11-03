package protocol

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// exactJSONValue is a private wire field for SDK-owned arbitrary JSON. Callers
// provide one validated field value; numbers retain their tokens rather than
// passing through float64. Other fields keep their existing decoding policy.
type exactJSONValue struct{ value any }

func (v *exactJSONValue) UnmarshalJSON(data []byte) error {
	return jsondecode.NativeError(jsonv2.Unmarshal(data, &v.value,
		json.DefaultOptionsV1(), jsonv2.WithUnmarshalers(exactJSONNumbers)))
}

// The default backend can decode the selected field directly from its active
// input, avoiding another complete validation pass for large argument strings.
func (v *exactJSONValue) UnmarshalJSONFrom(decoder *jsontext.Decoder) error {
	return jsondecode.NativeError(jsonv2.UnmarshalDecode(decoder, &v.value,
		json.DefaultOptionsV1(), jsonv2.WithUnmarshalers(exactJSONNumbers)))
}

// Override only numbers in interface slots. The native decoder still owns all
// other JSON kinds and legacy map/duplicate behavior, without a streaming input
// buffer copying large strings or objects a second time.
var exactJSONNumbers = jsonv2.UnmarshalFromFunc(func(decoder *jsontext.Decoder, value *any) error {
	if decoder.PeekKind() != jsontext.KindNumber {
		return errors.ErrUnsupported
	}
	raw, err := decoder.ReadValue()
	if err != nil {
		return err
	}
	*value = json.Number(string(raw))
	return nil
})

// Shadowing Arguments erases the containing native struct name. Restore only
// direct SDK-owned field errors; wrapped/custom error chains remain untouched.
func toolArgumentDecodeError(err error, name string) error {
	typed, ok := err.(*json.UnmarshalTypeError) //nolint:errorlint // Preserve wrapped error chains rather than replacing their inner cause.
	if ok && typed.Struct == "" && typed.Field != "" {
		restored := *typed
		restored.Struct = name
		return &restored
	}
	return err
}
