package transport

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRequestTraceSelection(t *testing.T) {
	for _, members := range []string{
		``, `,"trace":null`, `,"trace":{}`,
		`,"trace":{"traceparent":null,"tracestate":null}`,
		`,"trace":{"traceparent":"","tracestate":"arbitrary opaque text"}`,
		`,"TRACE":{"TRACEPARENT":"parent"}`,
		`,"tr\u0061ce":{"tracep\u0061rent":"parent"}`,
		`,"trace":{"traceparent":"parent"},"trace":{"tracestate":"state"}`,
		`,"trace":{"traceparent":"parent"},"trace":null`,
		`,"trace":null,"trace":{"tracestate":"state"}`,
		`,"trace":{"traceparent":"parent","traceparent":null}`,
		`,"trace":{"extra":{"n":1e400},"traceparent":"parent"}`,
		`,"trace":[]`, `,"trace":42`, `,"trace":"wrong"`,
		`,"trace":{"traceparent":42}`, `,"trace":{"tracestate":false}`,
		`,"trace":[],"trace":{}`,
		`,"trace":{"traceparent":42,"traceparent":"parent"}`,
	} {
		t.Run(members, func(t *testing.T) {
			body := []byte(`{"id":"r","method":"fixture/trace","params":{"n":7}` + members + `}`)
			// Ordinary native decoding supplies an independent pointer/duplicate
			// reference, including nullable strings and case-folded aliases.
			var reference struct {
				Trace *struct {
					Traceparent *string `json:"traceparent,omitempty"`
					Tracestate  *string `json:"tracestate,omitempty"`
				} `json:"trace,omitempty"`
			}
			wantErr := json.Unmarshal(body, &reference)
			frame, err := decodeInboundFrame(body)
			var request Request
			publicErr := json.Unmarshal(body, &request)
			if (err != nil) != (wantErr != nil) || (publicErr != nil) != (wantErr != nil) {
				t.Fatalf("admission changed: frame=%v public=%v reference=%v", err, publicErr, wantErr)
			}
			if err != nil {
				return
			}
			for i := range body {
				body[i] = ' '
			}
			want, err := json.Marshal(reference.Trace)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(frame.Trace)
			if err != nil || string(got) != string(want) || !reflect.DeepEqual(frame.Trace, request.Trace) {
				t.Fatalf("trace selection or input ownership changed: %s / %s, %v", got, want, err)
			}
			if frame.Method != "fixture/trace" || frame.ID.value.Value != "r" || string(frame.Params) != `{"n":7}` {
				t.Fatal("existing envelope fields changed")
			}
			if frame.Trace != nil && frame.Trace.Traceparent != nil {
				*frame.Trace.Traceparent = "changed"
				if *request.Trace.Traceparent == "changed" {
					t.Fatal("independent decodes share trace storage")
				}
			}
		})
	}
}

func TestRequestTraceUnrelatedEnvelopes(t *testing.T) {
	for _, raw := range []string{
		`{"method":"notification","trace":[]}`,
		`{"id":"r","result":{},"trace":[]}`,
		`{"id":"r","error":{"code":1,"message":"m"},"trace":[]}`,
		`{"jsonrpc":"invalid","id":"r","method":"request","trace":[]}`,
		`{"id":[],"method":"request","trace":[]}`,
	} {
		frame, err := decodeInboundFrame([]byte(raw))
		if err != nil || frame.Trace != nil {
			t.Fatalf("ignored trace altered frame classification: %+v %v", frame, err)
		}
	}
}

func TestRequestTracePendingReplay(t *testing.T) {
	in, peer := io.Pipe()
	tr := NewStdioTransport(in, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = peer.Close() })
	if _, err := io.WriteString(peer, `{"id":"r","method":"fixture/trace","trace":{"traceparent":"parent","tracestate":"state"}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	awaitPayloadCondition(t, func() bool {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		return len(tr.pendingReqHandler) == 1
	})
	seen := make(chan Request, 1)
	tr.OnRequest(func(_ context.Context, request Request) (Response, error) {
		seen <- request
		return Response{Result: json.RawMessage(`{}`)}, nil
	})
	select {
	case request := <-seen:
		if request.Trace == nil || request.Trace.Traceparent == nil || request.Trace.Tracestate == nil ||
			*request.Trace.Traceparent != "parent" || *request.Trace.Tracestate != "state" {
			t.Fatal("pending request replay lost trace context")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending request replay stalled")
	}
}

func TestRequestTraceRejectionAndRecovery(t *testing.T) {
	in, peer := io.Pipe()
	out, writer := io.Pipe()
	tr := NewStdioTransport(in, writer)
	t.Cleanup(func() { _ = tr.Close(); _ = peer.Close(); _ = out.Close(); _ = writer.Close() })
	calls := make(chan Request, 1)
	tr.OnRequest(func(_ context.Context, r Request) (Response, error) {
		calls <- r
		return Response{Result: json.RawMessage(`{}`)}, nil
	})
	decoder := json.NewDecoder(out)
	for _, members := range []string{
		`"trace":[]`,
		`"trace":{"traceparent":42},"trace":{"traceparent":"later"}`,
		`"trace":{"tracestate":` + strings.Repeat("9", 1<<16) + `}`,
		`"trace":{"traceparent":"opaque","tracestate":"state"}`,
	} {
		if _, err := io.WriteString(peer, `{"id":"r","method":"fixture/trace",`+members+"}\n"); err != nil {
			t.Fatal(err)
		}
		type result struct {
			response Response
			err      error
		}
		done := make(chan result, 1)
		go func() { var response Response; err := decoder.Decode(&response); done <- result{response, err} }()
		select {
		case got := <-done:
			if got.err != nil || got.response.ID.Value != "r" {
				t.Fatalf("reply: %+v", got)
			}
			if strings.Contains(members, "opaque") {
				if got.response.Error != nil {
					t.Fatalf("recovery rejected: %+v", got.response)
				}
				select {
				case request := <-calls:
					if request.Trace == nil || *request.Trace.Traceparent != "opaque" || *request.Trace.Tracestate != "state" {
						t.Fatal("recovery lost trace")
					}
				default:
					t.Fatal("recovery callback missing")
				}
			} else {
				if got.response.Error == nil || got.response.Error.Code != ErrCodeInvalidRequest || len(got.response.Error.Message) > 4096 {
					t.Fatalf("wrong rejection: %+v", got.response)
				}
				select {
				case <-calls:
					t.Fatal("invalid trace published to callback")
				default:
				}
			}
		case <-time.After(3 * time.Second):
			t.Fatal("trace admission reply stalled")
		}
	}
}
