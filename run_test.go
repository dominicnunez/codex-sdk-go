package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// mockProcess creates a Process with a mock transport for testing Run().
// The mock is pre-configured to respond to initialize, thread/start, and turn/start.
// It returns the mock so callers can inject notifications.
func mockProcess(t *testing.T) (*codex.Process, *MockTransport) {
	t.Helper()
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
			"ephemeral":     true,
		},
	})

	_ = mock.SetResponseData("turn/start", map[string]interface{}{
		"turn": map[string]interface{}{
			"id":     "turn-1",
			"status": "inProgress",
			"items":  []interface{}{},
		},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	return proc, mock
}

func TestRunSuccess(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run in a goroutine since it blocks waiting for notifications.
	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{
			Prompt: "Say hello",
		})
		ch <- runResult{r, err}
	}()

	// Give Run() time to register listeners and send requests.
	time.Sleep(50 * time.Millisecond)

	// Inject item/completed notification with an agentMessage.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-1","text":"Hello there!"}}`),
	})

	// Inject turn/completed notification.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[{"type":"agentMessage","id":"item-1","text":"Hello there!"}]}}`),
	})

	result := <-ch
	if result.err != nil {
		t.Fatalf("Run() error: %v", result.err)
	}

	if result.result.Response != "Hello there!" {
		t.Errorf("Response = %q, want %q", result.result.Response, "Hello there!")
	}

	if len(result.result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(result.result.Items))
	}

	if result.result.Turn.ID != "turn-1" {
		t.Errorf("Turn.ID = %q, want %q", result.result.Turn.ID, "turn-1")
	}

	if result.result.Thread.ID != "thread-1" {
		t.Errorf("Thread.ID = %q, want %q", result.result.Thread.ID, "thread-1")
	}
}

func TestRunContextCancellation(t *testing.T) {
	proc, _ := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := proc.Run(ctx, codex.RunOptions{
		Prompt: "This will time out",
	})
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestRunEmptyPrompt(t *testing.T) {
	proc, _ := mockProcess(t)
	ctx := context.Background()

	_, err := proc.Run(ctx, codex.RunOptions{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestRunTurnError(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{
			Prompt: "This will fail",
		})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	// Inject turn/completed with an error.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"model rate limited"}}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from turn error")
	}
	if result.result != nil {
		t.Errorf("expected nil result on error, got %+v", result.result)
	}
}

func TestRunTurnErrorUnwrap(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{
			Prompt: "This will fail with details",
		})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"model rate limited","codexErrorInfo":{"code":"rate_limit"},"additionalDetails":"retry after 30s"}}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from turn error")
	}

	var turnErr *codex.TurnError
	if !errors.As(result.err, &turnErr) {
		t.Fatalf("errors.As failed: could not extract *TurnError from %v", result.err)
	}
	if turnErr.Message != "model rate limited" {
		t.Errorf("TurnError.Message = %q, want %q", turnErr.Message, "model rate limited")
	}
	if turnErr.CodexErrorInfo == nil {
		t.Error("TurnError.CodexErrorInfo is nil, want non-nil")
	}
	if turnErr.AdditionalDetails == nil || *turnErr.AdditionalDetails != "retry after 30s" {
		t.Errorf("TurnError.AdditionalDetails = %v, want %q", turnErr.AdditionalDetails, "retry after 30s")
	}
}

func TestRunInitializeFailure(t *testing.T) {
	mock := NewMockTransport()
	mock.SetSendError(fmt.Errorf("connection refused"))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	_, err := proc.Run(ctx, codex.RunOptions{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error from initialize failure")
	}
	if !strings.Contains(err.Error(), "initialize") {
		t.Errorf("error = %q, want it to mention 'initialize'", err)
	}

	// Second call retries (init is not latched on failure).
	_, err2 := proc.Run(ctx, codex.RunOptions{Prompt: "hello again"})
	if err2 == nil {
		t.Fatal("expected initialize error on second call (still failing)")
	}
}

func TestRunThreadStartFailure(t *testing.T) {
	mock := NewMockTransport()

	_ = mock.SetResponseData("initialize", map[string]interface{}{
		"userAgent": "codex-test/1.0",
	})

	// thread/start returns an RPC error.
	mock.SetResponse("thread/start", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "invalid model",
		},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	_, err := proc.Run(ctx, codex.RunOptions{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error from thread/start failure")
	}
	if !strings.Contains(err.Error(), "thread/start") {
		t.Errorf("error = %q, want it to mention 'thread/start'", err)
	}
}

func TestRunTurnStartFailure(t *testing.T) {
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
			"ephemeral":     true,
		},
	})

	// turn/start returns an RPC error.
	mock.SetResponse("turn/start", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "rate limited",
		},
	})

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	_, err := proc.Run(ctx, codex.RunOptions{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error from turn/start failure")
	}
	if !strings.Contains(err.Error(), "turn/start") {
		t.Errorf("error = %q, want it to mention 'turn/start'", err)
	}
}

func TestRunWithAllOptions(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	effort := codex.ReasoningEffortHigh
	personality := codex.PersonalityFriendly
	var approvalPolicy codex.AskForApproval = codex.ApprovalPolicyNever

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{
			Prompt:         "Explain generics",
			Instructions:   codex.Ptr("Be concise"),
			Model:          codex.Ptr("o3"),
			Effort:         &effort,
			Personality:    &personality,
			ApprovalPolicy: &approvalPolicy,
		})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	// Verify the thread/start params contain our options.
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

	var threadParams map[string]interface{}
	if err := json.Unmarshal(threadReq.Params, &threadParams); err != nil {
		t.Fatalf("unmarshal thread/start params: %v", err)
	}

	if threadParams["developerInstructions"] != "Be concise" {
		t.Errorf("developerInstructions = %v, want 'Be concise'", threadParams["developerInstructions"])
	}
	if threadParams["model"] != "o3" {
		t.Errorf("model = %v, want 'o3'", threadParams["model"])
	}
	if threadParams["personality"] != "friendly" {
		t.Errorf("personality = %v, want 'friendly'", threadParams["personality"])
	}
	if threadParams["approvalPolicy"] != "never" {
		t.Errorf("approvalPolicy = %v, want 'never'", threadParams["approvalPolicy"])
	}

	// Verify turn/start params contain effort.
	var turnReq *codex.Request
	for i := 0; i < mock.CallCount(); i++ {
		req := mock.GetSentRequest(i)
		if req != nil && req.Method == "turn/start" {
			turnReq = req
			break
		}
	}
	if turnReq == nil {
		t.Fatal("turn/start request not found")
	}

	var turnParams map[string]interface{}
	if err := json.Unmarshal(turnReq.Params, &turnParams); err != nil {
		t.Fatalf("unmarshal turn/start params: %v", err)
	}

	if turnParams["effort"] != "high" {
		t.Errorf("effort = %v, want 'high'", turnParams["effort"])
	}

	// Complete the turn so Run() returns.
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

	result := <-ch
	if result.err != nil {
		t.Fatalf("Run() error: %v", result.err)
	}
	if result.result.Response != "Done" {
		t.Errorf("Response = %q, want %q", result.result.Response, "Done")
	}
}

func TestRunMultipleItems(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{
			Prompt: "Do something complex",
		})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	// Inject multiple item/completed notifications.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"reasoning","id":"item-1"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-2","text":"First message"}}`),
	})
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "item/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":"item-3","text":"Final answer"}}`),
	})

	// Inject turn/completed.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	result := <-ch
	if result.err != nil {
		t.Fatalf("Run() error: %v", result.err)
	}

	if len(result.result.Items) != 3 {
		t.Errorf("len(Items) = %d, want 3", len(result.result.Items))
	}

	// Response should be the last agentMessage text.
	if result.result.Response != "Final answer" {
		t.Errorf("Response = %q, want %q", result.result.Response, "Final answer")
	}
}

func TestRunInitRetry(t *testing.T) {
	mock := NewMockTransport()

	// First call: init fails.
	mock.SetSendError(fmt.Errorf("connection refused"))

	client := codex.NewClient(mock, codex.WithRequestTimeout(2*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx := context.Background()
	_, err := proc.Run(ctx, codex.RunOptions{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error from first init failure")
	}

	// Fix the transport — second call should retry and succeed.
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

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx2, codex.RunOptions{Prompt: "retry"})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx2, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	result := <-ch
	if result.err != nil {
		t.Fatalf("Run() error on retry: %v", result.err)
	}
	if result.result == nil {
		t.Fatal("expected non-nil result on successful retry")
	}
}

func TestRunTurnCompletedUnmarshalFailure(t *testing.T) {
	proc, mock := mockProcess(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		result *codex.RunResult
		err    error
	}
	ch := make(chan runResult, 1)

	go func() {
		r, err := proc.Run(ctx, codex.RunOptions{Prompt: "malformed completion"})
		ch <- runResult{r, err}
	}()

	time.Sleep(50 * time.Millisecond)

	// Inject a turn/completed with valid threadId but malformed turn body.
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":12345,"status":false,"items":"not-an-array"}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from malformed turn/completed")
	}
	if !strings.Contains(result.err.Error(), "unmarshal turn/completed") {
		t.Errorf("error = %q, want it to mention 'unmarshal turn/completed'", result.err)
	}
	if result.result != nil {
		t.Error("expected nil result on unmarshal failure")
	}
}

func TestProcessCloseFromClient(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	proc := codex.NewProcessFromClient(client)

	// Close() and Wait() should not panic on a process with no cmd/transport.
	if err := proc.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
	if err := proc.Wait(); err != nil {
		t.Errorf("Wait() error: %v", err)
	}
}
