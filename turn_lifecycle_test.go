package codex_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// notifyDuringSendTransport wraps a MockTransport and fires a turn/completed
// notification during the turn/start RPC — before returning the response.
// This simulates a fast server that pushes notifications while the client is
// still inside Send, proving that notification listeners are already registered
// at that point.
type notifyDuringSendTransport struct {
	*MockTransport
	notifHandler codex.NotificationHandler
	threadID     string
}

func (t *notifyDuringSendTransport) OnNotify(handler codex.NotificationHandler) {
	t.notifHandler = handler
	t.MockTransport.OnNotify(handler)
}

func (t *notifyDuringSendTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	resp, err := t.MockTransport.Send(ctx, req)
	if err != nil {
		return resp, err
	}

	// After the mock returns the turn/start response but before we return it
	// to the caller, inject item/completed and turn/completed notifications.
	// The caller (executeTurn) has already registered listeners before calling
	// Send, so these notifications must be handled correctly.
	if req.Method == "turn/start" && t.notifHandler != nil {
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "item/completed",
			Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turnId":"turn-1","item":{"type":"agentMessage","id":"item-early","text":"early bird"}}`),
		})
		t.notifHandler(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  json.RawMessage(`{"threadId":"` + t.threadID + `","turn":{"id":"turn-1","status":"completed","items":[{"type":"agentMessage","id":"item-early","text":"early bird"}]}}`),
		})
	}

	return resp, nil
}

func TestRunNotificationBeforeTurnStartResponse(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})
	_ = base.SetResponseData("thread/start", map[string]interface{}{
		"approvalPolicy": "never",
		"cwd":            "/tmp",
		"model":          "o3",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "readOnly"},
		"thread": map[string]interface{}{
			"id":            "thread-1",
			"cliVersion":    "1.0.0",
			"createdAt":     1700000000,
			"cwd":           "/tmp",
			"modelProvider": "openai",
			"preview":       "",
			"source":        "exec",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     1700000000,
			"ephemeral":     true,
		},
	})
	_ = base.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	transport := &notifyDuringSendTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := proc.Run(ctx, codex.RunOptions{Prompt: "early notification"})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.Response != "early bird" {
		t.Errorf("Response = %q, want %q", result.Response, "early bird")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
	if result.Turn.ID != "turn-1" {
		t.Errorf("Turn.ID = %q, want %q", result.Turn.ID, "turn-1")
	}
}

func TestRunStreamedNotificationBeforeTurnStartResponse(t *testing.T) {
	base := NewMockTransport()

	_ = base.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})
	_ = base.SetResponseData("thread/start", map[string]interface{}{
		"approvalPolicy": "never",
		"cwd":            "/tmp",
		"model":          "o3",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "readOnly"},
		"thread": map[string]interface{}{
			"id":            "thread-1",
			"cliVersion":    "1.0.0",
			"createdAt":     1700000000,
			"cwd":           "/tmp",
			"modelProvider": "openai",
			"preview":       "",
			"source":        "exec",
			"status":        map[string]interface{}{"type": "idle"},
			"turns":         []interface{}{},
			"updatedAt":     1700000000,
			"ephemeral":     true,
		},
	})
	_ = base.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	transport := &notifyDuringSendTransport{
		MockTransport: base,
		threadID:      "thread-1",
	}

	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "early notification"})

	var events []codex.Event
	for event, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, event)
	}

	// Expect at least ItemCompleted and TurnCompleted events.
	var gotItemCompleted, gotTurnCompleted bool
	for _, e := range events {
		switch e.(type) {
		case *codex.ItemCompleted:
			gotItemCompleted = true
		case *codex.TurnCompleted:
			gotTurnCompleted = true
		}
	}

	if !gotItemCompleted {
		t.Error("missing ItemCompleted event from early notification")
	}
	if !gotTurnCompleted {
		t.Error("missing TurnCompleted event from early notification")
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
	}
	if result.Response != "early bird" {
		t.Errorf("Response = %q, want %q", result.Response, "early bird")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
}
