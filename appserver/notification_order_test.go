package appserver

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

func TestStdioPreservesTypedDeltaListenerOrder(t *testing.T) {
	r, w := io.Pipe()
	tr := transport.NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	c := protocol.NewClient(tr)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	c.AddAgentMessageDeltaListener(func(n protocol.AgentMessageDeltaNotification) {
		if n.Delta == "A" {
			close(entered)
			<-release
		}
	})
	seen := make(chan string, 2)
	c.AddAgentMessageDeltaListener(func(n protocol.AgentMessageDeltaNotification) { seen <- n.Delta })
	barrier := make(chan struct{})
	c.OnNotification(protocol.NotifyConfigWarning, func(context.Context, protocol.Notification) { close(barrier) })
	writeOrderedWire(t, w, protocol.NotifyAgentMessageDelta, map[string]any{"threadId": "thread", "ThreadID": "other", "turnId": "turn", "itemId": "item", "delta": "A"})
	waitOrderedWire(t, entered)
	writeOrderedWire(t, w, protocol.NotifyAgentMessageDelta, map[string]any{"threadId": "thread", "turnId": "turn", "itemId": "item", "delta": "B"})
	writeOrderedWire(t, w, protocol.NotifyConfigWarning, map[string]any{})
	waitOrderedWire(t, barrier)
	unblock()
	for _, want := range []string{"A", "B"} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("typed listener got %q; want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("typed listener did not finish")
		}
	}
}

func TestStdioPreservesCachedThreadNameOrder(t *testing.T) {
	r, w := io.Pipe()
	tr := transport.NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	c := protocol.NewClient(tr)
	c.CacheThreadState(protocol.Thread{ID: "thread"})
	seen := make(chan int, 2)
	c.OnThreadNameUpdated(func(n protocol.ThreadNameUpdatedNotification) { seen <- len(*n.ThreadName) })
	// The first valid frame is expensive to decode, but the newer name must
	// still reach the cache and listeners after it.
	for _, name := range []string{strings.Repeat("A", 8<<20), "new"} {
		writeOrderedWire(t, w, protocol.NotifyThreadNameUpdated, map[string]any{"threadId": "thread", "threadName": name})
	}
	for _, want := range []int{8 << 20, 3} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("name callback length = %d; want %d", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("name notification did not finish")
		}
	}
	snapshot, ok := c.ThreadStateSnapshot("thread")
	if !ok || snapshot.Name == nil || *snapshot.Name != "new" {
		t.Fatal("cache did not retain newest name")
	}
}

func writeOrderedWire(t *testing.T, w io.Writer, method string, params any) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}

func waitOrderedWire(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("notification barrier timed out")
	}
}
