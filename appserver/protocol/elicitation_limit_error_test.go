package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestElicitationLimitPriorErrors(t *testing.T) {
	for _, raw := range []string{`[]`, `null`, `{}`, `{"type":"object","properties":null}`, `{"type":false,"properties":{"p":{"type":"string","minLength":4294967296}}}`, `{"type":"object","properties":{},"required":[17]}`, `{"type":"object","properties":{"p":{"type":"number","default":1e400}}}`, `{"type":"object","properties":{"p":{"type":"string","minLength":4294967296,"default":1e400}}}`, `{"type":"object","properties":{},"required":[`} {
		type wire McpElicitationSchema
		var prior wire
		required := []string{"properties", "type"}
		want := unmarshalInboundObject([]byte(raw), &prior, required, required)
		var current McpElicitationSchema
		got := current.UnmarshalJSON([]byte(raw))
		if want == nil || got == nil || want.Error() != got.Error() {
			t.Fatalf("prior native/required error changed: %s: %v / %v", raw, want, got)
		}
		var a, b *json.UnmarshalTypeError
		if errors.As(want, &a) != errors.As(got, &b) || !reflect.DeepEqual(a, b) {
			t.Fatalf("native error fields changed: %#v / %#v", a, b)
		}
	}
}
