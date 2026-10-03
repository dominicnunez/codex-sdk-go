package protocol_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// A scalar model derived from the InputModality schema and its established
// short-error contract; it does not invoke any production diagnostic helper.
type diagnosticReferenceModality string

func (m *diagnosticReferenceModality) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value != "text" && value != "image" && value != "audio" {
		return fmt.Errorf("invalid inputModality %q", value)
	}
	*m = diagnosticReferenceModality(value)
	return nil
}

func TestValueDiagnosticCompatibility(t *testing.T) {
	for _, body := range []string{
		`{"before":"new","value":"bad","after":"new"}`,
		`{"before":"new","value":42,"after":"new"}`,
		`{"before":"new","value":null,"after":"new"}`,
		`{"before":"new","value":"bad","value":"text","after":"new"}`,
		`{"before":"new","value":"text","value":"bad","after":"new"}`,
		`{"before":42,"value":"bad","after":"new"}`,
		`{"before":"new","value":"text","after":42}`,
	} {
		got := struct {
			Before string
			Value  codex.InputModality
			After  string
		}{"seed", codex.InputModalityImage, "seed"}
		want := struct {
			Before string
			Value  diagnosticReferenceModality
			After  string
		}{"seed", "image", "seed"}
		gotErr, wantErr := json.Unmarshal([]byte(body), &got), json.Unmarshal([]byte(body), &want)
		if !reflect.DeepEqual(gotErr, wantErr) || got.Before != want.Before || string(got.Value) != string(want.Value) || got.After != want.After {
			t.Fatalf("error metadata, precedence or reused receiver changed: got=%+v/%v want=%+v/%v", got, gotErr, want, wantErr)
		}
	}
}

func TestValueDiagnosticUnknownVariants(t *testing.T) {
	body, err := json.Marshal(map[string]any{"type": strings.Repeat("future", 1<<18), "extra": "keep"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dest := range []any{new(codex.ReviewTargetWrapper), new(codex.DynamicToolCallOutputContentItemWrapper), new(codex.ConfigLayerSourceWrapper)} {
		if err := json.Unmarshal(body, dest); err != nil {
			t.Fatalf("unknown fallback became a rejection: %v", err)
		}
		encoded, err := json.Marshal(dest)
		if err != nil || !bytes.Equal(encoded, body) {
			t.Fatal("accepted unknown data was truncated")
		}
	}
}
