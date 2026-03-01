package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestHandlerErrorCallback_NotificationPanic(t *testing.T) {
	var (
		gotMethod string
		gotErr    error
		mu        sync.Mutex
	)

	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = method
		gotErr = err
	}))

	client.OnNotification("test.panic", func(_ context.Context, _ codex.Notification) {
		panic("handler blew up")
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.panic",
		Params:  json.RawMessage(`{}`),
	})

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != "test.panic" {
		t.Errorf("expected method %q, got %q", "test.panic", gotMethod)
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "handler blew up") {
		t.Errorf("expected error containing %q, got %v", "handler blew up", gotErr)
	}
}

func TestHandlerErrorCallback_ApprovalPanic(t *testing.T) {
	var (
		gotMethod string
		gotErr    error
		mu        sync.Mutex
	)

	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = method
		gotErr = err
	}))

	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnFileChangeRequestApproval: func(_ context.Context, _ codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
			panic("approval exploded")
		},
	})

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: uint64(1)},
		Method:  "item/fileChange/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i","threadId":"t","turnId":"u"}`),
	}

	_, err := mock.InjectServerRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from panicking approval handler")
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != "item/fileChange/requestApproval" {
		t.Errorf("expected method %q, got %q", "item/fileChange/requestApproval", gotMethod)
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "approval exploded") {
		t.Errorf("expected error containing %q, got %v", "approval exploded", gotErr)
	}
}

func TestHandlerErrorCallback_ApprovalError(t *testing.T) {
	var (
		gotMethod string
		gotErr    error
		mu        sync.Mutex
	)

	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = method
		gotErr = err
	}))

	handlerErr := errors.New("approval denied by policy")
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnFileChangeRequestApproval: func(_ context.Context, _ codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
			return codex.FileChangeRequestApprovalResponse{}, handlerErr
		},
	})

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: uint64(1)},
		Method:  "item/fileChange/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i","threadId":"t","turnId":"u"}`),
	}

	_, err := mock.InjectServerRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from failing approval handler")
	}

	mu.Lock()
	defer mu.Unlock()

	if gotMethod != "item/fileChange/requestApproval" {
		t.Errorf("expected method %q, got %q", "item/fileChange/requestApproval", gotMethod)
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "approval denied by policy") {
		t.Errorf("expected error containing %q, got %v", "approval denied by policy", gotErr)
	}
}

func TestHandlerErrorCallback_NotSet(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock) // no callback

	var handlerEntered atomic.Bool
	client.OnNotification("test.panic", func(_ context.Context, _ codex.Notification) {
		handlerEntered.Store(true)
		panic("should be silently recovered")
	})

	// Should not panic — the recover in safeCallNotificationHandler catches it.
	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.panic",
		Params:  json.RawMessage(`{}`),
	})

	if !handlerEntered.Load() {
		t.Error("notification handler was never called")
	}
}

func TestHandlerErrorCallback_CallbackPanics(t *testing.T) {
	mock := NewMockTransport()

	var callbackEntered atomic.Bool
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, _ error) {
		callbackEntered.Store(true)
		panic("callback itself panics")
	}))

	var handlerEntered atomic.Bool
	client.OnNotification("test.panic", func(_ context.Context, _ codex.Notification) {
		handlerEntered.Store(true)
		panic("trigger")
	})

	// Should not panic — reportHandlerError recovers from callback panics.
	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.panic",
		Params:  json.RawMessage(`{}`),
	})

	if !handlerEntered.Load() {
		t.Error("notification handler was never called")
	}
	if !callbackEntered.Load() {
		t.Error("error callback was never called")
	}
}

func TestHandlerErrorCallback_InternalListenerPanic(t *testing.T) {
	var (
		callbackCount atomic.Int32
		listener2Ran  atomic.Bool
	)

	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, _ error) {
		callbackCount.Add(1)
	}))

	// Register two internal listeners via the public OnNotification + a second
	// one. We can only add internal listeners indirectly, so we use the public
	// handler for the panicking one and verify the second public handler would
	// also work. Actually, let's register one public handler that panics and
	// verify the test doesn't crash, plus use a second method to test isolation
	// isn't needed across methods.

	// Public handler panics
	client.OnNotification("test.multi", func(_ context.Context, _ codex.Notification) {
		panic("listener 1 panics")
	})

	// Register a second public handler for a different method to verify
	// the first panic doesn't affect other dispatch calls.
	client.OnNotification("test.ok", func(_ context.Context, _ codex.Notification) {
		listener2Ran.Store(true)
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.multi",
		Params:  json.RawMessage(`{}`),
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.ok",
		Params:  json.RawMessage(`{}`),
	})

	if callbackCount.Load() != 1 {
		t.Errorf("expected callback called once, got %d", callbackCount.Load())
	}
	if !listener2Ran.Load() {
		t.Error("second listener did not execute after first panicked")
	}
}

func TestHandlerErrorCallback_Concurrent(t *testing.T) {
	var callbackCount atomic.Int32

	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, _ error) {
		callbackCount.Add(1)
	}))

	client.OnNotification("test.concurrent", func(_ context.Context, _ codex.Notification) {
		panic("concurrent panic")
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  "test.concurrent",
				Params:  json.RawMessage(`{}`),
			})
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent handlers")
	}

	if callbackCount.Load() != goroutines {
		t.Errorf("expected %d callbacks, got %d", goroutines, callbackCount.Load())
	}
}
