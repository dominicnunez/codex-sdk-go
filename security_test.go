package codex_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestSecurityRejectsTurnCompletedMissingTurnID(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		err error
	}
	ch := make(chan runResult, 1)

	go func() {
		_, err := proc.Run(ctx, codex.RunOptions{Prompt: "missing turn id"})
		ch <- runResult{err: err}
	}()

	waitForMethodCallCount(t, mock, "turn/start", 1)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"status":"completed","items":[]}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from missing turn.id")
	}
	if !strings.Contains(result.err.Error(), "invalid turn/completed notification") {
		t.Errorf("error = %q, want invalid turn/completed notification", result.err)
	}
}

func TestSecurityRejectsTurnCompletedNonTerminalStatus(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		err error
	}
	ch := make(chan runResult, 1)

	go func() {
		_, err := proc.Run(ctx, codex.RunOptions{Prompt: "invalid terminal status"})
		ch <- runResult{err: err}
	}()

	waitForMethodCallCount(t, mock, "turn/start", 1)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","items":[]}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from non-terminal turn.status")
	}
	if !strings.Contains(result.err.Error(), "invalid turn/completed notification") {
		t.Errorf("error = %q, want invalid turn/completed notification", result.err)
	}
}

func TestSecurityStreamIgnoresTurnCompletedMissingThreadID(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "missing thread id"})
	waitForRunStreamedReady(t, mock)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"turn":{"id":"turn-1","status":"completed","items":[]}}`),
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
	result := stream.Result()
	if result == nil {
		t.Fatal("expected result after valid turn completion")
	}
	if result.Turn.ID != "turn-1" {
		t.Fatalf("Turn.ID = %q, want %q", result.Turn.ID, "turn-1")
	}
}

type concurrentThreadTransport struct {
	mu                  sync.Mutex
	notificationHandler codex.NotificationHandler
	threadSeq           int
	methodCalls         map[string]int
}

func newConcurrentThreadTransport() *concurrentThreadTransport {
	return &concurrentThreadTransport{
		methodCalls: make(map[string]int),
	}
}

func (t *concurrentThreadTransport) Send(_ context.Context, req codex.Request) (codex.Response, error) {
	t.mu.Lock()
	t.methodCalls[req.Method]++
	t.mu.Unlock()

	switch req.Method {
	case "initialize":
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"userAgent":"codex-test/1.0"}`),
		}, nil
	case "thread/start":
		t.mu.Lock()
		t.threadSeq++
		threadID := fmt.Sprintf("thread-%d", t.threadSeq)
		t.mu.Unlock()

		result := map[string]interface{}{
			"approvalPolicy": "never",
			"cwd":            "/tmp",
			"model":          "o3",
			"modelProvider":  "openai",
			"sandbox":        map[string]interface{}{"type": "readOnly"},
			"thread": map[string]interface{}{
				"id":            threadID,
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
		}
		payload, err := json.Marshal(result)
		if err != nil {
			return codex.Response{}, err
		}
		return codex.Response{JSONRPC: "2.0", ID: req.ID, Result: payload}, nil
	case "turn/start":
		var params struct {
			ThreadID string `json:"threadId"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return codex.Response{}, err
		}
		result := map[string]interface{}{
			"turn": map[string]interface{}{
				"id":     "turn-" + params.ThreadID,
				"status": "inProgress",
				"items":  []interface{}{},
			},
		}
		payload, err := json.Marshal(result)
		if err != nil {
			return codex.Response{}, err
		}
		return codex.Response{JSONRPC: "2.0", ID: req.ID, Result: payload}, nil
	default:
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}
}

func (t *concurrentThreadTransport) Notify(_ context.Context, _ codex.Notification) error { return nil }
func (t *concurrentThreadTransport) OnRequest(_ codex.RequestHandler)                     {}
func (t *concurrentThreadTransport) OnNotify(handler codex.NotificationHandler) {
	t.mu.Lock()
	t.notificationHandler = handler
	t.mu.Unlock()
}
func (t *concurrentThreadTransport) Close() error { return nil }

func (t *concurrentThreadTransport) injectNotification(ctx context.Context, notif codex.Notification) {
	t.mu.Lock()
	handler := t.notificationHandler
	t.mu.Unlock()
	if handler != nil {
		handler(ctx, notif)
	}
}

func (t *concurrentThreadTransport) waitForMethod(tb *testing.T, method string, minCalls int) {
	tb.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		t.mu.Lock()
		calls := t.methodCalls[method]
		t.mu.Unlock()
		if calls >= minCalls {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	tb.Fatalf("timeout waiting for %s call count >= %d", method, minCalls)
}

func TestSecurityMalformedTurnCompletedIsIsolatedAcrossConcurrentStreams(t *testing.T) {
	transport := newConcurrentThreadTransport()
	client := codex.NewClient(transport, codex.WithRequestTimeout(5*time.Second))
	proc := codex.NewProcessFromClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	streamA := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "stream A"})
	streamB := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "stream B"})

	transport.waitForMethod(t, "turn/start", 2)

	transport.injectNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"turn":{"status":"completed","items":[]}}`),
	})

	for _, threadID := range []string{"thread-1", "thread-2"} {
		transport.injectNotification(ctx, codex.Notification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  json.RawMessage(fmt.Sprintf(`{"threadId":"%s","turn":{"id":"turn-%s","status":"completed","items":[]}}`, threadID, threadID)),
		})
	}

	drain := func(name string, stream *codex.Stream) {
		t.Helper()
		for _, err := range stream.Events() {
			if err != nil {
				t.Fatalf("%s stream error: %v", name, err)
			}
		}
		result := stream.Result()
		if result == nil {
			t.Fatalf("%s stream result is nil", name)
		}
		if result.Turn.Error != nil {
			t.Fatalf("%s stream failed with turn error: %v", name, result.Turn.Error)
		}
		if result.Thread.ID == "" {
			t.Fatalf("%s stream missing thread ID", name)
		}
	}

	drain("A", streamA)
	drain("B", streamB)
}
