package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestMcpResultPriorErrors(t *testing.T) {
	for _, raw := range []string{`[]`, `null`, `false`, `{}`, `{"content":null}`, `{"content":null,"content":[]}`, `{"content":17}`, `{"content":{}}`, `{"content":"bad"}`, `{"content":false}`, `{"content":[],"content":17}`, `{"content":[`, `{"content":[],"structuredContent":`} {
		type wire McpToolCallResult
		var prior wire
		want := unmarshalRequiredEventObject([]byte(raw), &prior, "content")
		var current McpToolCallResult
		got := current.UnmarshalJSON([]byte(raw))
		if want == nil || got == nil || want.Error() != got.Error() {
			t.Fatalf("prior error changed for %s: %v / %v", raw, want, got)
		}
		var a, b *json.UnmarshalTypeError
		if errors.As(want, &a) != errors.As(got, &b) || !reflect.DeepEqual(a, b) {
			t.Fatalf("native fields changed for %s: %#v / %#v", raw, a, b)
		}
	}
}
