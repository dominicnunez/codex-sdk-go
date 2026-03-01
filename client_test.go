package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestClientSendRequest verifies that the Client can send a request and receive a response.
func TestClientSendRequest(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Configure mock to return a specific response
	expectedResult := json.RawMessage(`{"status":"ok"}`)
	mock.SetResponse("test.method", codex.Response{
		JSONRPC: "2.0",
		Result:  expectedResult,
	})

	// Send request
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "req-1"},
		Method:  "test.method",
		Params:  json.RawMessage(`{"foo":"bar"}`),
	}

	ctx := context.Background()
	resp, err := client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify response
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC=2.0, got %s", resp.JSONRPC)
	}
	if string(resp.Result) != string(expectedResult) {
		t.Errorf("expected result=%s, got %s", string(expectedResult), string(resp.Result))
	}

	// Verify the request was sent through the transport
	sentReq := mock.GetSentRequest(0)
	if sentReq == nil {
		t.Fatal("no request was sent")
	}
	if sentReq.Method != "test.method" {
		t.Errorf("expected method=test.method, got %s", sentReq.Method)
	}
}

// TestClientNotificationListener verifies that notification listeners are dispatched correctly.
func TestClientNotificationListener(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register a notification listener
	received := make(chan codex.Notification, 1)
	client.OnNotification("test.notification", func(ctx context.Context, notif codex.Notification) {
		received <- notif
	})

	// Simulate server sending a notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.notification",
		Params:  json.RawMessage(`{"event":"test"}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, notif)

	// Wait for the listener to be called
	select {
	case receivedNotif := <-received:
		if receivedNotif.Method != "test.notification" {
			t.Errorf("expected method=test.notification, got %s", receivedNotif.Method)
		}
		if string(receivedNotif.Params) != `{"event":"test"}` {
			t.Errorf("expected params={\"event\":\"test\"}, got %s", string(receivedNotif.Params))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("notification listener was not called")
	}
}

// TestClientUnknownNotification verifies that unknown notification methods don't cause errors.
func TestClientUnknownNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register a listener for a specific method
	called := false
	client.OnNotification("test.known", func(ctx context.Context, notif codex.Notification) {
		called = true
	})

	// Inject an unknown notification method
	unknownNotif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.unknown",
		Params:  json.RawMessage(`{}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, unknownNotif)

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Verify the listener was NOT called (unknown method should be ignored)
	if called {
		t.Error("listener was called for unknown notification method")
	}

	// Verify no error occurred (unknown notifications should be gracefully ignored)
	// The test passing means no panic occurred
}

// TestClientRequestTimeout verifies that requests respect context timeouts.
func TestClientRequestTimeout(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock, codex.WithRequestTimeout(50*time.Millisecond))

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "timeout-req"},
		Method:  "test.timeout",
		Params:  json.RawMessage(`{}`),
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Send(ctx, req)
	if err == nil {
		t.Fatal("expected canceled error, got nil")
	}

	// Verify it's a CanceledError (not TimeoutError — cancellation is user-initiated)
	var cancelErr *codex.CanceledError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected CanceledError, got: %v", err)
	}
}

// TestClientDefaultTimeout verifies that the client uses the default timeout when configured.
func TestClientDefaultTimeout(t *testing.T) {
	mock := NewMockTransport()
	timeout := 100 * time.Millisecond
	client := codex.NewClient(mock, codex.WithRequestTimeout(timeout))

	// Send a request with a background context (no explicit timeout)
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "default-timeout"},
		Method:  "test.method",
		Params:  json.RawMessage(`{}`),
	}

	// Mock returns immediately, so this should succeed
	ctx := context.Background()
	_, err := client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify request was sent
	sentReq := mock.GetSentRequest(0)
	if sentReq == nil {
		t.Fatal("no request was sent")
	}

	// Now test that a slow response triggers a TimeoutError.
	// Use a very short timeout so the mock's lack of response causes expiry.
	shortTimeout := 25 * time.Millisecond
	slowClient := codex.NewClient(NewSlowMockTransport(shortTimeout*2), codex.WithRequestTimeout(shortTimeout))

	slowReq := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "timeout-fires"},
		Method:  "test.slow",
		Params:  json.RawMessage(`{}`),
	}

	_, err = slowClient.Send(context.Background(), slowReq)
	if err == nil {
		t.Fatal("expected TimeoutError, got nil")
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected TimeoutError, got: %T: %v", err, err)
	}
}

// TestClientMultipleListeners verifies that multiple listeners for different methods work.
func TestClientMultipleListeners(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Register multiple listeners
	method1Called := make(chan bool, 1)
	method2Called := make(chan bool, 1)

	client.OnNotification("method.one", func(ctx context.Context, notif codex.Notification) {
		method1Called <- true
	})

	client.OnNotification("method.two", func(ctx context.Context, notif codex.Notification) {
		method2Called <- true
	})

	ctx := context.Background()

	// Send notification for method.one
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "method.one",
		Params:  json.RawMessage(`{}`),
	})

	// Send notification for method.two
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "method.two",
		Params:  json.RawMessage(`{}`),
	})

	// Verify both were called
	select {
	case <-method1Called:
	case <-time.After(100 * time.Millisecond):
		t.Error("method.one listener was not called")
	}

	select {
	case <-method2Called:
	case <-time.After(100 * time.Millisecond):
		t.Error("method.two listener was not called")
	}
}

// TestClientRPCError verifies that RPC errors are properly wrapped.
func TestClientRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Configure mock to return an error response
	mock.SetResponse("test.error", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "Invalid request",
		},
	})

	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "err-req"},
		Method:  "test.error",
		Params:  json.RawMessage(`{}`),
	}

	ctx := context.Background()
	_, err := client.Send(ctx, req)
	if err == nil {
		t.Fatal("expected RPC error, got nil")
	}

	// Verify it's an RPCError
	var rpcErr *codex.RPCError
	if !isRPCError(err, &rpcErr) {
		t.Fatalf("expected RPCError, got: %T", err)
	}

	if rpcErr.RPCError().Code != -32600 {
		t.Errorf("expected error code -32600, got %d", rpcErr.RPCError().Code)
	}
}

// isTimeoutError checks if err is or wraps a TimeoutError or DeadlineExceeded.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeoutErr *codex.TimeoutError
	return errors.As(err, &timeoutErr)
}

// isRPCError checks if err is or wraps an RPCError.
func isRPCError(err error, target interface{}) bool {
	switch v := target.(type) {
	case **codex.RPCError:
		return errors.As(err, v)
	case **codex.TimeoutError:
		return errors.As(err, v)
	}
	return false
}
