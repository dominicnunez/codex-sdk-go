package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestFileImageDecodePaths(t *testing.T) {
	image := json.RawMessage(`{"type":"image","fileId":"file-1"}`)
	check := func(t *testing.T, input codex.UserInput) {
		t.Helper()
		got, ok := input.(*codex.ImageUserInput)
		if !ok || got.FileID == nil || *got.FileID != "file-1" || got.URL != "" {
			t.Fatalf("decoded image = %#v", input)
		}
		encoded, err := json.Marshal(input)
		if err != nil || string(encoded) != string(image) {
			t.Fatalf("round-trip = %s, err = %v", encoded, err)
		}
	}
	t.Run("shared decoder", func(t *testing.T) {
		input, err := codex.UnmarshalUserInput(image)
		if err != nil {
			t.Fatal(err)
		}
		check(t, input)
	})
	t.Run("turn start", func(t *testing.T) {
		var params codex.TurnStartParams
		if err := json.Unmarshal([]byte(`{"threadId":"thread","input":[`+string(image)+`]}`), &params); err != nil {
			t.Fatal(err)
		}
		check(t, params.Input[0])
	})
	t.Run("turn steer", func(t *testing.T) {
		var params codex.TurnSteerParams
		if err := json.Unmarshal([]byte(`{"threadId":"thread","expectedTurnId":"turn","input":[`+string(image)+`]}`), &params); err != nil {
			t.Fatal(err)
		}
		check(t, params.Input[0])
	})
	t.Run("persisted thread", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		t.Cleanup(func() { _ = client.Close() })
		thread := validThreadPayload("thread")
		thread["turns"] = []interface{}{map[string]interface{}{
			"id": "turn", "status": "completed", "items": []interface{}{map[string]interface{}{
				"type": "userMessage", "id": "message", "content": []json.RawMessage{image},
			}},
		}}
		if err := transport.SetResponseData("thread/read", map[string]interface{}{"thread": thread}); err != nil {
			t.Fatal(err)
		}
		response, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread"})
		if err != nil {
			t.Fatal(err)
		}
		item, ok := response.Thread.Turns[0].Items[0].Value.(*codex.UserMessageThreadItem)
		if !ok {
			t.Fatalf("persisted item decoded as %T", response.Thread.Turns[0].Items[0].Value)
		}
		check(t, item.Content[0])
	})
	t.Run("item notification", func(t *testing.T) {
		var notification codex.ItemStartedNotification
		payload := `{"threadId":"thread","turnId":"turn","startedAtMs":1,"item":{"type":"userMessage","id":"message","content":[` + string(image) + `]}}`
		if err := json.Unmarshal([]byte(payload), &notification); err != nil {
			t.Fatal(err)
		}
		item, ok := notification.Item.Value.(*codex.UserMessageThreadItem)
		if !ok {
			t.Fatalf("notification item decoded as %T", notification.Item.Value)
		}
		check(t, item.Content[0])
	})
	for _, payload := range []string{`{"type":"image"}`, `{"type":"image","url":null,"fileId":null}`, `{"type":"image","fileId":3}`} {
		if _, err := codex.UnmarshalUserInput([]byte(payload)); err == nil {
			t.Fatalf("accepted invalid image %s", payload)
		}
	}
	if input, err := codex.UnmarshalUserInput([]byte(`{"type":"image","url":"https://example.com/image.png"}`)); err != nil || input.(*codex.ImageUserInput).URL == "" {
		t.Fatalf("URL image compatibility: input %#v, err %v", input, err)
	}
	var output codex.FunctionCallOutputContentItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"input_image","file_id":"file-1"}`), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.Value.(*codex.InputImageFunctionCallOutputContentItem).FileID; got == nil || *got != "file-1" {
		t.Fatalf("function output file ID = %v", got)
	}
}
