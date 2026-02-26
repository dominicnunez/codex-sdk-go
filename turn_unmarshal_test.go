package codex_test

import (
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestUnknownUserInputType verifies that UnmarshalUserInput returns an error for unknown types
func TestUnknownUserInputType(t *testing.T) {
	// Create a TurnStartParams with an unknown UserInput type
	jsonData := []byte(`{
		"threadId": "thread-123",
		"input": [
			{"type": "unknownType", "someField": "value"}
		]
	}`)

	var params codex.TurnStartParams
	err := json.Unmarshal(jsonData, &params)

	// We expect an error because the type is unknown
	if err == nil {
		t.Fatal("expected error for unknown UserInput type, got nil")
	}

	// Verify error message mentions the unknown type
	if !strings.Contains(err.Error(), "unknown UserInput type") {
		t.Errorf("error message should mention 'unknown UserInput type', got: %v", err)
	}
	if !strings.Contains(err.Error(), "unknownType") {
		t.Errorf("error message should include the unknown type name, got: %v", err)
	}
}

// TestValidUserInputTypeAfterUnknown verifies that valid types still work correctly
func TestValidUserInputTypeAfterUnknown(t *testing.T) {
	// Create a TurnStartParams with a valid UserInput type
	jsonData := []byte(`{
		"threadId": "thread-123",
		"input": [
			{"type": "text", "text": "Hello world"}
		]
	}`)

	var params codex.TurnStartParams
	err := json.Unmarshal(jsonData, &params)

	// Valid type should not error
	if err != nil {
		t.Fatalf("unexpected error for valid UserInput type: %v", err)
	}

	// Verify the input was parsed correctly
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
