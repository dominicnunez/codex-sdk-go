package codex

import (
	"context"
	"strings"
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

func TestHandleNotificationRunsInternalListenersBeforePublicHandler(t *testing.T) {
	transport := &mockInternalTransport{}
	c := NewClient(transport)

	order := make(chan string, 2)
	c.addNotificationListener("test/order", func(_ context.Context, _ Notification) {
		order <- "internal"
	})
	c.OnNotification("test/order", func(_ context.Context, _ Notification) {
		order <- "public"
	})

	c.handleNotification(context.Background(), Notification{
		JSONRPC: "2.0",
		Method:  "test/order",
	})

	first := <-order
	second := <-order
	if first != "internal" || second != "public" {
		t.Fatalf("listener order = %q, %q; want internal before public", first, second)
	}
}

func TestHandleNotificationInternalListenerPanicReportsErrorAndContinues(t *testing.T) {
	const (
		method       = "test/internal-panic"
		panicMessage = "internal listener panics"
		wantErrors   = 1
	)

	var (
		gotMethod string
		gotErr    error
		errCount  int
	)

	transport := &mockInternalTransport{}
	c := NewClient(transport, WithHandlerErrorCallback(func(method string, err error) {
		gotMethod = method
		gotErr = err
		errCount++
	}))

	secondInternalRan := false
	publicRan := false

	c.addNotificationListener(method, func(_ context.Context, _ Notification) {
		panic(panicMessage)
	})
	c.addNotificationListener(method, func(_ context.Context, _ Notification) {
		secondInternalRan = true
	})
	c.OnNotification(method, func(_ context.Context, _ Notification) {
		publicRan = true
	})

	c.handleNotification(context.Background(), Notification{
		JSONRPC: "2.0",
		Method:  method,
	})

	if errCount != wantErrors {
		t.Fatalf("handler error count = %d; want %d", errCount, wantErrors)
	}
	if gotMethod != method {
		t.Fatalf("handler error method = %q; want %q", gotMethod, method)
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), panicMessage) {
		t.Fatalf("handler error = %v; want panic message %q", gotErr, panicMessage)
	}
	if !secondInternalRan {
		t.Fatal("second internal listener did not execute after first panicked")
	}
	if !publicRan {
		t.Fatal("public handler did not execute after internal listener panicked")
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
func (m *mockInternalTransport) Close() error                   { return nil }
