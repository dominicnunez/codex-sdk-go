package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedBoundaryWriter struct {
	*boundaryWriter
	data safeBuffer
}

func (w *recordedBoundaryWriter) Write(p []byte) (int, error) {
	n, err := w.boundaryWriter.Write(p)
	if n != 0 {
		_, _ = w.data.Write(p[:n])
	}
	return n, err
}

func retainedRejectionBytes(t *StdioTransport) int {
	t.rejectionBudget.mu.Lock()
	defer t.rejectionBudget.mu.Unlock()
	return t.rejectionBudget.bytes
}

func TestRejectionReaderProgress(t *testing.T) {
	for _, mode := range []string{"response-only", "invalid-notification-version", "handler-error", "invalid-version", "invalid-version-type", "invalid-method-type", "invalid-method-null", "invalid-id", "array-id", "boolean-id", "missing-method", "overload"} {
		t.Run(mode, func(t *testing.T) {
			reader, peer := io.Pipe()
			writer := &recordedBoundaryWriter{boundaryWriter: newBoundaryWriter(nil)}
			transport := NewStdioTransport(reader, writer)
			releaseHandlers := make(chan struct{})
			var once sync.Once
			unblockHandlers := func() { once.Do(func() { close(releaseHandlers) }) }
			defer func() { writer.unblock(); unblockHandlers(); _ = transport.Close(); _ = peer.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			entered := make(chan struct{}, inboundRequestWorkers)
			transport.OnRequest(func(_ context.Context, req Request) (Response, error) {
				if mode == "handler-error" {
					return Response{}, errors.New("fixture handler failure")
				}
				select {
				case entered <- struct{}{}:
				default:
				}
				<-releaseHandlers
				return Response{ID: req.ID, Result: json.RawMessage(`{}`)}, nil
			})
			done := make(chan boundaryOutcome, 1)
			go func() {
				r, e := transport.Send(ctx, Request{ID: RequestID{Value: "wanted"}, Method: "test/request"})
				done <- boundaryOutcome{r, e}
			}()
			waitBoundary(t, writer.held)
			response := `{"id":"wanted","result":{"ok":true}}` + "\n"
			prefix := ""
			var expectedID any = "bad"
			expectedCode := ErrCodeInvalidRequest
			switch mode {
			case "invalid-notification-version":
				prefix = `{"jsonrpc":"1.0","method":"test"}` + "\n"
			case "handler-error":
				prefix = `{"id":"bad","method":"test"}` + "\n"
				expectedCode = ErrCodeInternalError
			case "invalid-version":
				prefix = `{"jsonrpc":"1.0","id":"bad","method":"test"}` + "\n"
			case "invalid-version-type":
				prefix = `{"jsonrpc":17,"id":"bad","method":"test"}` + "\n"
			case "invalid-method-type":
				prefix = `{"id":"bad","method":17}` + "\n"
			case "invalid-method-null":
				prefix = `{"id":"bad","method":null}` + "\n"
			case "invalid-id":
				prefix = `{"id":{"bad":1},"method":"test"}` + "\n"
				expectedID = nil
			case "array-id":
				prefix = `{"id":[1],"method":"test"}` + "\n"
				expectedID = nil
			case "boolean-id":
				prefix = `{"id":true,"method":"test"}` + "\n"
				expectedID = nil
			case "missing-method":
				prefix = `{"id":"bad"}` + "\n"
			case "overload":
				expectedID = 100 + inboundRequestQueueSize
				expectedCode = ErrCodeInternalError
				for n := range inboundRequestWorkers {
					if _, e := fmt.Fprintf(peer, "{\"id\":%d,\"method\":\"test\"}\n", n); e != nil {
						t.Fatal(e)
					}
				}
				for range inboundRequestWorkers {
					select {
					case <-entered:
					case <-ctx.Done():
						t.Fatal("workers not occupied")
					}
				}
				var b strings.Builder
				for n := range inboundRequestQueueSize + 1 {
					fmt.Fprintf(&b, "{\"id\":%d,\"method\":\"test\"}\n", n+100)
				}
				prefix = b.String()
			}
			written := make(chan error, 1)
			go func() { _, e := io.WriteString(peer, prefix+response); written <- e }()
			select {
			case e := <-written:
				if e != nil {
					t.Fatal(e)
				}
			case <-ctx.Done():
				t.Fatal("fixture input not delivered")
			}
			hasReply := mode != "response-only" && mode != "invalid-notification-version"
			if hasReply {
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for len(transport.writeQueue) == 0 {
					select {
					case <-ticker.C:
					case <-ctx.Done():
						t.Fatal("rejection reply not enqueued")
					}
				}
			}
			if hasReply && mode != "handler-error" && retainedRejectionBytes(transport) == 0 {
				t.Error("queued rejection was not charged to its byte owner")
			}
			var outcome boundaryOutcome
			early := false
			select {
			case outcome = <-done:
				early = true
			case <-time.After(200 * time.Millisecond):
				t.Errorf("unrelated valid response is blocked behind %s error reply while request writer remains pending", mode)
			}
			writer.unblock()
			if !early {
				select {
				case outcome = <-done:
				case <-ctx.Done():
					t.Fatal("response did not recover after writer release")
				}
			}
			if outcome.err != nil || string(outcome.resp.Result) != `{"ok":true}` {
				t.Fatalf("correlated response changed: %s error=%v", outcome.resp.Result, outcome.err)
			}
			if hasReply {
				awaitPayloadCondition(t, func() bool {
					return strings.Count(writer.data.String(), "\n") == 2 && retainedRejectionBytes(transport) == 0
				})
				frames := strings.Split(strings.TrimSpace(writer.data.String()), "\n")
				var reply Response
				if err := json.Unmarshal([]byte(frames[1]), &reply); err != nil {
					t.Fatal(err)
				}
				if reply.Error == nil || reply.Error.Code != expectedCode || fmt.Sprint(reply.ID.Value) != fmt.Sprint(expectedID) {
					t.Fatalf("wrong rejection reply: %+v", reply)
				}
			}
		})
	}
}
