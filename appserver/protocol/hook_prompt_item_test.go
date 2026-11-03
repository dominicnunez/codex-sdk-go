package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func hookPromptRead(t *testing.T, raw string) error {
	t.Helper()
	thread := validProcessThreadPayload("hook-thread")
	thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": []any{json.RawMessage(raw)}}}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	result, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "hook-thread"})
	if err != nil && result.Thread.ID != "" {
		t.Error("failed read returned partial thread")
	}
	return err
}

func hookPromptCallbacks(t *testing.T, raw string, valid bool) {
	t.Helper()
	mock := NewMockTransport()
	reports, started, completed := 0, 0, 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
		if method != "item/started" && method != "item/completed" {
			t.Errorf("diagnostic origin=%s", method)
		}
		reports++
	}))
	defer client.Close()
	client.OnItemStarted(func(p.ItemStartedNotification) { started++ })
	removeStarted := client.AddItemStartedListener(func(p.ItemStartedNotification) { started++ })
	defer removeStarted()
	client.OnItemCompleted(func(p.ItemCompletedNotification) { completed++ })
	removeCompleted := client.AddItemCompletedListener(func(p.ItemCompletedNotification) { completed++ })
	defer removeCompleted()
	for _, method := range []string{"item/started", "item/completed"} {
		body := `{"threadId":"hook-thread","turnId":"turn","startedAtMs":0,"completedAtMs":0,"item":` + raw + `}`
		mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(body)})
	}
	if valid {
		if started != 2 || completed != 2 || reports != 0 {
			t.Errorf("valid callbacks=%d/%d reports=%d", started, completed, reports)
		}
	} else if started != 0 || completed != 0 || reports < 2 {
		t.Errorf("malformed item published callbacks=%d/%d reports=%d", started, completed, reports)
	}
}

func TestHookPromptRejectsMalformed(t *testing.T) {
	for _, raw := range []string{
		`{"type":"hookPrompt","fragments":[]}`,
		`{"type":"hookPrompt","id":null,"fragments":[]}`,
		`{"type":"hookPrompt","id":"i"}`,
		`{"type":"hookPrompt","id":"i","fragments":null}`,
		`{"type":"hookPrompt","id":"i","fragmen\u0074s":null}`,
		`{"type":"hookPrompt","id":"i","fragments":{}}`,
		`{"type":"hookPrompt","id":"i","fragments":[null]}`,
		`{"type":"hookPrompt","id":"i","fragments":[42]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"text":"t"}]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r"}]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":null,"text":"t"}]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":null}]}`,
		`{"type":"hookPrompt","id":"i","fragments":null,"fragments":[]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":null,"text":"t"}]}`,
		`{"type":"hookPrompt","id":"i","fragments":[],"id":null,"id":"i"}`,
		`{"type":"hookPrompt","id":"i","fragments":[],"completedAtMs":"bad"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			prior := &p.UnknownThreadItem{Type: "future", Raw: json.RawMessage(`{"type":"future"}`)}
			wrapper := p.ThreadItemWrapper{Value: prior}
			if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
				t.Error("malformed known hookPrompt admitted")
			} else if wrapper.Value != prior {
				t.Error("new rejection replaced prior union receiver")
			}
			if err := hookPromptRead(t, raw); err == nil {
				t.Error("Thread.Read published malformed hookPrompt")
			}
			hookPromptCallbacks(t, raw, false)
		})
	}
}

func TestHookPromptValidControls(t *testing.T) {
	for _, raw := range []string{
		`{"type":"hookPrompt","id":"","fragments":[]}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"","text":""}],"startedAtMs":0,"completedAtMs":null}`,
		`{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":"prompt"}],"startedAtMs":null,"completedAtMs":0}`,
		`{"type":"future-hook","id":null,"fragments":null,"completedAtMs":"opaque"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var wrapper p.ThreadItemWrapper
			if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
				t.Fatal(err)
			}
			if err := hookPromptRead(t, raw); err != nil {
				t.Fatal(err)
			}
			hookPromptCallbacks(t, raw, true)
		})
	}
}
