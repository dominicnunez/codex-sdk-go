package codex

import (
	"context"
	"sync"
	"testing"
)

func TestAddNotificationListenerUnsubscribeIdempotent(t *testing.T) {
	transport := &mockInternalTransport{}
	c := NewClient(transport)

	called := 0
	unsub := c.addNotificationListener("test/method", func(_ context.Context, _ Notification) {
		called++
	})

	// Unsubscribe twice — second call must be a no-op, not panic or corrupt.
	unsub()
	unsub()

	// Verify the listener was actually removed by dispatching a notification.
	c.handleNotification(context.Background(), Notification{
		JSONRPC: "2.0",
		Method:  "test/method",
	})

	if called != 0 {
		t.Errorf("listener called %d times after double unsubscribe, want 0", called)
	}
}

// TestConcurrentInternalListeners exercises addNotificationListener,
// handleNotification dispatch, and unsubscribe concurrently under -race.
func TestConcurrentInternalListeners(t *testing.T) {
	transport := &mockInternalTransport{}
	c := NewClient(transport)

	const goroutines = 10
	const iterations = 50

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				unsub := c.addNotificationListener("test/concurrent", func(_ context.Context, _ Notification) {})

				c.handleNotification(ctx, Notification{
					JSONRPC: "2.0",
					Method:  "test/concurrent",
				})

				unsub()
			}
		}()
	}

	wg.Wait()
}

// mockInternalTransport satisfies the Transport interface for internal tests.
type mockInternalTransport struct{}

func (m *mockInternalTransport) Send(_ context.Context, _ Request) (Response, error) {
	return Response{}, nil
}

func (m *mockInternalTransport) Notify(_ context.Context, _ Notification) error {
	return nil
}

func (m *mockInternalTransport) OnNotify(_ NotificationHandler) {}
func (m *mockInternalTransport) OnRequest(_ RequestHandler)     {}
func (m *mockInternalTransport) Close() error                   { return nil }
