package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func awaitPayloadCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("payload ownership condition not reached")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTerminalUnblocksFullWriteQueueAndRejectsLateAdmission(t *testing.T) {
	reader, peer := io.Pipe()
	writer := newBoundaryWriter(nil)
	tr := NewStdioTransport(reader, writer)
	t.Cleanup(func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() })
	const producers = outboundWriteQueueSize + 16
	result := make(chan error, producers+1)
	go func() { result <- tr.Notify(context.Background(), Notification{Method: "held"}) }()
	waitBoundary(t, writer.held)
	for i := range producers {
		go func() {
			if i%2 == 0 {
				result <- tr.Notify(context.Background(), Notification{Method: "queued"})
			} else {
				result <- tr.writeMessage(Response{ID: RequestID{Value: i}, Result: json.RawMessage(`{}`)})
			}
		}()
	}
	awaitPayloadCondition(t, func() bool { return len(tr.writeQueue) == outboundWriteQueueSize })
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	for range producers + 1 {
		select {
		case err := <-result:
			if err == nil {
				t.Fatal("abandoned write succeeded")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("full-queue producer did not unblock")
		}
	}
	if len(tr.writeQueue) != 0 {
		t.Fatal("late producer survived terminal drain")
	}
	if err := tr.Notify(context.Background(), Notification{Method: "after stop"}); err == nil {
		t.Fatal("terminal Notify succeeded")
	}
	if len(tr.writeQueue) != 0 {
		t.Fatal("terminal admission retained a payload")
	}
}

func TestTerminalRequestReplayRacesAdmissionAndStop(t *testing.T) {
	for range 30 {
		reader, peer := io.Pipe()
		tr := NewStdioTransport(reader, io.Discard)
		for range 4 {
			tr.enqueueRequest(Request{Method: "approval"})
		}
		awaitPayloadCondition(t, func() bool { tr.mu.Lock(); defer tr.mu.Unlock(); return len(tr.pendingReqHandler) == 4 })
		start := make(chan struct{})
		var workers sync.WaitGroup
		for range 3 {
			workers.Go(func() {
				<-start
				for range 20 {
					tr.enqueueRequest(Request{Method: "racing"})
				}
			})
		}
		workers.Go(func() {
			<-start
			tr.OnRequest(func(context.Context, Request) (Response, error) { return Response{}, nil })
		})
		workers.Go(func() { <-start; _ = tr.Close() })
		close(start)
		workers.Wait()
		waitBoundary(t, tr.ReaderStopped())
		tr.mu.Lock()
		retained := len(tr.pendingReqHandler)
		tr.mu.Unlock()
		if retained != 0 || len(tr.requestQueue) != 0 || len(tr.writeQueue) != 0 {
			t.Fatal("racing replay/admission retained terminal payloads")
		}
		_ = peer.Close()
	}
}

func TestTerminalReleasesPreHandlerRequests(t *testing.T) {
	for _, cause := range []string{"close", "eof", "read failure", "oversize", "write failure"} {
		t.Run(cause, func(t *testing.T) {
			reader, peer := io.Pipe()
			tr := NewStdioTransport(reader, io.Discard)
			t.Cleanup(func() { _ = tr.Close(); _ = peer.Close() })
			for id := range 4 {
				tr.enqueueRequest(Request{ID: RequestID{Value: id}, Method: "approval", Params: json.RawMessage(`{"data":"retained"}`)})
			}
			awaitPayloadCondition(t, func() bool { tr.mu.Lock(); defer tr.mu.Unlock(); return len(tr.pendingReqHandler) == 4 })
			stopPayloadTransport(t, tr, peer, cause)
			tr.mu.Lock()
			retained := len(tr.pendingReqHandler)
			tr.mu.Unlock()
			if retained != 0 {
				t.Fatalf("retained %d pre-handler requests", retained)
			}
			var calls atomic.Int32
			tr.OnRequest(func(context.Context, Request) (Response, error) { calls.Add(1); return Response{}, nil })
			tr.enqueueRequest(Request{Method: "late"})
			tr.handleRequest(Request{Method: "late direct worker"})
			tr.mu.Lock()
			retained = len(tr.pendingReqHandler)
			tr.mu.Unlock()
			if calls.Load() != 0 || retained != 0 || len(tr.requestQueue) != 0 {
				t.Fatal("terminal request was admitted or retained")
			}
		})
	}
}

func stopPayloadTransport(t *testing.T, tr *StdioTransport, peer *io.PipeWriter, cause string) {
	t.Helper()
	switch cause {
	case "close":
		_ = tr.Close()
	case "eof":
		_ = peer.Close()
	case "read failure":
		_ = peer.CloseWithError(errors.New("read failure"))
	case "oversize":
		_, _ = peer.Write(append(bytes.Repeat([]byte{'x'}, maxInboundMessageSizeBytes+1), '\n'))
	case "write failure":
		tr.handleWriteFailure(errors.New("write failure"))
	}
	waitBoundary(t, tr.ReaderStopped())
}

func TestTerminalReleasesQueuedRequests(t *testing.T) {
	for _, cause := range []string{"close", "eof", "read failure", "oversize", "write failure"} {
		t.Run(cause, func(t *testing.T) {
			reader, peer := io.Pipe()
			tr := NewStdioTransport(reader, io.Discard)
			release := make(chan struct{})
			entered := make(chan struct{}, inboundRequestWorkers)
			t.Cleanup(func() { close(release); _ = tr.Close(); _ = peer.Close() })
			var calls atomic.Int32
			tr.OnRequest(func(context.Context, Request) (Response, error) {
				calls.Add(1)
				entered <- struct{}{}
				<-release
				return Response{}, context.Canceled
			})
			for range inboundRequestWorkers {
				tr.enqueueRequest(Request{Method: "held"})
				waitBoundary(t, entered)
			}
			for range inboundRequestQueueSize {
				tr.enqueueRequest(Request{Method: "queued", Params: json.RawMessage(`{"data":"retained"}`)})
			}
			if len(tr.requestQueue) != inboundRequestQueueSize {
				t.Fatal("fixture queue not full")
			}
			stopPayloadTransport(t, tr, peer, cause)
			if len(tr.requestQueue) != 0 {
				t.Fatalf("retained %d queued requests", len(tr.requestQueue))
			}
			tr.enqueueRequest(Request{Method: "late"})
			if len(tr.requestQueue) != 0 || calls.Load() != inboundRequestWorkers {
				t.Fatal("terminal admitted extra callback")
			}
		})
	}
}

func TestTerminalReleasesQueuedWrites(t *testing.T) {
	for _, cause := range []string{"close", "eof", "read failure", "oversize", "write failure"} {
		t.Run(cause, func(t *testing.T) {
			reader, peer := io.Pipe()
			writer := newBoundaryWriter(nil)
			if cause == "write failure" {
				writer.err = errors.New("actual writer failure")
			}
			tr := NewStdioTransport(reader, writer)
			t.Cleanup(func() { writer.unblock(); _ = tr.Close(); _ = peer.Close() })
			result := make(chan error, 9)
			go func() { result <- tr.Notify(context.Background(), Notification{Method: "held"}) }()
			waitBoundary(t, writer.held)
			for range 8 {
				go func() {
					result <- tr.Notify(context.Background(), Notification{Method: "queued", Params: json.RawMessage(`{"data":"retained"}`)})
				}()
			}
			awaitPayloadCondition(t, func() bool { return len(tr.writeQueue) == 8 })
			if cause == "write failure" {
				writer.unblock()
				waitBoundary(t, tr.ReaderStopped())
			} else {
				stopPayloadTransport(t, tr, peer, cause)
			}
			for range 9 {
				select {
				case err := <-result:
					if err == nil {
						t.Fatal("abandoned write succeeded")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("write caller stuck")
				}
			}
			if len(tr.writeQueue) != 0 {
				t.Fatalf("retained %d queued writes", len(tr.writeQueue))
			}
		})
	}
}

func TestTerminalExecutionClaimsPrecedeContextCancellation(t *testing.T) {
	// Terminal state is committed before stop cancels its context. Work received
	// in that window must be abandoned before calling the external writer.
	writer := &writeCountWriter{}
	tr := &StdioTransport{closed: true, ctx: context.Background(), writer: writer, writeQueue: make(chan writeEnvelope, 1)}
	var callbacks atomic.Int32
	tr.reqHandler = func(context.Context, Request) (Response, error) { callbacks.Add(1); return Response{}, nil }
	tr.handleRequest(Request{Method: "dequeued"})
	if callbacks.Load() != 0 || len(tr.pendingReqHandler) != 0 {
		t.Fatal("terminal dequeued work entered callback or replay storage")
	}
	tr.writeQueue <- writeEnvelope{payload: []byte("abandoned"), done: make(chan error, 1)}
	tr.writeLoop()
	if len(tr.writeQueue) != 0 || writer.calls.Load() != 0 {
		t.Fatal("terminal dequeued work entered external writer")
	}
}
