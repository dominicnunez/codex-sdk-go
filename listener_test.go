package codex

import (
	"context"
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
func (m *mockInternalTransport) OnPanic(_ func(any))            {}
func (m *mockInternalTransport) Close() error                   { return nil }
