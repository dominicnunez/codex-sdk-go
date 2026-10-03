package transport

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// Pause Send before its completion select, leaving writer/reader/response
// readiness under the test's control rather than relying on goroutine timing.
type responseSelectionContext struct {
	context.Context
	calls   atomic.Int32
	pauseAt int32
	paused  chan struct{}
	release chan struct{}
}

func (c *responseSelectionContext) Done() <-chan struct{} {
	at := c.pauseAt
	if at == 0 {
		at = 2
	}
	if c.calls.Add(1) == at {
		close(c.paused)
		<-c.release
	}
	return c.Context.Done()
}

func outcomeTransport() *StdioTransport {
	ctx, cancel := context.WithCancel(context.Background())
	tr := &StdioTransport{ctx: ctx, cancelCtx: cancel, pendingReqs: make(map[string]pendingReq), writeQueue: make(chan writeEnvelope, 1), readerStopped: make(chan struct{})}
	tr.startReadLoopOnce.Do(func() {})
	return tr
}

func TestSendAcceptedOutcomeWithAllTerminalCasesReady(t *testing.T) {
	for _, wire := range []string{
		`{"id":"accepted","result":{"ok":true}}`,
		`{"id":"accepted","error":{"code":-32000,"message":"denied"}}`,
		`{"id":"accepted","result":null,"error":{}}`,
		`{"jsonrpc":"invalid","id":"accepted","result":null}`,
	} {
		for _, writerFailed := range []bool{false, true} {
			t.Run(wire+"/writeFailed="+boolName(writerFailed), func(t *testing.T) {
				tr := outcomeTransport()
				defer tr.cancelCtx()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				paused := &responseSelectionContext{Context: ctx, paused: make(chan struct{}), release: make(chan struct{})}
				type outcome struct {
					resp Response
					err  error
				}
				done := make(chan outcome, 1)
				go func() {
					resp, err := tr.Send(paused, Request{ID: RequestID{Value: "accepted"}, Method: "test"})
					done <- outcome{resp, err}
				}()
				var env writeEnvelope
				select {
				case env = <-tr.writeQueue:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				select {
				case <-paused.paused:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				tr.processInboundLine([]byte(wire))
				if writerFailed {
					env.done <- errors.New("writer failed after frame")
				} else {
					env.done <- nil
				}
				tr.mu.Lock()
				tr.closed = true
				tr.readerEOF = true
				tr.mu.Unlock()
				tr.cancelCtx()
				close(tr.readerStopped)
				cancel()
				close(paused.release)
				select {
				case got := <-done:
					if got.err != nil || got.resp.ID.Value != "accepted" {
						t.Fatalf("accepted outcome lost: %+v", got)
					}
					if wire == `{"id":"accepted","result":{"ok":true}}` {
						if string(got.resp.Result) != `{"ok":true}` || got.resp.Error != nil {
							t.Fatalf("response=%+v", got.resp)
						}
					} else {
						code := ErrCodeParseError
						message := "failed to parse server response"
						switch wire {
						case `{"id":"accepted","error":{"code":-32000,"message":"denied"}}`:
							code, message = -32000, "denied"
						case `{"jsonrpc":"invalid","id":"accepted","result":null}`:
							code, message = ErrCodeInvalidRequest, errInvalidResponseJSONRPC
						}
						if got.resp.Error == nil || got.resp.Error.Code != code || got.resp.Error.Message != message {
							t.Fatalf("accepted error changed: %+v want=%d/%s", got.resp, code, message)
						}
					}
				case <-time.After(5 * time.Second):
					t.Fatal("Send did not finish")
				}
			})
		}
	}
}

func boolName(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func TestSendAdmissionFailureCannotAcceptResponse(t *testing.T) {
	tr := outcomeTransport()
	defer tr.cancelCtx()
	// An invalid outbound value never enters the pending map or write queue.
	_, err := tr.Send(context.Background(), Request{ID: RequestID{Value: "invalid"}, Method: "test", Params: json.RawMessage(`{`)})
	if err == nil || len(tr.pendingReqs) != 0 || len(tr.writeQueue) != 0 {
		t.Fatalf("invalid admission err=%v pending=%d queued=%d", err, len(tr.pendingReqs), len(tr.writeQueue))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = tr.Send(ctx, Request{ID: RequestID{Value: "cancelled"}, Method: "test"})
	if !errors.Is(err, context.Canceled) || len(tr.pendingReqs) != 0 || len(tr.writeQueue) != 0 {
		t.Fatalf("cancelled admission err=%v", err)
	}
}

func TestSendQueueAdmissionFailureRejectsSpeculativeResponse(t *testing.T) {
	tr := outcomeTransport()
	defer tr.cancelCtx()
	tr.writeQueue <- writeEnvelope{} // no writer; outbound admission must wait
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := &responseSelectionContext{Context: ctx, pauseAt: 1, paused: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := tr.Send(paused, Request{ID: RequestID{Value: "speculative"}, Method: "test"})
		done <- err
	}()
	select {
	case <-paused.paused:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	tr.handleResponse(Response{ID: RequestID{Value: "speculative"}, Result: json.RawMessage(`true`)})
	cancel()
	close(paused.release)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unadmitted request accepted: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Send did not finish")
	}
	if len(tr.writeQueue) != 1 || len(tr.pendingReqs) != 0 {
		t.Fatal("failed queue admission retained request")
	}
}

func TestSendTerminalReadinessWithoutUsableResponse(t *testing.T) {
	for _, wire := range []string{`{"id":"foreign","result":true}`, `{"id":{},"result":true}`, `{}`} {
		t.Run(wire, func(t *testing.T) {
			tr := outcomeTransport()
			defer tr.cancelCtx()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			paused := &responseSelectionContext{Context: ctx, paused: make(chan struct{}), release: make(chan struct{})}
			done := make(chan error, 1)
			go func() {
				_, err := tr.Send(paused, Request{ID: RequestID{Value: "accepted"}, Method: "test"})
				done <- err
			}()
			var env writeEnvelope
			select {
			case env = <-tr.writeQueue:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case <-paused.paused:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			tr.processInboundLine([]byte(wire))
			env.done <- nil
			tr.mu.Lock()
			tr.closed = true
			tr.readerEOF = true
			tr.mu.Unlock()
			tr.cancelCtx()
			close(tr.readerStopped)
			close(paused.release)
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("write completion without response became success")
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if len(tr.pendingReqs) != 0 {
				t.Fatal("failed request retained")
			}
		})
	}
}

func TestPendingAbandonmentRejectsLaterResponse(t *testing.T) {
	tr := outcomeTransport()
	defer tr.cancelCtx()
	pending := pendingReq{id: RequestID{Value: "same"}, ch: make(chan pendingReqResult, 1)}
	tr.pendingReqs["s:same"] = pending
	_, err := tr.finishPendingRequest("s:same", pending, context.Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	tr.handleResponse(Response{ID: pending.id, Result: json.RawMessage(`true`)})
	if len(pending.ch) != 0 || len(tr.pendingReqs) != 0 {
		t.Fatal("abandoned response was published")
	}
}
