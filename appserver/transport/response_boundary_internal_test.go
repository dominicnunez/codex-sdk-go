package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Read two starts only after the first complete inbound line has been processed.
// The pipe returns exactly the single line written by each test.
type boundaryReader struct {
	*io.PipeReader
	processed chan struct{}
	reads     atomic.Int32
	eofError  error
}

func (r *boundaryReader) Read(p []byte) (int, error) {
	if r.reads.Add(1) == 2 {
		close(r.processed)
	}
	n, err := r.PipeReader.Read(p)
	if errors.Is(err, io.EOF) && r.eofError != nil {
		err = r.eofError
	}
	return n, err
}

// A full frame reaches the external writer, but its delimiter call remains in
// flight until released. It can then report bytes consumed together with error.
type boundaryWriter struct {
	held        chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
	heldOnce    sync.Once
	err         error
}

func newBoundaryWriter(err error) *boundaryWriter {
	return &boundaryWriter{held: make(chan struct{}), release: make(chan struct{}), err: err}
}

func (w *boundaryWriter) Write(p []byte) (int, error) {
	if len(p) == 1 && p[0] == '\n' {
		w.heldOnce.Do(func() { close(w.held) })
		<-w.release
		return len(p), w.err
	}
	return len(p), nil
}

func (w *boundaryWriter) unblock() { w.releaseOnce.Do(func() { close(w.release) }) }

func waitBoundary(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for synchronized transport boundary")
	}
}

func assertBoundaryPendingEmpty(t *testing.T, tr *StdioTransport) {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.pendingReqs) != 0 {
		t.Fatalf("terminal request retained %d pending owners", len(tr.pendingReqs))
	}
}

type boundaryOutcome struct {
	resp Response
	err  error
}

// Pause the completion select only after queue admission. This context has an
// existing deadline, so the transport does not wrap away its synchronization.
type boundarySelectionContext struct {
	context.Context
	calls           atomic.Int32
	paused, release chan struct{}
}

func (c *boundarySelectionContext) Done() <-chan struct{} {
	if c.calls.Add(1) == 2 {
		close(c.paused)
		<-c.release
	}
	return c.Context.Done()
}

func boundarySend(t *testing.T, tr *StdioTransport) (*boundarySelectionContext, <-chan boundaryOutcome, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	paused := &boundarySelectionContext{Context: ctx, paused: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	unpause := func() { once.Do(func() { close(paused.release) }) }
	t.Cleanup(unpause)
	done := make(chan boundaryOutcome, 1)
	go func() {
		resp, err := tr.Send(paused, Request{ID: RequestID{Value: "boundary"}, Method: "test/boundary"})
		done <- boundaryOutcome{resp, err}
	}()
	return paused, done, unpause
}

func receiveBoundary(t *testing.T, done <-chan boundaryOutcome) boundaryOutcome {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("Send did not complete at its terminal boundary")
		return boundaryOutcome{}
	}
}

func writeBoundaryResponse(t *testing.T, peer *io.PipeWriter, reader *boundaryReader) {
	t.Helper()
	if _, err := io.WriteString(peer, "{\"id\":\"boundary\",\"result\":{\"ok\":true}}\n"); err != nil {
		t.Fatal(err)
	}
	waitBoundary(t, reader.processed)
}

func TestSendAcceptedResponseAcrossActualReadFailure(t *testing.T) {
	readErr := errors.New("boundary non-EOF read failure")
	r, peer := io.Pipe()
	reader := &boundaryReader{PipeReader: r, processed: make(chan struct{}), eofError: readErr}
	writer := newBoundaryWriter(nil)
	tr := NewStdioTransport(reader, writer)
	t.Cleanup(func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() })
	paused, done, unpause := boundarySend(t, tr)
	waitBoundary(t, writer.held)
	waitBoundary(t, paused.paused)
	writeBoundaryResponse(t, peer, reader)
	_ = peer.Close()
	waitBoundary(t, tr.ReaderStopped())
	if !errors.Is(tr.ScanErr(), readErr) {
		t.Fatalf("ScanErr=%v; want real read failure", tr.ScanErr())
	}
	unpause()
	got := receiveBoundary(t, done)
	if got.err != nil || string(got.resp.Result) != `{"ok":true}` {
		t.Fatalf("accepted response lost across read failure: %+v", got)
	}
	assertBoundaryPendingEmpty(t, tr)
}

func TestSendActualCloseAndWriteFailureOrder(t *testing.T) {
	for _, cause := range []string{"close", "write failure"} {
		for _, accepted := range []bool{false, true} {
			t.Run(cause+"/accepted="+strconv.FormatBool(accepted), func(t *testing.T) {
				writeErr := errors.New("boundary writer failure")
				var writerErr error
				if cause == "write failure" {
					writerErr = writeErr
				}
				r, peer := io.Pipe()
				reader := &boundaryReader{PipeReader: r, processed: make(chan struct{})}
				writer := newBoundaryWriter(writerErr)
				tr := NewStdioTransport(reader, writer)
				t.Cleanup(func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() })
				paused, done, unpause := boundarySend(t, tr)
				waitBoundary(t, writer.held)
				waitBoundary(t, paused.paused)
				if accepted {
					writeBoundaryResponse(t, peer, reader)
				}
				wantCause := errTransportClosed
				if cause == "close" {
					if err := tr.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					wantCause = writeErr
					writer.unblock()
				}
				waitBoundary(t, tr.ReaderStopped())
				if !accepted {
					// A response that arrives after real shutdown cannot claim ownership.
					tr.handleResponse(Response{ID: RequestID{Value: "boundary"}, Result: json.RawMessage(`true`)})
				}
				unpause()
				got := receiveBoundary(t, done)
				if accepted {
					if got.err != nil || string(got.resp.Result) != `{"ok":true}` {
						t.Fatalf("accepted outcome=%+v", got)
					}
				} else {
					var transportErr *TransportError
					if !errors.Is(got.err, wantCause) || !errors.As(got.err, &transportErr) || len(got.resp.Result) != 0 {
						t.Fatalf("unaccepted outcome=%+v; want transport cause %v", got, wantCause)
					}
				}
				if cause == "write failure" && !errors.Is(tr.ScanErr(), writeErr) {
					t.Fatalf("ScanErr=%v", tr.ScanErr())
				}
				if cause == "close" && tr.ScanErr() != nil {
					t.Fatalf("explicit close recorded IO failure: %v", tr.ScanErr())
				}
				assertBoundaryPendingEmpty(t, tr)
			})
		}
	}
}

type boundaryWriteFunc func([]byte) (int, error)

func (f boundaryWriteFunc) Write(p []byte) (int, error) { return f(p) }

func TestSendUnusablePartialZeroAndFailedWrites(t *testing.T) {
	writeErr := errors.New("boundary incomplete write")
	for _, mode := range []string{"zero", "failed", "partial error", "short then failed"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			writer := boundaryWriteFunc(func(p []byte) (int, error) {
				call := calls.Add(1)
				switch mode {
				case "zero":
					return 0, nil
				case "partial error":
					return len(p) / 2, writeErr
				case "short then failed":
					if call == 1 {
						return len(p) / 2, nil
					}
				}
				return 0, writeErr
			})
			r, peer := io.Pipe()
			tr := NewStdioTransport(r, writer)
			t.Cleanup(func() { _ = tr.Close(); _ = peer.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			resp, err := tr.Send(ctx, Request{ID: RequestID{Value: "incomplete"}, Method: "test/incomplete"})
			want := writeErr
			if mode == "zero" {
				want = errZeroByteWrite
			}
			var transportErr *TransportError
			if !errors.Is(err, want) || !errors.As(err, &transportErr) || len(resp.Result) != 0 || resp.Error != nil {
				t.Fatalf("incomplete write response=%+v err=%v; want %v", resp, err, want)
			}
			waitBoundary(t, tr.ReaderStopped())
			if !errors.Is(tr.ScanErr(), want) {
				t.Fatalf("ScanErr=%v; want %v", tr.ScanErr(), want)
			}
			if mode == "short then failed" && calls.Load() != 2 {
				t.Fatalf("short write calls=%d", calls.Load())
			}
			assertBoundaryPendingEmpty(t, tr)
		})
	}
}

func TestNotifyAndInternalReplyRequireActualWrite(t *testing.T) {
	for _, kind := range []string{"notify", "internal success reply", "internal error reply"} {
		t.Run(kind, func(t *testing.T) {
			r, peer := io.Pipe()
			reader := &boundaryReader{PipeReader: r, processed: make(chan struct{})}
			writer := newBoundaryWriter(nil)
			tr := NewStdioTransport(reader, writer)
			t.Cleanup(func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			if kind == "notify" {
				go func() { done <- tr.Notify(ctx, Notification{Method: "test/notify"}) }()
			} else {
				tr.OnRequest(func(_ context.Context, req Request) (Response, error) {
					if kind == "internal error reply" {
						return Response{}, errors.New("boundary application handler error")
					}
					return Response{ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}, nil
				})
				// Observe completion at the real worker dispatch boundary, which has
				// no public completion callback. Registration and write loop are real.
				go func() { tr.handleRequest(Request{ID: RequestID{Value: "reply"}, Method: "test/reply"}); done <- nil }()
			}
			waitBoundary(t, writer.held)
			writeBoundaryResponse(t, peer, reader) // unrelated inbound correlation cannot complete either write
			select {
			case err := <-done:
				t.Fatalf("%s completed before writer return: %v", kind, err)
			default:
			}
			writer.unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if tr.ScanErr() != nil {
				t.Fatalf("successful write ScanErr=%v", tr.ScanErr())
			}
		})
	}
}
