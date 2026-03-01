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
	for event, err := range stream.Events() {
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
	for _, err := range stream.Events() {
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
	for event, err := range stream.Events() {
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
	for event, err := range stream.Events() {
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
	for event, err := range stream.Events() {
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
		for range stream.Events() {
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
	for _, err := range stream.Events() {
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
	for _, err := range stream.Events() {
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
	for _, err := range stream.Events() {
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
	for event, err := range stream.Events() {
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

func TestRunStreamedEarlyBreak(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	time.Sleep(50 * time.Millisecond)

	// Inject a delta then completion.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"Hi","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	// Break out of the Events loop after the first event.
	count := 0
	for range stream.Events() {
		count++
		break
	}

	if count != 1 {
		t.Errorf("expected 1 event before break, got %d", count)
	}

	// The lifecycle goroutine should still complete and close done.
	// Result() must not hang.
	done := make(chan struct{})
	go func() {
		stream.Result()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Result() hung after early break from Events()")
	}
}

func TestRunStreamedCollabEvents(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "use agents"})

	time.Sleep(50 * time.Millisecond)

	// Inject a collab item/started notification (spawnAgent).
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/started",
		Params: json.RawMessage(`{
			"threadId":"thread-1","turnId":"turn-1",
			"item":{
				"type":"collabAgentToolCall","id":"tc-1",
				"tool":"spawnAgent","status":"inProgress",
				"agentsStates":{"child-1":{"status":"pendingInit"}},
				"receiverThreadIds":["child-1"],
				"senderThreadId":"thread-1"
			}
		}`),
	})

	// Inject a collab item/completed notification.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params: json.RawMessage(`{
			"threadId":"thread-1","turnId":"turn-1",
			"item":{
				"type":"collabAgentToolCall","id":"tc-1",
				"tool":"spawnAgent","status":"completed",
				"agentsStates":{"child-1":{"status":"running"}},
				"receiverThreadIds":["child-1"],
				"senderThreadId":"thread-1"
			}
		}`),
	})

	// Complete the turn.
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

	// Expected order: CollabToolCall(started), ItemStarted, CollabToolCall(completed), ItemCompleted, TurnCompleted
	typeNames := make([]string, 0, len(events))
	for _, e := range events {
		switch ev := e.(type) {
		case *codex.CollabToolCallEvent:
			typeNames = append(typeNames, fmt.Sprintf("CollabToolCall(%s)", ev.Phase))
		case *codex.ItemStarted:
			typeNames = append(typeNames, "ItemStarted")
		case *codex.ItemCompleted:
			typeNames = append(typeNames, "ItemCompleted")
		case *codex.TurnCompleted:
			typeNames = append(typeNames, "TurnCompleted")
		default:
			typeNames = append(typeNames, fmt.Sprintf("unknown(%T)", e))
		}
	}

	expected := []string{
		"CollabToolCall(started)", "ItemStarted",
		"CollabToolCall(completed)", "ItemCompleted",
		"TurnCompleted",
	}

	if len(typeNames) != len(expected) {
		t.Fatalf("got %d events %v, want %d events %v", len(typeNames), typeNames, len(expected), expected)
	}
	for i := range expected {
		if typeNames[i] != expected[i] {
			t.Errorf("events[%d] = %q, want %q", i, typeNames[i], expected[i])
		}
	}

	// Verify the collab event data.
	started := events[0].(*codex.CollabToolCallEvent)
	if started.Phase != codex.CollabToolCallStartedPhase {
		t.Errorf("started.Phase = %q, want started", started.Phase)
	}
	if started.Tool != codex.CollabAgentToolSpawnAgent {
		t.Errorf("started.Tool = %q, want spawnAgent", started.Tool)
	}
	if started.SenderThreadId != "thread-1" {
		t.Errorf("started.SenderThreadId = %q, want 'thread-1'", started.SenderThreadId)
	}

	completed := events[2].(*codex.CollabToolCallEvent)
	if completed.Phase != codex.CollabToolCallCompletedPhase {
		t.Errorf("completed.Phase = %q, want completed", completed.Phase)
	}
	if completed.Status != codex.CollabAgentToolCallStatusCompleted {
		t.Errorf("completed.Status = %q, want completed", completed.Status)
	}
	state, ok := completed.AgentsStates["child-1"]
	if !ok {
		t.Fatal("child-1 not found in AgentsStates")
	}
	if state.Status != codex.CollabAgentStatusRunning {
		t.Errorf("child-1 status = %q, want running", state.Status)
	}
}

func TestRunStreamedInitRetry(t *testing.T) {
	mock := NewMockTransport()

	// First call: initialize fails.
	mock.SetSendError(fmt.Errorf("connection refused"))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error from first init failure")
	}

	// Fix the transport — init should retry.
	mock.SetSendError(nil)
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
	_ = mock.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	stream2 := proc.RunStreamed(ctx2, codex.RunOptions{Prompt: "retry"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx2, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	var events []codex.Event
	for event, err := range stream2.Events() {
		if err != nil {
			t.Fatalf("unexpected error on retry: %v", err)
		}
		events = append(events, event)
	}

	// Should have at least a TurnCompleted event.
	foundTC := false
	for _, e := range events {
		if _, ok := e.(*codex.TurnCompleted); ok {
			foundTC = true
		}
	}
	if !foundTC {
		t.Error("expected TurnCompleted event after successful retry")
	}
}

func TestRunStreamedTurnCompletedUnmarshalFailure(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "malformed completion"})

	time.Sleep(50 * time.Millisecond)

	// Inject a turn/completed notification where the turn field is malformed.
	// The threadId carrier unmarshal succeeds, but the full TurnCompletedNotification
	// unmarshal fails because "turn" is not a valid Turn object.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":12345,"status":false,"items":"not-an-array"}}`),
	})

	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from malformed turn/completed")
	}
	if !strings.Contains(gotErr.Error(), "unmarshal turn/completed") {
		t.Errorf("error = %q, want it to mention 'unmarshal turn/completed'", gotErr)
	}
}

func TestRunStreamedApprovalFlowDuringTurn(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Register an approval handler.
	var approvalCalled atomic.Bool
	proc.Client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnCommandExecutionRequestApproval: func(_ context.Context, p codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
			approvalCalled.Store(true)
			return codex.CommandExecutionRequestApprovalResponse{
				Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "accept"},
			}, nil
		},
	})

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "run a command"})

	time.Sleep(50 * time.Millisecond)

	// Inject a server→client approval request mid-turn.
	approvalParams, _ := json.Marshal(codex.CommandExecutionRequestApprovalParams{
		ItemID:   "item-cmd-1",
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		Command:  codex.Ptr("ls -la"),
	})
	resp, err := mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "approval-1"},
		Method:  "item/commandExecution/requestApproval",
		Params:  approvalParams,
	})
	if err != nil {
		t.Fatalf("InjectServerRequest error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("approval response has error: %v", resp.Error.Message)
	}

	// Complete the turn normally.
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

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if !approvalCalled.Load() {
		t.Error("approval handler was not called during streamed turn")
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
	}
	if result.Response != "Done" {
		t.Errorf("Response = %q, want 'Done'", result.Response)
	}
}

func TestRunStreamedEventsSingleUse(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	// First iteration should yield events.
	var firstCount int
	for range stream.Events() {
		firstCount++
	}
	if firstCount == 0 {
		t.Fatal("first iteration yielded zero events")
	}

	// Second call should yield a single ErrStreamConsumed error.
	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr != codex.ErrStreamConsumed {
		t.Errorf("second Events() error = %v, want ErrStreamConsumed", gotErr)
	}
}

func TestRunStreamedIgnoresCrossThreadNotifications(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "Say hello"})

	time.Sleep(50 * time.Millisecond)

	// Inject notifications for a different thread — these should be ignored.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"Wrong","itemId":"item-wrong","threadId":"thread-OTHER","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-OTHER","turnId":"turn-1","item":{"type":"agentMessage","id":"item-wrong","text":"Wrong thread"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-OTHER","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	// Give time for the cross-thread notifications to be dispatched and filtered.
	time.Sleep(50 * time.Millisecond)

	// Now inject the correct notifications for thread-1.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/agentMessage/delta",
		Params:  json.RawMessage(`{"delta":"Correct","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Correct thread"}}`),
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

	// Should have exactly 3 events: TextDelta, ItemCompleted, TurnCompleted — all from thread-1.
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3; events: %v", len(events), events)
	}

	if td, ok := events[0].(*codex.TextDelta); !ok {
		t.Errorf("events[0] type = %T, want *TextDelta", events[0])
	} else if td.Delta != "Correct" {
		t.Errorf("TextDelta.Delta = %q, want %q", td.Delta, "Correct")
	}

	if _, ok := events[1].(*codex.ItemCompleted); !ok {
		t.Errorf("events[1] type = %T, want *ItemCompleted", events[1])
	}

	if _, ok := events[2].(*codex.TurnCompleted); !ok {
		t.Errorf("events[2] type = %T, want *TurnCompleted", events[2])
	}

	result := stream.Result()
	if result == nil {
		t.Fatal("Result() returned nil")
	}
	if result.Response != "Correct thread" {
		t.Errorf("Response = %q, want %q", result.Response, "Correct thread")
	}
	if len(result.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(result.Items))
	}
}

func TestStreamEventsConsumedOnSecondCall(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	// Drain the first iterator.
	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Second call should yield ErrStreamConsumed.
	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr != codex.ErrStreamConsumed {
		t.Errorf("second Events() error = %v, want ErrStreamConsumed", gotErr)
	}
}

func TestRunStreamedBackpressure_SlowConsumerReceivesAllEvents(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "burst"})

	time.Sleep(50 * time.Millisecond)

	// Inject more events than the channel buffer (64) to force backpressure.
	// Injection must happen in a separate goroutine because once the buffer
	// fills, streamSendEvent blocks until the consumer reads.
	const totalDeltas = 100
	go func() {
		for i := 0; i < totalDeltas; i++ {
			mock.InjectServerNotification(ctx, codex.Notification{
				JSONRPC: "2.0",
				Method:  "item/agentMessage/delta",
				Params:  json.RawMessage(fmt.Sprintf(`{"delta":"chunk-%d","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`, i)),
			})
		}

		// Complete the turn.
		mock.InjectServerNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
		})
	}()

	// Consume slowly to exercise backpressure.
	var received int
	for event, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected error after %d events: %v", received, err)
		}
		if _, ok := event.(*codex.TextDelta); ok {
			received++
			time.Sleep(1 * time.Millisecond)
		}
	}

	if received != totalDeltas {
		t.Errorf("received %d deltas, want %d", received, totalDeltas)
	}
}

func TestRunStreamedBackpressure_ContextCancellationUnblocksSender(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "cancel me"})

	time.Sleep(50 * time.Millisecond)

	// Inject notifications from a goroutine to fill the buffer. Once the
	// buffer is full, streamSendEvent blocks. Cancelling the context must
	// unblock it, preventing goroutine leaks.
	go func() {
		for i := 0; i < 200; i++ {
			mock.InjectServerNotification(ctx, codex.Notification{
				JSONRPC: "2.0",
				Method:  "item/agentMessage/delta",
				Params:  json.RawMessage(`{"delta":"x","itemId":"item-1","threadId":"thread-1","turnId":"turn-1"}`),
			})
		}
	}()

	// Let some notifications queue up, then cancel without consuming.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// The lifecycle goroutine must not hang. Result() should return promptly.
	done := make(chan struct{})
	go func() {
		stream.Result()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle goroutine hung after context cancellation with full buffer")
	}
}

func TestStreamEventsConcurrentConsumption(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "concurrent"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	const goroutines = 10
	results := make(chan bool, goroutines)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func() {
			<-start
			var gotConsumed bool
			for _, err := range stream.Events() {
				if err == codex.ErrStreamConsumed {
					gotConsumed = true
				}
			}
			results <- gotConsumed
		}()
	}

	close(start)

	var winnersCount int
	var consumedCount int
	for i := 0; i < goroutines; i++ {
		if <-results {
			consumedCount++
		} else {
			winnersCount++
		}
	}

	if winnersCount != 1 {
		t.Errorf("expected exactly 1 winner, got %d", winnersCount)
	}
	if consumedCount != goroutines-1 {
		t.Errorf("expected %d ErrStreamConsumed, got %d", goroutines-1, consumedCount)
	}
}
