package transport

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotificationLifetimeCountsEmptyReservations(t *testing.T) {
	var budget notificationBudget
	reservation, err := budget.reserve(0)
	if err != nil {
		t.Fatal(err)
	}
	done := budget.doneSignal()
	budget.end()
	select {
	case <-done:
		t.Fatal("zero-byte notification was still owned")
	default:
	}
	if _, err := budget.reserve(1); err == nil {
		t.Fatal("ended admission accepted a new reservation")
	}
	n := bufferedNotification{reservation: reservation}
	n.release()
	n.release()
	waitAuditSignal(t, done)
}

func TestTransportLifetimeEmptyTermination(t *testing.T) {
	for _, ending := range []string{"eof", "read-error", "close"} {
		t.Run(ending, func(t *testing.T) {
			r, w := io.Pipe()
			tr := NewStdioTransport(r, io.Discard)
			done := tr.Done()
			t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
			if tr.Err() != nil {
				t.Fatal("live transport has terminal error")
			}
			cause := errors.New("fixture read failure")
			switch ending {
			case "eof":
				_ = w.Close()
				cause = io.EOF
			case "read-error":
				_ = w.CloseWithError(cause)
			case "close":
				_ = tr.Close()
				cause = errTransportClosed
			}
			waitAuditSignal(t, done)
			if !errors.Is(tr.Err(), cause) {
				t.Fatalf("terminal error = %v, want %v", tr.Err(), cause)
			}
			if tr.Done() != done {
				t.Fatal("terminal signal changed")
			}
		})
	}
}

func TestTransportLifetimeWaitsForEOFReplay(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	if _, err := io.WriteString(w, `{"method":"item/plan/delta","params":{"threadId":"thread","turnId":"turn","itemId":"plan","delta":"x"}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	waitAuditSignal(t, tr.ReaderStopped())
	select {
	case <-tr.Done():
		t.Fatal("EOF forgot queued notification awaiting handler registration")
	default:
	}
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	tr.OnNotify(func(context.Context, Notification) { close(entered); <-release })
	waitAuditSignal(t, entered)
	select {
	case <-tr.Done():
		t.Fatal("delivery finished before the accepted callback returned")
	default:
	}
	unblock()
	waitAuditSignal(t, tr.Done())
	if !errors.Is(tr.Err(), io.EOF) {
		t.Fatalf("EOF cause lost: %v", tr.Err())
	}
}

func TestTransportReentrantCloseRetiresClaimedCallback(t *testing.T) {
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, closeNow, closed, release := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	startClose := sync.OnceFunc(func() { close(closeNow) })
	defer startClose()
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	var calls atomic.Int32
	tr.OnNotify(func(context.Context, Notification) {
		calls.Add(1)
		close(entered)
		<-closeNow
		_ = tr.Close() // Must not wait for the callback that invoked it.
		close(closed)
		<-release
		panic("fixture callback panic")
	})
	for i := range 2 {
		if _, err := io.WriteString(w, `{"method":"item/plan/delta","params":{"threadId":"thread","turnId":"turn","itemId":"plan","delta":"x"}}`+"\n"); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			waitAuditSignal(t, entered)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		tr.notificationBudget.mu.Lock()
		active := tr.notificationBudget.active
		tr.notificationBudget.mu.Unlock()
		if active == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second notification was not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	startClose()
	waitAuditSignal(t, closed)
	select {
	case <-tr.Done():
		t.Fatal("claimed callback has not retired")
	default:
	}
	unblock()
	waitAuditSignal(t, tr.Done())
	if calls.Load() != 1 {
		t.Fatalf("close delivered discarded notification: callbacks=%d", calls.Load())
	}
}
