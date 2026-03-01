package codex_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestTransportConcurrentSend verifies that multiple goroutines can send requests
// concurrently and receive correctly matched responses.
func TestTransportConcurrentSend(t *testing.T) {
	mock := NewMockTransport()

	// Configure responses for different methods
	mock.SetResponse("method1", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"value": "response1"}`),
	})
	mock.SetResponse("method2", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"value": "response2"}`),
	})
	mock.SetResponse("method3", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"value": "response3"}`),
	})

	// Send requests concurrently from multiple goroutines
	const numGoroutines = 10
	const requestsPerGoroutine = 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			ctx := context.Background()

			for j := 0; j < requestsPerGoroutine; j++ {
				var method string
				switch j % 3 {
				case 1:
					method = "method2"
				case 2:
					method = "method3"
				default:
					method = "method1"
				}

				req := codex.Request{
					JSONRPC: "2.0",
					ID:      codex.RequestID{Value: int64(goroutineID*100 + j)},
					Method:  method,
					Params:  json.RawMessage(`{}`),
				}

				resp, err := mock.Send(ctx, req)
				if err != nil {
					t.Errorf("Send failed: %v", err)
					return
				}

				// Verify response ID matches request ID
				if resp.ID.Value != req.ID.Value {
					t.Errorf("Response ID mismatch: expected %v, got %v", req.ID.Value, resp.ID.Value)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify total request count
	// Note: MockTransport serializes requests via mutex, so all should be recorded
	expectedTotal := numGoroutines * requestsPerGoroutine
	if len(mock.SentRequests) != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, len(mock.SentRequests))
	}
}

// TestTransportConcurrentNotify verifies that multiple goroutines can send
// notifications concurrently without blocking each other.
func TestTransportConcurrentNotify(t *testing.T) {
	mock := NewMockTransport()

	const numGoroutines = 10
	const notificationsPerGoroutine = 5
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*notificationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			ctx := context.Background()

			for j := 0; j < notificationsPerGoroutine; j++ {
				notif := codex.Notification{
					JSONRPC: "2.0",
					Method:  "notification.test",
					Params:  json.RawMessage(`{}`),
				}

				err := mock.Notify(ctx, notif)
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Fatalf("Concurrent notify error: %v", err)
		}
	}

	// Verify total notification count
	expectedTotal := numGoroutines * notificationsPerGoroutine
	if len(mock.SentNotifications) != expectedTotal {
		t.Errorf("Expected %d total notifications, got %d", expectedTotal, len(mock.SentNotifications))
	}
}

// TestTransportConcurrentHandlers verifies that request and notification handlers
// can be invoked concurrently without race conditions.
func TestTransportConcurrentHandlers(t *testing.T) {
	mock := NewMockTransport()

	var requestCount, notificationCount int
	var mu sync.Mutex

	// Register handlers
	mock.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"status": "ok"}`),
		}, nil
	})

	mock.OnNotify(func(ctx context.Context, notif codex.Notification) {
		mu.Lock()
		notificationCount++
		mu.Unlock()
	})

	// Inject server messages concurrently
	const numGoroutines = 10
	const messagesPerGoroutine = 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			ctx := context.Background()

			for j := 0; j < messagesPerGoroutine; j++ {
				// Alternate between requests and notifications
				if j%2 == 0 {
					req := codex.Request{
						JSONRPC: "2.0",
						ID:      codex.RequestID{Value: int64(goroutineID*100 + j)},
						Method:  "server.request",
						Params:  json.RawMessage(`{}`),
					}
					_, _ = mock.InjectServerRequest(ctx, req)
				} else {
					notif := codex.Notification{
						JSONRPC: "2.0",
						Method:  "server.notification",
						Params:  json.RawMessage(`{}`),
					}
					mock.InjectServerNotification(ctx, notif)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify handler invocation counts
	// With messagesPerGoroutine=5, j values are 0,1,2,3,4
	// j%2==0 (requests): j=0,2,4 → 3 requests per goroutine
	// j%2!=0 (notifications): j=1,3 → 2 notifications per goroutine
	mu.Lock()
	expectedRequests := numGoroutines * 3      // j=0,2,4
	expectedNotifications := numGoroutines * 2 // j=1,3
	actualRequests := requestCount
	actualNotifications := notificationCount
	mu.Unlock()

	if actualRequests != expectedRequests {
		t.Errorf("Expected %d request handler invocations, got %d", expectedRequests, actualRequests)
	}
	if actualNotifications != expectedNotifications {
		t.Errorf("Expected %d notification handler invocations, got %d", expectedNotifications, actualNotifications)
	}
}

// TestTransportConcurrentSendAndHandlers verifies that client-to-server sends
// and server-to-client handler invocations can occur concurrently without deadlock.
func TestTransportConcurrentSendAndHandlers(t *testing.T) {
	mock := NewMockTransport()

	// Configure response for client requests
	mock.SetResponse("client.request", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"status": "ok"}`),
	})

	var handlerInvoked bool
	var mu sync.Mutex

	// Register server→client request handler
	mock.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		mu.Lock()
		handlerInvoked = true
		mu.Unlock()
		return codex.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"approved": true}`),
		}, nil
	})

	var wg sync.WaitGroup
	ctx := context.Background()

	// Goroutine 1: Send client→server requests
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			req := codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: int64(i)},
				Method:  "client.request",
				Params:  json.RawMessage(`{}`),
			}
			_, _ = mock.Send(ctx, req)
		}
	}()

	// Goroutine 2: Inject server→client requests
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			req := codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: int64(100 + i)},
				Method:  "server.approval",
				Params:  json.RawMessage(`{}`),
			}
			_, _ = mock.InjectServerRequest(ctx, req)
		}
	}()

	wg.Wait()

	// Verify handler was invoked
	mu.Lock()
	invoked := handlerInvoked
	mu.Unlock()

	if !invoked {
		t.Error("Server→client request handler was not invoked")
	}

	// Verify requests were sent
	if len(mock.SentRequests) != 10 {
		t.Errorf("Expected 10 client→server requests, got %d", len(mock.SentRequests))
	}
}

// TestTransportRequestResponseIDMatching verifies that response IDs correctly
// match their corresponding request IDs across different ID types.
func TestTransportRequestResponseIDMatching(t *testing.T) {
	mock := NewMockTransport()

	testCases := []struct {
		name string
		id   interface{}
	}{
		{"string ID", "request-123"},
		{"int64 ID", int64(456)},
		{"float64 ID", 789.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock.Reset()

			mock.SetResponse("test.method", codex.Response{
				JSONRPC: "2.0",
				Result:  json.RawMessage(`{"success": true}`),
			})

			req := codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: tc.id},
				Method:  "test.method",
				Params:  json.RawMessage(`{}`),
			}

			resp, err := mock.Send(context.Background(), req)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			// Verify response ID matches request ID
			// Note: JSON unmarshal may convert numeric types
			if resp.ID.Value != tc.id {
				t.Errorf("Response ID mismatch: expected %v, got %v", tc.id, resp.ID.Value)
			}
		})
	}
}

// TestTransportConcurrentClose verifies that Close can be called safely
// while other operations are in progress.
func TestTransportConcurrentClose(t *testing.T) {
	mock := NewMockTransport()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Start sending requests
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			req := codex.Request{
				JSONRPC: "2.0",
				ID:      codex.RequestID{Value: int64(i)},
				Method:  "test.method",
				Params:  json.RawMessage(`{}`),
			}
			_, _ = mock.Send(ctx, req)
			time.Sleep(time.Millisecond)
		}
	}()

	// Close transport after a short delay
	time.Sleep(10 * time.Millisecond)
	if err := mock.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	wg.Wait()

	// Verify that subsequent operations fail after close
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "after-close"},
		Method:  "test.method",
		Params:  json.RawMessage(`{}`),
	}
	_, err := mock.Send(ctx, req)
	if err == nil {
		t.Error("Expected error when sending on closed transport, got nil")
	}

	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "test.notification",
		Params:  json.RawMessage(`{}`),
	}
	err = mock.Notify(ctx, notif)
	if err == nil {
		t.Error("Expected error when notifying on closed transport, got nil")
	}
}

// TestTransportHandlerPanic verifies that handler panics don't crash the transport
// (this tests that the MockTransport is properly isolated for testing).
func TestTransportHandlerPanic(t *testing.T) {
	mock := NewMockTransport()

	// Register a handler that panics
	mock.OnRequest(func(ctx context.Context, req codex.Request) (codex.Response, error) {
		panic("handler panic")
	})

	// Inject a request - should panic
	req := codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: "test"},
		Method:  "test.method",
		Params:  json.RawMessage(`{}`),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic from handler, got none")
		}
	}()

	_, _ = mock.InjectServerRequest(context.Background(), req)
}
