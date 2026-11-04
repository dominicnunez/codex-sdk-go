package transport_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strconv"
	"testing"
	"time"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	tr "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

var schemaErrorCodes = []string{"-9223372036854775808", "-2147483649", "-2147483648", "-32700", "0", "2147483647", "2147483648", "9223372036854775807"}

func TestErrorCodeWidth(t *testing.T) {
	t.Logf("architecture=%s int_bits=%d", runtime.GOARCH, strconv.IntSize)
	for _, code := range schemaErrorCodes {
		t.Run(code, func(t *testing.T) {
			var value p.Error
			if e := json.Unmarshal([]byte(`{"code":`+code+`,"message":"fixture","data":{"n":7}}`), &value); e != nil {
				t.Fatalf("schema-valid int64 code rejected: %v", e)
			}
			if strconv.FormatInt(value.Code, 10) != code || value.Message != "fixture" || string(value.Data) != `{"n":7}` {
				t.Fatal("error fields changed")
			}
		})
	}
}

func TestErrorCodeWire(t *testing.T) {
	t.Logf("architecture=%s int_bits=%d", runtime.GOARCH, strconv.IntSize)
	for _, code := range schemaErrorCodes {
		t.Run(code, func(t *testing.T) {
			reader, peer := io.Pipe()
			output, writer := io.Pipe()
			transport := tr.NewStdioTransport(reader, writer)
			client := p.NewClient(transport)
			defer func() { _ = client.Close(); _ = peer.Close(); _ = output.Close(); _ = writer.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			fed := make(chan error, 1)
			go func() {
				_, e := bufio.NewReader(output).ReadBytes('\n')
				if e == nil {
					_, e = io.WriteString(peer, `{"id":"wanted","error":{"code":`+code+`,"message":"fixture","data":{"n":7}}}`+"\n")
				}
				fed <- e
			}()
			_, e := client.Send(ctx, p.Request{ID: p.RequestID{Value: "wanted"}, Method: "fixture/code"})
			var rpc *p.RPCError
			if !errors.As(e, &rpc) {
				t.Fatalf("missing RPCError: %v", e)
			}
			if strconv.FormatInt(rpc.Code(), 10) != code || rpc.Message() != "fixture" || string(rpc.Data()) != `{"n":7}` {
				t.Fatalf("schema-valid server error changed: code=%d message=%s data=%s want_code=%s", rpc.Code(), rpc.Message(), string(rpc.Data()), code)
			}
			select {
			case e := <-fed:
				if e != nil {
					t.Fatal(e)
				}
			case <-ctx.Done():
				t.Fatal("peer stuck")
			}
		})
	}
}

func TestErrorCodeReply(t *testing.T) {
	t.Logf("architecture=%s int_bits=%d", runtime.GOARCH, strconv.IntSize)
	for _, token := range schemaErrorCodes {
		t.Run(token, func(t *testing.T) {
			code, err := strconv.ParseInt(token, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			reader, peer := io.Pipe()
			output, writer := io.Pipe()
			transport := tr.NewStdioTransport(reader, writer)
			t.Cleanup(func() { _ = transport.Close(); _ = peer.Close(); _ = output.Close(); _ = writer.Close() })
			transport.OnRequest(func(context.Context, p.Request) (p.Response, error) {
				return p.Response{Error: &p.Error{Code: code, Message: "fixture", Data: json.RawMessage(`{"n":7}`)}}, nil
			})
			if _, err := io.WriteString(peer, `{"id":"wanted","method":"fixture/reply"}`+"\n"); err != nil {
				t.Fatal(err)
			}
			type observation struct {
				response p.Response
				err      error
			}
			seen := make(chan observation, 1)
			go func() {
				var response p.Response
				err := json.NewDecoder(output).Decode(&response)
				seen <- observation{response, err}
			}()
			select {
			case got := <-seen:
				if got.err != nil || got.response.ID.Value != "wanted" || got.response.Error == nil ||
					got.response.Error.Code != code || got.response.Error.Message != "fixture" || string(got.response.Error.Data) != `{"n":7}` {
					t.Fatalf("outgoing callback error changed: %+v %v", got.response, got.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("callback error reply stalled")
			}
		})
	}
}
