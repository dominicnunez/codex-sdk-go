package codex_test

import (
	"context"
	"encoding/json"
	"strings"
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

func TestConversationStartWithAllOptions(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx := context.Background()
	personality := codex.PersonalityFriendly
	var approvalPolicy codex.AskForApproval = codex.ApprovalPolicyNever

	_, err := proc.StartConversation(ctx, codex.ConversationOptions{
		Instructions:   codex.Ptr("Be concise"),
		Model:          codex.Ptr("o3"),
		Personality:    &personality,
		ApprovalPolicy: &approvalPolicy,
	})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	var threadReq *codex.Request
	for i := 0; i < mock.CallCount(); i++ {
		req := mock.GetSentRequest(i)
		if req != nil && req.Method == "thread/start" {
			threadReq = req
			break
		}
	}
	if threadReq == nil {
		t.Fatal("thread/start request not found")
	}

	var params map[string]interface{}
	if err := json.Unmarshal(threadReq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["developerInstructions"] != "Be concise" {
		t.Errorf("developerInstructions = %v, want 'Be concise'", params["developerInstructions"])
	}
	if params["model"] != "o3" {
		t.Errorf("model = %v, want 'o3'", params["model"])
	}
	if params["personality"] != "friendly" {
		t.Errorf("personality = %v, want 'friendly'", params["personality"])
	}
	if params["approvalPolicy"] != "never" {
		t.Errorf("approvalPolicy = %v, want 'never'", params["approvalPolicy"])
	}
}

func TestConversationTurnWithAllOptions(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	effort := codex.ReasoningEffortHigh
	ch := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{
			Prompt: "hello",
			Effort: &effort,
			Model:  codex.Ptr("o3"),
		})
		ch <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Find the turn/start request and verify it has effort and model.
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

	var params map[string]interface{}
	if err := json.Unmarshal(turnReq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["effort"] != "high" {
		t.Errorf("effort = %v, want 'high'", params["effort"])
	}
	if params["model"] != "o3" {
		t.Errorf("model = %v, want 'o3'", params["model"])
	}

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-ch; err != nil {
		t.Fatalf("Turn error: %v", err)
	}
}

func TestConversationTurnError(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	type turnResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan turnResult, 1)

	go func() {
		r, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "fail"})
		ch <- turnResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"rate limited"}}}`),
	})

	r := <-ch
	if r.err == nil {
		t.Fatal("expected error from turn error")
	}
	if r.result != nil {
		t.Error("expected nil result on turn error")
	}
}

func TestConversationTurnContextCancel(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	_, err = conv.Turn(ctx, codex.TurnOptions{Prompt: "timeout"})
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestConversationTurnStreamedTurnError(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "fail"})

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"model error"}}}`),
	})

	var gotErr error
	var foundTC bool
	for event, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
		if _, ok := event.(*codex.TurnCompleted); ok {
			foundTC = true
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from turn error")
	}
	if !foundTC {
		t.Error("expected TurnCompleted event before error")
	}
	if stream.Result() != nil {
		t.Error("expected nil Result() after turn error")
	}
}

func TestConversationTurnStreamedContextCancel(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "timeout"})

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

func TestStartConversationThreadStartFailure(t *testing.T) {
	mock := NewMockTransport()

	_ = mock.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})

	// thread/start returns an RPC error.
	mock.SetResponse("thread/start", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "invalid configuration",
		},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	_, err := proc.StartConversation(ctx, codex.ConversationOptions{
		Instructions: codex.Ptr("Be helpful"),
	})
	if err == nil {
		t.Fatal("expected error from thread/start failure")
	}
	if !strings.Contains(err.Error(), "thread/start") {
		t.Errorf("error = %q, want it to mention 'thread/start'", err)
	}
}

func TestConversationConcurrentTurnRejected(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Start first turn in background.
	firstDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "first"})
		firstDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Second turn should be rejected while the first is in progress.
	_, err = conv.Turn(ctx, codex.TurnOptions{Prompt: "second"})
	if err == nil {
		t.Fatal("expected error from concurrent turn")
	}
	if err.Error() != "a turn is already in progress on this conversation" {
		t.Errorf("error = %q, want turn-in-progress error", err)
	}

	// Complete the first turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-firstDone; err != nil {
		t.Fatalf("first turn error: %v", err)
	}

	// After the first completes, a new turn should work.
	thirdDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "third"})
		thirdDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-2","status":"completed","items":[]}}`),
	})

	if err := <-thirdDone; err != nil {
		t.Fatalf("third turn error: %v", err)
	}
}

func TestConversationThreadDeepCopyTurnError(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	turnDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "hello"})
		turnDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-turnDone; err != nil {
		t.Fatalf("Turn error: %v", err)
	}

	// Verify Items slice isolation across snapshots.
	snap := conv.Thread()
	if len(snap.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(snap.Turns))
	}
	snap.Turns[0].Items = append(snap.Turns[0].Items, codex.ThreadItemWrapper{})
	snap2 := conv.Thread()
	if len(snap2.Turns[0].Items) != 0 {
		t.Error("Items mutation leaked through deep copy")
	}
}

func TestConversationConcurrentTurnStreamedRejected(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Start first streamed turn in background.
	stream1 := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "first"})
	time.Sleep(50 * time.Millisecond)

	// Second TurnStreamed should be rejected while the first is in progress.
	stream2 := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "second"})

	var gotErr error
	for _, err := range stream2.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error from concurrent TurnStreamed")
	}
	if gotErr.Error() != "a turn is already in progress on this conversation" {
		t.Errorf("error = %q, want turn-in-progress error", gotErr)
	}

	// Complete the first turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream1.Events() {
		if err != nil {
			t.Fatalf("stream1 error: %v", err)
		}
	}
}

func TestConversationConcurrentTurnVsTurnStreamedRejected(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Start a Turn in background.
	firstDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "first"})
		firstDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// TurnStreamed should be rejected while Turn is in progress.
	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "second"})

	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error from concurrent TurnStreamed while Turn is active")
	}
	if gotErr.Error() != "a turn is already in progress on this conversation" {
		t.Errorf("error = %q, want turn-in-progress error", gotErr)
	}

	// Complete the first turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-firstDone; err != nil {
		t.Fatalf("first turn error: %v", err)
	}
}

func TestConversationConcurrentTurnStreamedVsTurnRejected(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Start a TurnStreamed in background.
	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "first"})
	time.Sleep(50 * time.Millisecond)

	// Turn should be rejected while TurnStreamed is in progress.
	_, err = conv.Turn(ctx, codex.TurnOptions{Prompt: "second"})
	if err == nil {
		t.Fatal("expected error from concurrent Turn while TurnStreamed is active")
	}
	if err.Error() != "a turn is already in progress on this conversation" {
		t.Errorf("error = %q, want turn-in-progress error", err)
	}

	// Complete the first streamed turn.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	}
}

func TestConversationThreadDeepCopyIsolation(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Execute one turn to populate thread state.
	turnDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "Hello"})
		turnDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-turnDone; err != nil {
		t.Fatalf("Turn error: %v", err)
	}

	// Get a snapshot and mutate it.
	snapshot1 := conv.Thread()
	originalLen := len(snapshot1.Turns)
	snapshot1.Turns = append(snapshot1.Turns, codex.Turn{ID: "injected"})

	// Get another snapshot and verify the mutation did not affect internal state.
	snapshot2 := conv.Thread()
	if len(snapshot2.Turns) != originalLen {
		t.Errorf("Thread mutation leaked: got %d turns, want %d", len(snapshot2.Turns), originalLen)
	}
}

func TestConversationThreadDeepCopyItemValueIsolation(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Execute a turn that produces an agentMessage item.
	turnDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "hello"})
		turnDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"original"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[{"type":"agentMessage","id":"item-1","text":"original"}]}}`),
	})

	if err := <-turnDone; err != nil {
		t.Fatalf("Turn error: %v", err)
	}

	// Get a snapshot and mutate the item value through the interface pointer.
	snap1 := conv.Thread()
	if len(snap1.Turns) == 0 || len(snap1.Turns[0].Items) == 0 {
		t.Fatal("expected at least one turn with one item")
	}
	msg, ok := snap1.Turns[0].Items[0].Value.(*codex.AgentMessageThreadItem)
	if !ok {
		t.Fatal("expected AgentMessageThreadItem")
	}
	msg.Text = "mutated"

	// A second snapshot should still see the original text.
	snap2 := conv.Thread()
	msg2, ok := snap2.Turns[0].Items[0].Value.(*codex.AgentMessageThreadItem)
	if !ok {
		t.Fatal("expected AgentMessageThreadItem")
	}
	if msg2.Text != "original" {
		t.Errorf("item value mutation leaked: got %q, want %q", msg2.Text, "original")
	}
}

func TestConversationThreadSnapshotDuringTurnCompletion(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	turnDone := make(chan error, 1)
	go func() {
		_, err := conv.Turn(ctx, codex.TurnOptions{Prompt: "race"})
		turnDone <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Concurrently call Thread() while completing the turn.
	// Run with -race to verify no data race.
	snapshotDone := make(chan struct{})
	go func() {
		defer close(snapshotDone)
		for i := 0; i < 100; i++ {
			snap := conv.Thread()
			_ = snap.Turns
		}
	}()

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	if err := <-turnDone; err != nil {
		t.Fatalf("Turn error: %v", err)
	}
	<-snapshotDone
}

func TestConversationStreamedThreadSnapshotDuringTurnCompletion(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	stream := conv.TurnStreamed(ctx, codex.TurnOptions{Prompt: "race"})

	time.Sleep(50 * time.Millisecond)

	// Concurrently call Thread() while iterating streamed events.
	// Run with -race to verify no data race on the shared thread state.
	snapshotDone := make(chan struct{})
	go func() {
		defer close(snapshotDone)
		for i := 0; i < 100; i++ {
			snap := conv.Thread()
			_ = snap.Turns
		}
	}()

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

	for _, err := range stream.Events() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}
	<-snapshotDone
}

func TestConversationThreadDeepCopyIsolation_ZeroTurnsPointerFields(t *testing.T) {
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
			"ephemeral":     false,
			"name":          "original-name",
			"agentNickname": "original-nickname",
			"agentRole":     "original-role",
			"path":          "/original/path",
			"gitInfo": map[string]interface{}{
				"branch":    "main",
				"originUrl": "https://example.com/repo.git",
				"sha":       "abc123",
			},
		},
	})

	_ = mock.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":    "turn-1",
			"items": []interface{}{},
		},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	conv, err := proc.StartConversation(ctx, codex.ConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation: %v", err)
	}

	// Snapshot with zero turns but populated pointer fields.
	snap := conv.Thread()
	if len(snap.Turns) != 0 {
		t.Fatalf("expected 0 turns, got %d", len(snap.Turns))
	}

	// Mutate snapshot string pointer fields.
	*snap.Name = "mutated-name"
	*snap.AgentNickname = "mutated-nickname"
	*snap.AgentRole = "mutated-role"
	*snap.Path = "/mutated/path"

	// Mutate snapshot GitInfo fields.
	*snap.GitInfo.Branch = "feature-branch"
	*snap.GitInfo.OriginURL = "https://mutated.com/repo.git"
	*snap.GitInfo.SHA = "deadbeef"

	// A second snapshot should still see original values.
	snap2 := conv.Thread()

	if *snap2.Name != "original-name" {
		t.Errorf("Name mutation leaked: got %q, want %q", *snap2.Name, "original-name")
	}
	if *snap2.AgentNickname != "original-nickname" {
		t.Errorf("AgentNickname mutation leaked: got %q, want %q", *snap2.AgentNickname, "original-nickname")
	}
	if *snap2.AgentRole != "original-role" {
		t.Errorf("AgentRole mutation leaked: got %q, want %q", *snap2.AgentRole, "original-role")
	}
	if *snap2.Path != "/original/path" {
		t.Errorf("Path mutation leaked: got %q, want %q", *snap2.Path, "/original/path")
	}
	if *snap2.GitInfo.Branch != "main" {
		t.Errorf("GitInfo.Branch mutation leaked: got %q, want %q", *snap2.GitInfo.Branch, "main")
	}
	if *snap2.GitInfo.OriginURL != "https://example.com/repo.git" {
		t.Errorf("GitInfo.OriginURL mutation leaked: got %q, want %q", *snap2.GitInfo.OriginURL, "https://example.com/repo.git")
	}
	if *snap2.GitInfo.SHA != "abc123" {
		t.Errorf("GitInfo.SHA mutation leaked: got %q, want %q", *snap2.GitInfo.SHA, "abc123")
	}

	// Also verify GitInfo pointer identity is distinct.
	if snap.GitInfo == snap2.GitInfo {
		t.Error("GitInfo pointer is shared between snapshots")
	}
}
