package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

const invalidRejectionFrame = `{"jsonrpc":"1.0","id":"bad","method":"test"}` + "\n"

func TestRejectionTerminalDiscardsQueuedPayloads(t *testing.T) {
	for _, ending := range []string{"close", "eof", "read-error", "write-error"} {
		t.Run(ending, func(t *testing.T) {
			reader, peer := io.Pipe()
			var writeErr error
			if ending == "write-error" {
				writeErr = io.ErrClosedPipe
			}
			writer := newBoundaryWriter(writeErr)
			tr := NewStdioTransport(reader, writer)
			defer func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() }()
			done := make(chan error, 1)
			go func() { done <- tr.Notify(context.Background(), Notification{Method: "held"}) }()
			waitBoundary(t, writer.held)
			if _, err := io.WriteString(peer, invalidRejectionFrame+invalidRejectionFrame); err != nil {
				t.Fatal(err)
			}
			awaitPayloadCondition(t, func() bool { return len(tr.writeQueue) == 2 })
			if retainedRejectionBytes(tr) == 0 {
				t.Fatal("queued payloads have no reservation")
			}
			switch ending {
			case "close":
				_ = tr.Close()
			case "eof":
				_ = peer.Close()
			case "read-error":
				_ = peer.CloseWithError(io.ErrUnexpectedEOF)
			case "write-error":
				writer.unblock()
			}
			waitBoundary(t, tr.ReaderStopped())
			awaitPayloadCondition(t, func() bool { return len(tr.writeQueue) == 0 && retainedRejectionBytes(tr) == 0 })
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("abandoned notification reported written success")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("terminal notification did not finish")
			}
			tr.enqueueRejection(RequestID{Value: "late"}, ErrCodeInvalidRequest, "invalid")
			if retainedRejectionBytes(tr) != 0 || len(tr.writeQueue) != 0 {
				t.Fatal("late rejection retained a payload after terminal admission")
			}
		})
	}
}

func TestClaimedRejectionRetainsReservationUntilWriteReturns(t *testing.T) {
	reader, peer := io.Pipe()
	writer := newBoundaryWriter(nil)
	tr := NewStdioTransport(reader, writer)
	defer func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() }()
	if _, err := io.WriteString(peer, invalidRejectionFrame); err != nil {
		t.Fatal(err)
	}
	waitBoundary(t, writer.held)
	charged := retainedRejectionBytes(tr)
	if charged == 0 {
		t.Fatal("active rejection lost its reservation")
	}
	remainder, err := tr.rejectionBudget.reserve(maxRetainedRejectionBytes - charged)
	if err != nil {
		t.Fatal(err)
	}
	defer remainder.release()
	if _, err := tr.rejectionBudget.reserve(1); !errors.Is(err, errRejectionByteLimit) {
		t.Fatalf("active payload was not included in byte limit: %v", err)
	}
	remainder.release()
	_ = tr.Close()
	if got := retainedRejectionBytes(tr); got != charged {
		t.Fatalf("Close forgot a claimed external write: bytes=%d want%d", got, charged)
	}
	writer.unblock()
	awaitPayloadCondition(t, func() bool { return retainedRejectionBytes(tr) == 0 })
}

func TestRejectionByteExhaustionTerminatesAdmission(t *testing.T) {
	reader, peer := io.Pipe()
	var output safeBuffer
	tr := NewStdioTransport(reader, &output)
	defer func() { _ = tr.Close(); _ = peer.Close() }()
	// Charge the byte owner without allocating a giant fixture. The public
	// reader must reject a real reply while that owner has no space left.
	reserved, err := tr.rejectionBudget.reserve(maxRetainedRejectionBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer reserved.release()
	if _, err := io.WriteString(peer, invalidRejectionFrame); err != nil {
		t.Fatal(err)
	}
	waitBoundary(t, tr.ReaderStopped())
	if !errors.Is(tr.ScanErr(), errRejectionByteLimit) || output.String() != "" {
		t.Fatalf("byte exhaustion failed admission incorrectly: cause=%v output=%q", tr.ScanErr(), output.String())
	}
	reserved.release()
	if retainedRejectionBytes(tr) != 0 {
		t.Fatal("released byte owner remained charged")
	}
}

func TestRejectionFullQueueAdmissionUnblocksOnClose(t *testing.T) {
	reader, peer := io.Pipe()
	writer := newBoundaryWriter(nil)
	tr := NewStdioTransport(reader, writer)
	defer func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() }()
	notifyDone := make(chan error, 1)
	go func() { notifyDone <- tr.Notify(context.Background(), Notification{Method: "held"}) }()
	waitBoundary(t, writer.held)
	for range outboundWriteQueueSize {
		if _, err := tr.queueWrite(tr.ctx, Notification{Method: "queued"}, "fixture", false); err != nil {
			t.Fatal(err)
		}
	}
	admitted := make(chan struct{})
	go func() {
		defer close(admitted)
		tr.enqueueRejection(RequestID{Value: "waiting"}, ErrCodeInvalidRequest, "invalid")
	}()
	awaitPayloadCondition(t, func() bool { return retainedRejectionBytes(tr) > 0 })
	select {
	case <-admitted:
		t.Fatal("full queue did not retain admission backpressure")
	default:
	}
	_ = tr.Close()
	waitBoundary(t, admitted)
	if retainedRejectionBytes(tr) != 0 || len(tr.writeQueue) != 0 {
		t.Fatal("terminal admission left rejection payloads behind")
	}
	select {
	case <-notifyDone:
	case <-time.After(3 * time.Second):
		t.Fatal("held notification did not observe terminal state")
	}
}

type dequeueStopContext struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func (c *dequeueStopContext) Done() <-chan struct{} { return c.done }
func (c *dequeueStopContext) Err() error {
	c.once.Do(func() { close(c.done) })
	return context.Canceled
}

func TestRejectionDequeuedAtCancellationIsRetired(t *testing.T) {
	ctx := &dequeueStopContext{Context: context.Background(), done: make(chan struct{})}
	var output safeBuffer
	tr := &StdioTransport{ctx: ctx, writer: &output, writeQueue: make(chan writeEnvelope, 1)}
	reservation, err := tr.rejectionBudget.reserve(16)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	tr.writeQueue <- writeEnvelope{payload: []byte(`{"id":"unused"}`), done: done, rejection: reservation}
	tr.writeLoop() // Err becomes canceled only after the envelope is received.
	if output.String() != "" || retainedRejectionBytes(tr) != 0 || len(tr.writeQueue) != 0 {
		t.Fatal("dequeued abandoned reply was written or retained")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("abandoned reply reported written success")
		}
	default:
		t.Fatal("dequeued abandoned reply lost its completion")
	}
}

func TestRequestWorkerReplyStillWaitsForPhysicalWrite(t *testing.T) {
	reader, peer := io.Pipe()
	writer := newBoundaryWriter(nil)
	tr := NewStdioTransport(reader, writer)
	defer func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() }()
	tr.OnRequest(func(context.Context, Request) (Response, error) { return Response{Result: json.RawMessage(`{}`)}, nil })
	done := make(chan struct{})
	go func() { defer close(done); tr.handleRequest(Request{ID: RequestID{Value: "worker"}, Method: "test"}) }()
	waitBoundary(t, writer.held)
	select {
	case <-done:
		t.Fatal("worker treated queued admission as written completion")
	default:
	}
	writer.unblock()
	waitBoundary(t, done)
}

type rejectionFailureWriter struct {
	*boundaryWriter
	frames int
}

func (w *rejectionFailureWriter) Write(p []byte) (int, error) {
	if len(p) == 1 && p[0] == '\n' {
		w.frames++
		if w.frames == 2 {
			return 0, io.ErrClosedPipe
		}
	}
	return w.boundaryWriter.Write(p)
}

func TestRejectionWriteFailurePreservesAcceptedResponse(t *testing.T) {
	reader, peer := io.Pipe()
	writer := &rejectionFailureWriter{boundaryWriter: newBoundaryWriter(nil)}
	tr := NewStdioTransport(reader, writer)
	defer func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() }()
	outcome := make(chan boundaryOutcome, 1)
	go func() {
		response, err := tr.Send(context.Background(), Request{ID: RequestID{Value: "wanted"}, Method: "test"})
		outcome <- boundaryOutcome{response, err}
	}()
	waitBoundary(t, writer.held)
	if _, err := io.WriteString(peer, invalidRejectionFrame+`{"id":"wanted","result":{"ok":true}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-outcome:
		if got.err != nil || string(got.resp.Result) != `{"ok":true}` {
			t.Fatalf("response was not accepted before rejection write: %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("response was blocked behind rejection write")
	}
	writer.unblock()
	waitBoundary(t, tr.ReaderStopped())
	awaitPayloadCondition(t, func() bool { return retainedRejectionBytes(tr) == 0 })
	if !errors.Is(tr.ScanErr(), io.ErrClosedPipe) {
		t.Fatalf("actual rejection write failure was lost: %v", tr.ScanErr())
	}
	if _, err := tr.Send(context.Background(), Request{ID: RequestID{Value: "later"}, Method: "test"}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("later Send lost terminal writer failure: %v", err)
	}
}
