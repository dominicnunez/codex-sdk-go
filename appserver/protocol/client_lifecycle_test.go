package protocol_test

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func waitClientLifetime(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("client lifetime fixture did not progress")
	}
}

func TestClientFallbackCloseRetiresClaimedNotification(t *testing.T) {
	for _, panics := range []bool{false, true} {
		name := "return"
		if panics {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			mock := NewMockTransport()
			client := protocol.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			closed, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer func() { unblock(); waitClientLifetime(t, returned) }()
			var calls atomic.Int32
			client.OnNotification("fixture", func(context.Context, protocol.Notification) {
				calls.Add(1)
				_ = client.Close() // Reentrant close must not wait for this claim.
				close(closed)
				<-release
				if panics {
					panic("application callback failure")
				}
			})
			go func() {
				defer close(returned)
				mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "fixture"})
			}()
			waitClientLifetime(t, closed)
			select {
			case <-client.Done():
				t.Fatal("client retired an executing callback")
			default:
			}
			mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "fixture"})
			unblock()
			waitClientLifetime(t, returned)
			waitClientLifetime(t, client.Done())
			if calls.Load() != 1 || client.Err() == nil {
				t.Fatalf("close admission/count wrong: calls=%d err=%v", calls.Load(), client.Err())
			}
		})
	}
}

type lifetimeTransport struct {
	*MockTransport
	done chan struct{}
}

func (m *lifetimeTransport) Done() <-chan struct{} { return m.done }
func (*lifetimeTransport) Err() error              { return io.EOF }

func TestClientUsesTransportDeliveryBoundary(t *testing.T) {
	tr := &lifetimeTransport{MockTransport: NewMockTransport(), done: make(chan struct{})}
	client := protocol.NewClient(tr)
	if client.Done() != tr.Done() {
		t.Fatal("client replaced transport delivery ownership")
	}
	called := false
	client.OnNotification("claimed", func(context.Context, protocol.Notification) { called = true })
	_ = client.Close()
	select {
	case <-client.Done():
		t.Fatal("Close retired callbacks still owned by the transport")
	default:
	}
	// The transport may have claimed this callback before Close, but not yet
	// entered the client. Its own Done is authoritative for that ownership.
	tr.InjectServerNotification(context.Background(), protocol.Notification{Method: "claimed"})
	close(tr.done)
	waitClientLifetime(t, client.Done())
	if !called || client.Err() != io.EOF {
		t.Fatal("transport-admitted callback or terminal cause lost")
	}
}
