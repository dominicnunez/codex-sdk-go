package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStdioNotificationBytesBoundedBeforeHandler(t *testing.T) {
	for _, method := range []string{"item/agentMessage/delta", "thread/name/updated", "error", "mixed", "unknown"} {
		for _, owner := range []string{`"a"`, `99`} {
			t.Run(method+owner, func(t *testing.T) { checkStdioNotificationByteLimit(t, method, owner) })
		}
	}
}

func checkStdioNotificationByteLimit(t *testing.T, method, owner string) {
	t.Helper()
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	// Each frame is below the 10 MiB frame limit, and nine events are far
	// below all count limits. Their retained payloads exceed 64 MiB.
	params := json.RawMessage(`{"threadId":` + owner + `,"delta":"` + strings.Repeat("x", 8*1024*1024) + `"}`)
	for i := range 9 {
		frameMethod := method
		if method == "mixed" {
			frameMethod = []string{"item/agentMessage/delta", "thread/name/updated", "error"}[i%3]
		}
		data, err := json.Marshal(Notification{JSONRPC: "2.0", Method: frameMethod, Params: params})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			break // A byte overflow must close the read side.
		}
	}
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	if method == "unknown" {
		if tr.ScanErr() != nil || retainedNotificationBytes(tr) > 64*1024*1024 {
			t.Fatal("best-effort byte exhaustion failed connection or exceeded budget")
		}
		tr.OnNotify(func(context.Context, Notification) {})
	} else if !errors.Is(tr.ScanErr(), errNotificationByteLimit) {
		t.Fatalf("byte overflow error = %v", tr.ScanErr())
	}
	if err := tr.Notify(context.Background(), Notification{Method: "probe"}); err == nil {
		t.Fatal("byte overflow left the transport writable")
	}
	waitNotificationBytes(t, tr, 0)
	if got := retainedNotificationBytes(tr); got != 0 {
		t.Fatalf("byte overflow retained %d bytes", got)
	}
}

func TestStdioNotificationBudgetIncludesCallbacks(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, release := make(chan struct{}, 16), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	tr.OnNotify(func(context.Context, Notification) { entered <- struct{}{}; <-release })
	for i := range 9 {
		params := json.RawMessage(`{"threadId":"` + strings.Repeat("a", i+1) + `","delta":"` + strings.Repeat("x", 8*1024*1024) + `"}`)
		data, err := json.Marshal(Notification{JSONRPC: "2.0", Method: "item/agentMessage/delta", Params: params})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			break
		}
		if i == 0 {
			waitAuditSignal(t, entered)
		}
	}
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	if !errors.Is(tr.ScanErr(), errNotificationByteLimit) {
		t.Fatalf("active callbacks escaped byte limit: %v", tr.ScanErr())
	}
	if got := retainedNotificationBytes(tr); got <= 0 || got > 64*1024*1024 {
		t.Fatalf("callback retention = %d bytes", got)
	}
	unblock()
	waitNotificationBytes(t, tr, 0)
}

func TestNotificationBudgetTransfersAndEviction(t *testing.T) {
	tr := newBudgetTestTransport(t)
	n := Notification{JSONRPC: "2.0", Method: "unknown", Params: json.RawMessage(`{"value":"data"}`)}
	// 3+7+16 bytes are charged once. The same reservation must move from a
	// channel to the pre-handler buffer and back during registration.
	const size = 26
	tr.enqueueNotification(n)
	tr.enqueueNotification(n) // A full best-effort channel drops this entry.
	if got := retainedNotificationBytes(tr); got != size {
		t.Fatalf("best-effort drop retained %d bytes", got)
	}
	tr.handleNotification(<-tr.notifQueue)
	for range 127 {
		tr.enqueueNotification(n)
		tr.handleNotification(<-tr.notifQueue)
	}
	oldBacking := tr.pendingNotifHandle
	tr.enqueueNotification(n)
	tr.handleNotification(<-tr.notifQueue)
	if got := retainedNotificationBytes(tr); got != 128*size {
		t.Fatalf("eviction retention = %d bytes", got)
	}
	if oldBacking[0].Params != nil || oldBacking[0].reservation != nil {
		t.Fatal("evicted backing-array slot still owns payload")
	}
	tr.OnNotify(func(context.Context, Notification) { panic("recovered") })
	// Replay keeps one entry in the channel and drops the remainder. It must
	// neither reserve a second time nor retain detached replay references.
	if got := retainedNotificationBytes(tr); got != size {
		t.Fatalf("replay retention = %d bytes", got)
	}
	tr.handleNotification(<-tr.notifQueue)
	if got := retainedNotificationBytes(tr); got != 0 {
		t.Fatalf("panic recovery retained %d bytes", got)
	}
}

func TestNotificationBudgetCloseDuringEOFBacklogDrain(t *testing.T) {
	tr := newBudgetTestTransport(t)
	// Force streaming fallback into its backlog, then enter the real EOF
	// draining path. Close must free the rest while the first callback blocks.
	tr.streamingBacklog.draining = true
	n := Notification{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":99}`)}
	for range 4 {
		tr.enqueueNotification(n)
	}
	const size = 23 + 15
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	tr.OnNotify(func(context.Context, Notification) { close(entered); <-release })
	tr.stopAfterReaderEOF()
	waitAuditSignal(t, entered)
	_ = tr.Close()
	if got := retainedNotificationBytes(tr); got != size {
		t.Fatalf("Close retained %d bytes; want only active callback %d", got, size)
	}
	unblock()
	waitNotificationBytes(t, tr, 0)
	tr.enqueueNotification(n)
	if got := retainedNotificationBytes(tr); got != 0 {
		t.Fatalf("stopped transport admitted %d bytes", got)
	}
}

func TestNotificationBudgetChargesBackingCapacityAndOwner(t *testing.T) {
	tr := newBudgetTestTransport(t)
	params := make(json.RawMessage, 0, 1024)
	params = append(params, `{"threadId":"a"}`...)
	tr.enqueueNotification(Notification{Method: "item/agentMessage/delta", Params: params})
	// The allocation capacity and separately parsed "thread:a" key count.
	if got := retainedNotificationBytes(tr); got != 1024+23+8 {
		t.Fatalf("backing allocation/owner retention = %d", got)
	}
	queue := tr.turnNotifQueues["thread:a"]
	tr.clearTurnScopedNotificationQueue(queue)
	if got := retainedNotificationBytes(tr); got != 0 || queue.threadKey != "" {
		t.Fatalf("clear retained bytes=%d owner=%q", got, queue.threadKey)
	}
	tr.enqueueNotification(Notification{Method: "item/agentMessage/delta", Params: params})
	_ = tr.Close()
	if got := retainedNotificationBytes(tr); got != 0 {
		t.Fatalf("Close retained %d bytes", got)
	}
}

func TestNotificationBudgetCancellationDoesNotLoseReservation(t *testing.T) {
	for range 1000 {
		tr := newBudgetTestTransport(t)
		tr.enqueueNotification(Notification{Method: "unknown", Params: json.RawMessage(`{"value":1}`)})
		done := make(chan struct{})
		go func() { tr.handleNotificationQueue(tr.notifQueue); close(done) }()
		tr.cancelCtx()
		_ = tr.Close()
		waitAuditSignal(t, done)
		if got := retainedNotificationBytes(tr); got != 0 {
			t.Fatalf("cancellation lost ownership of %d bytes", got)
		}
	}
}

func TestNotificationBudgetFallbackReplayAfterEOF(t *testing.T) {
	for range 100 {
		tr := newBudgetTestTransport(t)
		tr.enqueueNotification(Notification{Method: "unknown", Params: json.RawMessage(`{"value":1}`)})
		tr.handleNotification(<-tr.notifQueue)
		tr.stopAfterReaderEOF()
		tr.OnNotify(func(context.Context, Notification) {})
		// Existing fallback policy cancels replay after reader stop. No entry
		// may transfer into a channel whose workers and EOF drain have exited.
		if got := retainedNotificationBytes(tr); got != 0 {
			t.Fatalf("EOF replay stranded %d bytes", got)
		}
	}
}

func TestStdioNotificationBudgetStreamingDrainAtEOF(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, release := make(chan struct{}, 8), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var delivered atomic.Int32
	tr.OnNotify(func(context.Context, Notification) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		delivered.Add(1)
	})
	const count = 1200
	for range count {
		writeAuditNotification(t, w, Notification{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":99,"delta":"x"}`)})
	}
	waitAuditSignal(t, entered)
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	if err := tr.ScanErr(); err != nil {
		t.Fatal(err)
	}
	unblock()
	waitNotificationBytes(t, tr, 0)
	if got := delivered.Load(); got != count {
		t.Fatalf("EOF delivered %d notifications; want %d", got, count)
	}
}

func newBudgetTestTransport(t *testing.T) *StdioTransport {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	tr := &StdioTransport{ctx: ctx, cancelCtx: cancel, readerStopped: make(chan struct{}),
		notifQueue: make(chan bufferedNotification, 1), streamingNotifQueue: make(chan bufferedNotification, 1),
		protectedNotifQueue: make(chan bufferedNotification, 1), criticalNotifQueue: make(chan bufferedNotification, 1)}
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}

func retainedNotificationBytes(tr *StdioTransport) int {
	tr.notificationBudget.mu.Lock()
	defer tr.notificationBudget.mu.Unlock()
	return tr.notificationBudget.bytes
}

func waitNotificationBytes(t *testing.T, tr *StdioTransport, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if retainedNotificationBytes(tr) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("retained bytes=%d; want %d", retainedNotificationBytes(tr), want)
}
