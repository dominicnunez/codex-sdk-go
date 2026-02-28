package codex_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestRunStreamedSuccess(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "Say hello"})

	// Give the lifecycle goroutine time to register listeners and send requests.
	time.Sleep(50 * time.Millisecond)

	// Inject deltas then completion.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"Hel","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"lo!","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Hello!"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	var events []codex.Event
	for event, err := range stream.Events {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, event)
	}

	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// Verify event types in order.
	if _, ok := events[0].(*codex.TextDelta); !ok {
		t.Errorf("events[0] type = %T, want *TextDelta", events[0])
	}
	if _, ok := events[1].(*codex.TextDelta); !ok {
		t.Errorf("events[1] type = %T, want *TextDelta", events[1])
	}
	if ic, ok := events[2].(*codex.ItemCompleted); !ok {
		t.Errorf("events[2] type = %T, want *ItemCompleted", events[2])
	} else if _, ok := ic.Item.Value.(*codex.AgentMessageThreadItem); !ok {
		t.Errorf("ItemCompleted.Item type = %T, want *AgentMessageThreadItem", ic.Item.Value)
	}
	if _, ok := events[3].(*codex.TurnCompleted); !ok {
		t.Errorf("events[3] type = %T, want *TurnCompleted", events[3])
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
	}
	if result.Response != "Hello!" {
		t.Errorf("Response = %q, want %q", result.Response, "Hello!")
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.Items))
	}
}

func TestRunStreamedContextCancellation(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "This will time out"})

	var gotErr error
	for _, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestRunStreamedEmptyPrompt(t *testing.T) {
	proc, _ := mockProcess(t)
	ctx := context.Background()

	stream := proc.RunStreamed(ctx, codex.RunOptions{})

	var gotErr error
	for _, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestRunStreamedTurnError(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "This will fail"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"model rate limited"}}}`),
	})

	var gotErr error
	var events []codex.Event
	for event, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
		events = append(events, event)
	}

	if gotErr == nil {
		t.Fatal("expected error from turn error")
	}

	// TurnCompleted event should have been yielded before the error.
	foundTC := false
	for _, e := range events {
		if _, ok := e.(*codex.TurnCompleted); ok {
			foundTC = true
		}
	}
	if !foundTC {
		t.Error("expected TurnCompleted event before error")
	}

	// Result should be nil since the turn errored.
	if stream.Result() != nil {
		t.Error("expected nil Result() after turn error")
	}
}

func TestRunStreamedNoClobbering(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Register a user handler before RunStreamed.
	var userHandlerCalls atomic.Int32
	proc.Client.OnItemCompleted(func(n codex.ItemCompletedNotification) {
		userHandlerCalls.Add(1)
	})

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	time.Sleep(50 * time.Millisecond)

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

	var streamGotItem bool
	for event, err := range stream.Events {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := event.(*codex.ItemCompleted); ok {
			streamGotItem = true
		}
	}

	if !streamGotItem {
		t.Error("stream did not receive ItemCompleted event")
	}
	if userHandlerCalls.Load() == 0 {
		t.Error("user handler was not called (clobbered by RunStreamed)")
	}
}

func TestRunStreamedMultipleEventTypes(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "complex task"})

	time.Sleep(50 * time.Millisecond)

	// Mix different event types.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/reasoning/textDelta",
		Params:  json.RawMessage(`{"delta":"thinking...","itemId":"r-1","contentIndex":0,"threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":""}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"result","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"result"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	typeNames := make(map[string]bool)
	for event, err := range stream.Events {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch event.(type) {
		case *codex.ReasoningDelta:
			typeNames["ReasoningDelta"] = true
		case *codex.ItemStarted:
			typeNames["ItemStarted"] = true
		case *codex.TextDelta:
			typeNames["TextDelta"] = true
		case *codex.ItemCompleted:
			typeNames["ItemCompleted"] = true
		case *codex.TurnCompleted:
			typeNames["TurnCompleted"] = true
		}
	}

	for _, expected := range []string{"ReasoningDelta", "ItemStarted", "TextDelta", "ItemCompleted", "TurnCompleted"} {
		if !typeNames[expected] {
			t.Errorf("missing event type %s", expected)
		}
	}
}

func TestRunStreamedResultBeforeIteration(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	// Call Result() in a goroutine — it should block until done.
	type resultOut struct {
		r *codex.RunResult
	}
	resultCh := make(chan resultOut, 1)
	go func() {
		r := stream.Result()
		resultCh <- resultOut{r}
	}()

	time.Sleep(50 * time.Millisecond)

	// Drain the events iterator in another goroutine so the channel doesn't block.
	go func() {
		for range stream.Events {
		}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Done"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	select {
	case res := <-resultCh:
		if res.r == nil {
			t.Fatal("Result() returned nil")
		}
		if res.r.Response != "Done" {
			t.Errorf("Response = %q, want %q", res.r.Response, "Done")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Result() did not return within timeout")
	}
}

func TestRunStreamedInitializeFailure(t *testing.T) {
	mock := NewMockTransport()
	mock.SetSendError(fmt.Errorf("connection refused"))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	var gotErr error
	for _, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from initialize failure")
	}
	if !strings.Contains(gotErr.Error(), "initialize") {
		t.Errorf("error = %q, want it to mention 'initialize'", gotErr)
	}
}

func TestRunStreamedThreadStartFailure(t *testing.T) {
	mock := NewMockTransport()

	_ = mock.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})
	mock.SetResponse("thread/start", codex.Response{
		JSONRPC: "2.0",
		Error:   &codex.Error{Code: -32600, Message: "invalid model"},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	var gotErr error
	for _, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from thread/start failure")
	}
	if !strings.Contains(gotErr.Error(), "thread/start") {
		t.Errorf("error = %q, want it to mention 'thread/start'", gotErr)
	}
}

func TestRunStreamedTurnStartFailure(t *testing.T) {
	mock := NewMockTransport()

	_ = mock.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})
	_ = mock.SetResponseData("thread/start", map[string]interface{}{
		"approvalPolicy": "never",
		"cwd":            "/tmp",
		"model":          "o3",
		"modelProvider":  "openai",
		"sandbox":        map[string]interface{}{"type": "readOnly"},
		"thread": map[string]interface{}{
			"id": "thread-1", "cliVersion": "1.0.0", "createdAt": 1700000000,
			"cwd": "/tmp", "modelProvider": "openai", "preview": "", "source": "exec",
			"status": map[string]interface{}{"type": "idle"}, "turns": []interface{}{},
			"updatedAt": 1700000000, "ephemeral": true,
		},
	})
	mock.SetResponse("turn/start", codex.Response{
		JSONRPC: "2.0",
		Error:   &codex.Error{Code: -32600, Message: "rate limited"},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	var gotErr error
	for _, err := range stream.Events {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from turn/start failure")
	}
	if !strings.Contains(gotErr.Error(), "turn/start") {
		t.Errorf("error = %q, want it to mention 'turn/start'", gotErr)
	}
}

func TestRunStreamedAllDeltaTypes(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "all deltas"})

	time.Sleep(50 * time.Millisecond)

	// Inject turn/started.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/started",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","items":[]}}`),
	})
	// Reasoning summary delta.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/reasoning/summaryTextDelta",
		Params:  json.RawMessage(`{"delta":"summary...","itemId":"r-1","summaryIndex":0,"threadId":"thread-1","turnId":"turn-1"}`),
	})
	// Plan delta.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/plan/delta",
		Params:  json.RawMessage(`{"delta":"step 1","itemId":"p-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	// File change delta.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/fileChange/outputDelta",
		Params:  json.RawMessage(`{"delta":"+line","itemId":"f-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	// Complete the turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	typeNames := make(map[string]bool)
	for event, err := range stream.Events {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch event.(type) {
		case *codex.TurnStarted:
			typeNames["TurnStarted"] = true
		case *codex.ReasoningSummaryDelta:
			typeNames["ReasoningSummaryDelta"] = true
		case *codex.PlanDelta:
			typeNames["PlanDelta"] = true
		case *codex.FileChangeDelta:
			typeNames["FileChangeDelta"] = true
		case *codex.TurnCompleted:
			typeNames["TurnCompleted"] = true
		}
	}

	for _, expected := range []string{"TurnStarted", "ReasoningSummaryDelta", "PlanDelta", "FileChangeDelta", "TurnCompleted"} {
		if !typeNames[expected] {
			t.Errorf("missing event type %s", expected)
		}
	}
}
