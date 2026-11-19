package transport

import (
	"context"
	"encoding/json"
	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"io"
	"sync"
	"testing"
)

func TestSearchPartialsAreBestEffort(t *testing.T) {
	method := protocol.NotifyThreadSearchPartial
	if !isBestEffortNotificationMethod(method) || isProtectedNotificationMethod(method) || isStreamingNotificationMethod(method) || isCriticalNotificationMethod(method) {
		t.Fatal("incorrect partial delivery policy")
	}
	n := Notification{Method: method, Params: json.RawMessage(`{"requestId":1,"threadId":"injected","data":[{"thread":{"id":"other"}}]}`)}
	if orderedNotificationKey(n) != "" {
		t.Fatal("partial rows acquired an ordered thread owner")
	}
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	defer tr.Close()
	defer w.Close()
	// No handler is registered: queue pressure must discard best-effort events
	// without closing the connection, while retaining only bounded payloads.
	for range cap(tr.notifQueue) + 20 {
		writeAuditNotification(t, w, n)
	}
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	if tr.ScanErr() != nil {
		t.Fatalf("partial pressure closed connection: %v", tr.ScanErr())
	}
	_ = tr.Close()
	waitNotificationBytes(t, tr, 0)
	if retainedNotificationBytes(tr) != 0 {
		t.Fatal("discarded/closed partials retained reservations")
	}
}

func TestSearchPartialQueueOverflowDrops(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); _ = tr.Close(); _ = w.Close() })
	entered := make(chan struct{}, 8)
	seen := make(chan struct{}, 1)
	tr.OnNotify(func(_ context.Context, n Notification) {
		if n.Method == "partial/block" {
			entered <- struct{}{}
			<-release
			return
		}
		if n.Method == protocol.NotifyThreadSearchPartial {
			select {
			case seen <- struct{}{}:
			default:
			}
		}
	})
	for range 8 {
		writeAuditNotification(t, w, Notification{Method: "partial/block"})
		waitAuditSignal(t, entered)
	}
	for range cap(tr.notifQueue) {
		writeAuditNotification(t, w, Notification{Method: "partial/block"})
	}
	// Every generic worker is blocked and the queue is full. This partial must
	// be discarded without blocking the reader or retaining its reservation.
	n := Notification{Method: protocol.NotifyThreadSearchPartial, Params: json.RawMessage(`{"requestId":1,"data":[]}`)}
	writeAuditNotification(t, w, n)
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	if tr.ScanErr() != nil {
		t.Fatalf("droppable partial closed reader: %v", tr.ScanErr())
	}
	for _, queued := range drainSearchQueue(tr.notifQueue) {
		if queued.Method == protocol.NotifyThreadSearchPartial {
			t.Fatal("overflow partial was retained")
		}
		queued.release()
	}
	unblock()
	_ = tr.Close()
	waitNotificationBytes(t, tr, 0)
	select {
	case <-seen:
		t.Fatal("overflow partial was published")
	default:
	}
}
func drainSearchQueue(queue chan bufferedNotification) []bufferedNotification {
	var out []bufferedNotification
	for {
		select {
		case n := <-queue:
			out = append(out, n)
		default:
			return out
		}
	}
}
