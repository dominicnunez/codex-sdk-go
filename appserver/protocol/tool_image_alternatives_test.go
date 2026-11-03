package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestToolOutputImageAlternatives(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"url", `{"type":"input_image","image_url":"opaque-image"}`, true},
		{"file", `{"type":"input_image","file_id":"file"}`, true},
		{"empty-file", `{"type":"input_image","file_id":""}`, true},
		{"both-strings", `{"type":"input_image","image_url":"opaque-image","file_id":"file"}`, true},
		{"url-number-file", `{"type":"input_image","image_url":"opaque-image","file_id":17}`, true},
		{"url-object-file", `{"type":"input_image","image_url":"opaque-image","file_id":{}}`, true},
		{"file-bool-url", `{"type":"input_image","image_url":true,"file_id":"file"}`, true},
		{"file-object-url", `{"type":"input_image","image_url":{},"file_id":"file"}`, true},
		{"empty-url", `{"type":"input_image","image_url":""}`, true},
		{"detail-null", `{"type":"input_image","image_url":"opaque-image","detail":null}`, true},
		{"detail-original", `{"type":"input_image","image_url":"opaque-image","detail":"original"}`, true},
		{"detail-invalid", `{"type":"input_image","image_url":"opaque-image","detail":"future"}`, false},
		{"no-branch", `{"type":"input_image"}`, false},
		{"both-invalid", `{"type":"input_image","image_url":17,"file_id":false}`, false},
		{"both-null", `{"type":"input_image","image_url":null,"file_id":null}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var direct p.FunctionCallOutputContentItemWrapper
			err := json.Unmarshal([]byte(tc.body), &direct)
			if tc.valid {
				if err != nil {
					t.Errorf("valid alternative rejected by content codec: %v", err)
				} else {
					checkImageAlternatives(t, direct.Value, tc.body, true)
					encoded, e := json.Marshal(direct)
					if e != nil {
						t.Errorf("accepted content cannot encode: %v", e)
					}
					if tc.name == "empty-url" {
						var fields map[string]json.RawMessage
						_ = json.Unmarshal(encoded, &fields)
						if string(fields["image_url"]) != `""` {
							t.Error("empty URL presence erased")
						}
					}
				}
			} else if err == nil {
				t.Error("invalid alternatives accepted by content codec")
			}
			item := map[string]any{"type": "functionCallOutput", "id": "i", "name": "tool", "output": []any{json.RawMessage(tc.body)}}
			thread := validProcessThreadPayload("t")
			thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{item}}}
			m := NewMockTransport()
			diagnostics := 0
			c := p.NewClient(m, p.WithHandlerErrorCallback(func(string, error) { diagnostics++ }))
			defer c.Close()
			if e := m.SetResponseData("thread/read", map[string]any{"thread": thread}); e != nil {
				t.Fatal(e)
			}
			response, readErr := c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t", IncludeTurns: p.Ptr(true)})
			if tc.valid {
				if readErr != nil {
					t.Errorf("valid alternative rejected by Thread.Read: %v", readErr)
				} else if len(response.Thread.Turns) != 1 || len(response.Thread.Turns[0].Items) != 1 {
					t.Error("tool result disappeared")
				}
			} else if readErr == nil {
				t.Error("invalid alternative published by Thread.Read")
			}
			calls := 0
			check := func(w p.ThreadItemWrapper) {
				calls++
				if output, ok := w.Value.(*p.FunctionCallOutputThreadItem); !ok {
					t.Errorf("wrong typed tool output: %T", w.Value)
				} else {
					checkImageAlternatives(t, output.Output.Content[0].Value, tc.body, true)
				}
			}
			c.OnItemStarted(func(n p.ItemStartedNotification) { check(n.Item) })
			c.AddItemStartedListener(func(n p.ItemStartedNotification) { check(n.Item) })
			c.OnItemCompleted(func(n p.ItemCompletedNotification) { check(n.Item) })
			c.AddItemCompletedListener(func(n p.ItemCompletedNotification) { check(n.Item) })
			notification, e := json.Marshal(map[string]any{"threadId": "t", "turnId": "u", "startedAtMs": 0, "completedAtMs": 0, "item": item})
			if e != nil {
				t.Fatal(e)
			}
			for _, method := range []string{"item/started", "item/completed"} {
				m.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: notification})
			}
			if tc.valid {
				if calls != 4 || diagnostics != 0 {
					t.Errorf("valid image not delivered: callbacks=%d diagnostics=%d", calls, diagnostics)
				}
			} else if calls != 0 || diagnostics != 4 {
				t.Errorf("invalid image callbacks=%d diagnostics=%d", calls, diagnostics)
			}
		})
	}
}
