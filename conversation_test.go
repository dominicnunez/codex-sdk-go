package codex_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestConversationMultiTurn(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{
		Instructions: codex.Ptr("Be helpful"),
	})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	if conv.ThreadID() == "" {
		t.Fatal("ThreadID() is empty")
	}

	// First turn.
	type turnResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan turnResult, 1)

	go func() {
		r, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "Hello"})
		ch <- turnResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Hi there!"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	r1 := <-ch
	if r1.err != nil {
		t.Fatalf("Turn 1 error: %v", r1.err)
	}
	if r1.result.Response != "Hi there!" {
		t.Errorf("Turn 1 Response = %q, want 'Hi there!'", r1.result.Response)
	}

	// Second turn — uses same thread.
	ch2 := make(chan turnResult, 1)
	go func() {
		r, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "What is 2+2?"})
		ch2 <- turnResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-2","item":{"type":"agentMessage","id":"item-2","text":"4"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-2","status":"completed","items":[]}}`),
	})

	r2 := <-ch2
	if r2.err != nil {
		t.Fatalf("Turn 2 error: %v", r2.err)
	}
	if r2.result.Response != "4" {
		t.Errorf("Turn 2 Response = %q, want '4'", r2.result.Response)
	}

	// Thread should have accumulated turns.
	thread := conv.Thread()
	if len(thread.Turns) != 2 {
		t.Errorf("Thread.Turns = %d, want 2", len(thread.Turns))
	}
}

func TestConversationTurnStreamed(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "Stream me"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"Hi","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Hi"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	var events []codex.Event
	for event, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, event)
	}

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
	}
	if result.Response != "Hi" {
		t.Errorf("Response = %q, want 'Hi'", result.Response)
	}
}

func TestConversationEmptyPrompt(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx := context.Background()
	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	_, err = conv.Turn(ctx, codex.TurnOptions{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestConversationTurnStreamedEmptyPrompt(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx := context.Background()
	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	stream := conv.TurnStreamed(ctx, codex.TurnOptions{})

	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestConversationWithCollaborationMode(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	ch := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{
			Prompt: "Use agents",
			CollaborationMode: &codex.CollaborationMode{
				Mode:     codex.ModeKindPlan,
				Settings: codex.CollaborationModeSettings{Model: "o3"},
			},
		})
		ch <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Verify the turn/start request contains collaborationMode.
	var turnReq *codex.Request
	for i := 0; i < mock.CallCount(); i++ {
		req := mock.GetSentRequest(i)
		if req != nil && req.Method == "turn/start" {
			turnReq = req
		}
	}
	if turnReq == nil {
		t.Fatal("turn/start request not found")
	}

	var turnParams map[string]interface{}
	if err := json.Unmarshal(turnReq.Params, &turnParams); err != nil {
		t.Fatalf("unmarshal turn/start params: %v", err)
	}
	if _, ok := turnParams["collaborationMode"]; !ok {
		t.Error("collaborationMode not present in turn/start params")
	}

	// Complete the turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-ch; err != nil {
		t.Fatalf("Turn error: %v", err)
	}
}
