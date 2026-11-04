package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestRequestTraceContext(t *testing.T) {
	for _, trace := range []string{"", "null", `{}`, `{"traceparent":null,"tracestate":null}`, `{"traceparent":"","tracestate":""}`, `{"tracestate":"vendor=fixture"}`, `{"traceparent":"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}`, `{"traceparent":"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01","tracestate":"vendor=fixture"}`} {
		name := "absent"
		if trace != "" {
			name = trace
		}
		for _, route := range []string{"codec", "inbound", "outbound"} {
			t.Run(route+"/"+name, func(t *testing.T) {
				body := `{"id":"wanted","method":"fixture/trace","params":{"n":7}`
				if trace != "" {
					body += `,"trace":` + trace
				}
				body += `}`
				var request Request
				if e := json.Unmarshal([]byte(body), &request); e != nil {
					t.Fatal(e)
				}
				check := func(raw []byte) {
					t.Helper()
					var fields map[string]json.RawMessage
					if e := json.Unmarshal(raw, &fields); e != nil {
						t.Fatal(e)
					}
					if string(fields["id"]) != `"wanted"` || string(fields["method"]) != `"fixture/trace"` || string(fields["params"]) != `{"n":7}` {
						t.Fatal("existing request fields changed")
					}
					want := trace
					if trace == "null" {
						want = ""
					}
					if trace == `{"traceparent":null,"tracestate":null}` {
						want = `{}`
					}
					if string(fields["trace"]) != want {
						_, present := fields["trace"]
						t.Fatalf("trace context lost: present=%v raw=%q want=%s", present, string(fields["trace"]), want)
					}
				}
				if route == "codec" {
					raw, e := json.Marshal(request)
					if e != nil {
						t.Fatal(e)
					}
					check(raw)
					return
				}
				reader, peer := io.Pipe()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if route == "inbound" {
					tr := NewStdioTransport(reader, io.Discard)
					defer func() { _ = tr.Close(); _ = peer.Close() }()
					seen := make(chan Request, 1)
					tr.OnRequest(func(_ context.Context, r Request) (Response, error) {
						seen <- r
						return Response{Result: json.RawMessage(`{}`)}, nil
					})
					if _, e := io.WriteString(peer, body+"\n"); e != nil {
						t.Fatal(e)
					}
					select {
					case r := <-seen:
						raw, e := json.Marshal(r)
						if e != nil {
							t.Fatal(e)
						}
						check(raw)
					case <-ctx.Done():
						t.Fatal("request callback missing")
					}
					return
				}
				output, writer := io.Pipe()
				tr := NewStdioTransport(reader, writer)
				defer func() { _ = tr.Close(); _ = peer.Close(); _ = output.Close(); _ = writer.Close() }()
				type observation struct {
					wire []byte
					e    error
				}
				seen := make(chan observation, 1)
				go func() {
					line, e := bufio.NewReader(output).ReadBytes('\n')
					if e == nil {
						_, e = io.WriteString(peer, `{"id":"wanted","result":{},"trace":[]}`+"\n")
					}
					seen <- observation{line, e}
				}()
				r, e := tr.Send(ctx, request)
				if e != nil || r.ID.Value != "wanted" || string(r.Result) != `{}` {
					t.Fatalf("request did not complete: %v", e)
				}
				select {
				case got := <-seen:
					if got.e != nil {
						t.Fatal(got.e)
					}
					check(got.wire)
				case <-ctx.Done():
					t.Fatal("wire observation missing")
				}
			})
		}
	}
}
