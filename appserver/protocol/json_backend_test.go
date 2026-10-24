package protocol_test

import (
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type streamingJSONValue int

var errStreamingJSON = &json.UnmarshalTypeError{
	Value: "number " + strings.Repeat("9", 1<<16), Type: reflect.TypeFor[int64](),
}

func (*streamingJSONValue) UnmarshalJSONFrom(*jsontext.Decoder) error { return errStreamingJSON }
func (streamingJSONValue) MarshalJSONTo(*jsontext.Encoder) error      { return errStreamingJSON }

func TestOptionalNullableStreamingCodecOwnership(t *testing.T) {
	seed := streamingJSONValue(7)
	value := p.OptionalNullable[streamingJSONValue]{Present: true, Value: &seed}
	if err := json.Unmarshal([]byte(`1`), &value); err != errStreamingJSON || !errors.Is(err, errStreamingJSON) || value.Value != &seed || seed != 7 {
		t.Fatal("streaming application decode error or receiver identity changed")
	}
	_, err := json.Marshal(value)
	if !errors.Is(err, errStreamingJSON) {
		t.Fatal("streaming application encode cause lost")
	}
	var outer *json.MarshalerError
	if !errors.As(err, &outer) {
		t.Fatal("application encode wrapper lost")
	}
	// Every wrapper remains an application-owned chain; no inner diagnostic is
	// truncated merely because it resembles a native conversion error.
	if len(errStreamingJSON.Value) != len("number ")+(1<<16) {
		t.Fatal("application error mutated")
	}
}

func TestOptionalNullableDefaultNumberCause(t *testing.T) {
	var value p.OptionalNullable[json.Number]
	err := json.Unmarshal([]byte(`"`+strings.Repeat("x", 1<<16)+`"`), &value)
	var typed *json.UnmarshalTypeError
	if !errors.As(err, &typed) || typed.Type != reflect.TypeFor[json.Number]() || len(err.Error()) > 4096 || !strings.Contains(typed.Value, "bytes omitted") || value.Present || value.Value != nil {
		t.Fatal("native Number diagnostic or failed publication changed")
	}
	if err := json.Unmarshal([]byte(`"123"`), &value); err != nil || value.Value == nil || *value.Value != "123" {
		t.Fatal("Number receiver failed recovery")
	}
}
