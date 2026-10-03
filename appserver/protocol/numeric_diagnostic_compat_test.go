package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Independent native decoder model for Config's numeric fields. Like the
// established Config owner, it returns a direct type error to its containing
// decoder. It does not call the numeric diagnostic implementation.
type numericConfigReference codex.Config

func (c *numericConfigReference) UnmarshalJSON(data []byte) error {
	type native numericConfigReference
	err := json.Unmarshal(data, (*native)(c))
	var typed *json.UnmarshalTypeError
	if errors.As(err, &typed) && typed.Struct == "native" {
		typed.Struct = "Config"
	}
	return err
}

func compareNumericDiagnosticError(t *testing.T, got, want error) {
	t.Helper()
	var actual, expected *json.UnmarshalTypeError
	actualTyped, expectedTyped := errors.As(got, &actual), errors.As(want, &expected)
	if actualTyped != expectedTyped {
		t.Fatal("typed error classification changed")
	}
	if expectedTyped && len(expected.Value) > 2048 && strings.HasPrefix(expected.Value, "number ") {
		if len(actual.Value) > 2048 || !strings.Contains(actual.Value, "bytes omitted") {
			t.Fatal("oversized native value was not bounded")
		}
		metadata := *expected
		metadata.Value = actual.Value
		if !reflect.DeepEqual(*actual, metadata) {
			t.Fatal("typed metadata, offset or enclosing field context changed")
		}
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordinary error or precedence changed: got=%v want=%v", got, want)
	}
}

func TestNumericDiagnosticConfigCompatibility(t *testing.T) {
	large := "1" + strings.Repeat("0", 1<<16)
	for i, body := range []string{
		`{"model_context_window":9223372036854775808,"model":"new"}`,
		`{"model_context_window":` + large + `,"model":"new"}`,
		`{"model_context_window":` + large + `,"model_context_window":7,"model":"new"}`,
		`{"model_context_window":7,"model_context_window":` + large + `,"model":"new"}`,
		`{"model_context_window":null,"model":"new"}`,
		`{"model_context_window":"wrong","model":"new"}`,
		`{"model":42,"model_context_window":` + large + `}`,
		`{"model_context_window":` + large + `,"model":42}`,
		`{"model_context_window":` + large + `,"model_auto_compact_token_limit_scope":"invalid"}`,
		`{"model_context_window":` + large + `e+}`,
	} {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			actualNumber, referenceNumber := int64(3), int64(3)
			actualModel, referenceModel := "seed", "seed"
			actual := struct {
				Before string
				Value  codex.Config
				After  string
			}{Before: "seed", Value: codex.Config{ModelContextWindow: &actualNumber, Model: &actualModel}, After: "seed"}
			reference := struct {
				Before string
				Value  numericConfigReference
				After  string
			}{Before: "seed", Value: numericConfigReference{ModelContextWindow: &referenceNumber, Model: &referenceModel}, After: "seed"}
			input := []byte(`{"Before":"new","Value":` + body + `,"After":"new"}`)
			got, want := json.Unmarshal(input, &actual), json.Unmarshal(input, &reference)
			compareNumericDiagnosticError(t, got, want)
			if actual.Before != reference.Before || actual.After != reference.After || actualNumber != referenceNumber || actualModel != referenceModel || !reflect.DeepEqual(actual.Value, codex.Config(reference.Value)) {
				t.Fatal("reused receiver, alias storage, duplicates or enclosing traversal changed")
			}
		})
	}
}

func TestNumericDiagnosticAnonymousComposition(t *testing.T) {
	actual := struct {
		codex.Config
		After string
	}{After: "seed"}
	reference := struct {
		numericConfigReference
		After string
	}{After: "seed"}
	input := []byte(`{"model_context_window":9223372036854775808,"After":"new"}`)
	compareNumericDiagnosticError(t, json.Unmarshal(input, &actual), json.Unmarshal(input, &reference))
	if actual.After != reference.After || !reflect.DeepEqual(actual.Config, codex.Config(reference.numericConfigReference)) {
		t.Fatal("existing anonymous JSON method promotion changed")
	}
}
