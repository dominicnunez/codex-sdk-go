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

func TestTurnSteerParamsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(t *testing.T, p codex.TurnSteerParams)
	}{
		{
			name: "text input",
			json: `{
				"threadId": "t-1",
				"expectedTurnId": "turn-1",
				"input": [{"type": "text", "text": "hello"}]
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if p.ThreadID != "t-1" {
					t.Errorf("ThreadID = %q, want t-1", p.ThreadID)
				}
				if p.ExpectedTurnID != "turn-1" {
					t.Errorf("ExpectedTurnID = %q, want turn-1", p.ExpectedTurnID)
				}
				if len(p.Input) != 1 {
					t.Fatalf("len(Input) = %d, want 1", len(p.Input))
				}
				text, ok := p.Input[0].(*codex.TextUserInput)
				if !ok {
					t.Fatalf("Input[0] type = %T, want *TextUserInput", p.Input[0])
				}
				if text.Text != "hello" {
					t.Errorf("Input[0].Text = %q, want hello", text.Text)
				}
			},
		},
		{
			name: "image input",
			json: `{
				"threadId": "t-2",
				"expectedTurnId": "turn-2",
				"input": [{"type": "image", "url": "https://example.com/img.png"}]
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if len(p.Input) != 1 {
					t.Fatalf("len(Input) = %d, want 1", len(p.Input))
				}
				img, ok := p.Input[0].(*codex.ImageUserInput)
				if !ok {
					t.Fatalf("Input[0] type = %T, want *ImageUserInput", p.Input[0])
				}
				if img.URL != "https://example.com/img.png" {
					t.Errorf("Input[0].URL = %q, want https://example.com/img.png", img.URL)
				}
			},
		},
		{
			name: "mixed text and image inputs",
			json: `{
				"threadId": "t-3",
				"expectedTurnId": "turn-3",
				"input": [
					{"type": "text", "text": "look at this"},
					{"type": "image", "url": "https://example.com/pic.jpg"}
				]
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if len(p.Input) != 2 {
					t.Fatalf("len(Input) = %d, want 2", len(p.Input))
				}
				if _, ok := p.Input[0].(*codex.TextUserInput); !ok {
					t.Errorf("Input[0] type = %T, want *TextUserInput", p.Input[0])
				}
				if _, ok := p.Input[1].(*codex.ImageUserInput); !ok {
					t.Errorf("Input[1] type = %T, want *ImageUserInput", p.Input[1])
				}
			},
		},
		{
			name: "empty input array",
			json: `{
				"threadId": "t-4",
				"expectedTurnId": "turn-4",
				"input": []
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if len(p.Input) != 0 {
					t.Errorf("len(Input) = %d, want 0", len(p.Input))
				}
			},
		},
		{
			name: "unknown input type produces UnknownUserInput",
			json: `{
				"threadId": "t-5",
				"expectedTurnId": "turn-5",
				"input": [{"type": "futureType", "data": 42}]
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if len(p.Input) != 1 {
					t.Fatalf("len(Input) = %d, want 1", len(p.Input))
				}
				unknown, ok := p.Input[0].(*codex.UnknownUserInput)
				if !ok {
					t.Fatalf("Input[0] type = %T, want *UnknownUserInput", p.Input[0])
				}
				if unknown.Type != "futureType" {
					t.Errorf("UnknownUserInput.Type = %q, want futureType", unknown.Type)
				}
			},
		},
		{
			name:    "invalid JSON returns error",
			json:    `{not valid json`,
			wantErr: true,
		},
		{
			name: "missing input field yields empty slice",
			json: `{
				"threadId": "t-7",
				"expectedTurnId": "turn-7"
			}`,
			check: func(t *testing.T, p codex.TurnSteerParams) {
				if len(p.Input) != 0 {
					t.Errorf("len(Input) = %d, want 0", len(p.Input))
				}
				if p.ThreadID != "t-7" {
					t.Errorf("ThreadID = %q, want t-7", p.ThreadID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.TurnSteerParams
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, got)
		})
	}

	// Round-trip: marshal then unmarshal should preserve data
	t.Run("round-trip marshal/unmarshal", func(t *testing.T) {
		original := codex.TurnSteerParams{
			ThreadID:       "rt-thread",
			ExpectedTurnID: "rt-turn",
			Input: []codex.UserInput{
				&codex.TextUserInput{Text: "round trip"},
				&codex.ImageUserInput{URL: "https://example.com/rt.png"},
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var restored codex.TurnSteerParams
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if restored.ThreadID != original.ThreadID {
			t.Errorf("ThreadID = %q, want %q", restored.ThreadID, original.ThreadID)
		}
		if restored.ExpectedTurnID != original.ExpectedTurnID {
			t.Errorf("ExpectedTurnID = %q, want %q", restored.ExpectedTurnID, original.ExpectedTurnID)
		}
		if len(restored.Input) != 2 {
			t.Fatalf("len(Input) = %d, want 2", len(restored.Input))
		}

		text, ok := restored.Input[0].(*codex.TextUserInput)
		if !ok {
			t.Fatalf("Input[0] type = %T, want *TextUserInput", restored.Input[0])
		}
		if text.Text != "round trip" {
			t.Errorf("Input[0].Text = %q, want %q", text.Text, "round trip")
		}

		img, ok := restored.Input[1].(*codex.ImageUserInput)
		if !ok {
			t.Fatalf("Input[1] type = %T, want *ImageUserInput", restored.Input[1])
		}
		if img.URL != "https://example.com/rt.png" {
			t.Errorf("Input[1].URL = %q, want %q", img.URL, "https://example.com/rt.png")
		}
	})
}
