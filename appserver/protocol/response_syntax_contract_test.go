package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestResponseDirectDecodeErrorContract(t *testing.T) {
	for _, tc := range []struct {
		name, data, kind string
	}{
		{"trailing object", `{"data":[]} {}`, "object"},
		{"trailing scalar", `{"data":[]} 7`, "object"},
		{"trailing invalid token", `{"data":[]} x`, "object"},
		{"truncated field", `{"data":[],"extra":`, "object"},
		{"truncated object", `{"data":[]`, "object"},
		{"non object", `[]`, "object"},
		{"missing before trailing", `{} {}`, "missing"},
		{"null before trailing", `{"data":null} {}`, "null"},
		{"type before trailing", `{"data":7} {}`, "type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := PluginShareListResponse{Data: []PluginShareListItem{{}}}
			got := initial
			err := got.UnmarshalJSON([]byte(tc.data))
			if selectionErrorKind(err) != tc.kind {
				t.Fatalf("error kind=%q, want %q: %v", selectionErrorKind(err), tc.kind, err)
			}
			if !reflect.DeepEqual(got, initial) {
				t.Fatalf("failed response changed receiver: %+v", got)
			}
		})
	}
	// The standard library validates the entire input before calling a receiver.
	var response PluginShareListResponse
	err := json.Unmarshal([]byte(`{"data":[]} x`), &response)
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) || errors.Is(err, ErrResultNotObject) {
		t.Fatalf("standard JSON boundary lost syntax classification: %v", err)
	}
}
