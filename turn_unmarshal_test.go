package codex_test

import (
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestUnknownUserInputType verifies that UnmarshalUserInput preserves unknown types
// for forward compatibility with newer protocol versions.
func TestUnknownUserInputType(t *testing.T) {
	jsonData := []byte(`{
		"threadId": "thread-123",
		"input": [
			{"type": "unknownType", "someField": "value"}
		]
	}`)

	var params codex.TurnStartParams
	if err := json.Unmarshal(jsonData, &params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(params.Input) != 1 {
		t.Fatalf("expected 1 input, got %d", len(params.Input))
	}

	unknown, ok := params.Input[0].(*codex.UnknownUserInput)
	if !ok {
		t.Fatalf("expected *UnknownUserInput, got %T", params.Input[0])
	}

	// Verify roundtrip preserves the raw JSON
	out, err := json.Marshal(unknown)
	if err != nil {
		t.Fatalf("marshal unknown input: %v", err)
	}

	var rt map[string]interface{}
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if rt["type"] != "unknownType" {
		t.Errorf("roundtrip type = %v, want unknownType", rt["type"])
	}
	if rt["someField"] != "value" {
		t.Errorf("roundtrip someField = %v, want value", rt["someField"])
	}
}

// TestValidUserInputTypeAfterUnknown verifies that valid types still work correctly
func TestValidUserInputTypeAfterUnknown(t *testing.T) {
	jsonData := []byte(`{
		"threadId": "thread-123",
		"input": [
			{"type": "text", "text": "Hello world"}
		]
	}`)

	var params codex.TurnStartParams
	if err := json.Unmarshal(jsonData, &params); err != nil {
		t.Fatalf("unexpected error for valid UserInput type: %v", err)
	}

	if len(params.Input) != 1 {
		t.Fatalf("expected 1 input, got %d", len(params.Input))
	}

	textInput, ok := params.Input[0].(*codex.TextUserInput)
	if !ok {
		t.Fatalf("expected *TextUserInput, got %T", params.Input[0])
	}

	if textInput.Text != "Hello world" {
		t.Errorf("expected text 'Hello world', got '%s'", textInput.Text)
	}
}
