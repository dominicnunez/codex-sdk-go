package transport_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	tr "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type rpcErrorReply struct {
	response p.Response
	err      error
}
type rpcErrorRequest struct {
	request p.Request
	err     error
}
type rpcErrorPeer struct {
	client    *p.Client
	transport *tr.StdioTransport
	incoming  *io.PipeWriter
	requests  chan rpcErrorRequest
}

func newRPCErrorPeer(t testing.TB) *rpcErrorPeer {
	t.Helper()
	input, incoming := io.Pipe()
	outgoing, output := io.Pipe()
	transport := tr.NewStdioTransport(input, output)
	peer := &rpcErrorPeer{p.NewClient(transport), transport, incoming, make(chan rpcErrorRequest, 8)}
	t.Cleanup(func() { _ = incoming.Close(); _ = outgoing.Close(); _ = output.Close(); _ = peer.client.Close() })
	go func() {
		scanner := bufio.NewScanner(outgoing)
		for scanner.Scan() {
			var req p.Request
			err := json.Unmarshal(scanner.Bytes(), &req)
			peer.requests <- rpcErrorRequest{req, err}
			if err != nil {
				return
			}
		}
	}()
	return peer
}
func (peer *rpcErrorPeer) start(t testing.TB, id any) <-chan rpcErrorReply {
	t.Helper()
	done := make(chan rpcErrorReply, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	go func() {
		r, err := peer.client.Send(ctx, p.Request{ID: p.RequestID{Value: id}, Method: "fixture/error", Params: json.RawMessage(`{}`)})
		done <- rpcErrorReply{r, err}
	}()
	select {
	case request := <-peer.requests:
		if request.err != nil || !reflect.DeepEqual(request.request.ID, p.RequestID{Value: id}) {
			t.Fatalf("outbound request identity: %+v %v", request.request, request.err)
		}
	case <-ctx.Done():
		t.Fatal("outbound request not observed")
	}
	return done
}
func (peer *rpcErrorPeer) send(t testing.TB, id any, members string) {
	t.Helper()
	encoded, err := json.Marshal(p.RequestID{Value: id})
	if err != nil {
		t.Fatal(err)
	}
	fed := make(chan error, 1)
	go func() {
		_, err := io.WriteString(peer.incoming, `{"id":`+string(encoded)+`,`+members+`}`+"\n")
		fed <- err
	}()
	select {
	case err := <-fed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("response feed stuck")
	}
}
func receiveRPCError(t testing.TB, done <-chan rpcErrorReply) *p.RPCError {
	t.Helper()
	select {
	case got := <-done:
		var rpc *p.RPCError
		if !errors.As(got.err, &rpc) || !reflect.DeepEqual(got.response, p.Response{}) {
			t.Fatalf("expected RPCError and zero response, got %+v %v", got.response, got.err)
		}
		return rpc
	case <-time.After(3 * time.Second):
		t.Fatal("correlated response stuck")
		return nil
	}
}

func TestRPCErrorRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name, raw     string
		valid         bool
		code          int64
		message, data string
	}{
		{"empty", `{}`, false, 0, "", ""},
		{"missing-message", `{"code":-32001}`, false, 0, "", ""},
		{"missing-code", `{"message":"fixture"}`, false, 0, "", ""},
		{"null-code", `{"code":null,"message":"fixture"}`, false, 0, "", ""},
		{"null-message", `{"code":-32001,"message":null}`, false, 0, "", ""},
		{"both-null", `{"code":null,"message":null}`, false, 0, "", ""},
		{"zero-empty", `{"code":0,"message":""}`, true, 0, "", ""},
		{"data", `{"code":-32001,"message":"fixture","data":{"n":9007199254740993}}`, true, -32001, "fixture", `{"n":9007199254740993}`},
		{"boolean-code", `{"code":false,"message":"fixture"}`, false, 0, "", ""},
		{"numeric-message", `{"code":-32001,"message":42}`, false, 0, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := newRPCErrorPeer(t)
			done := peer.start(t, "wanted")
			peer.send(t, "wanted", `"error":`+tc.raw)
			rpc := receiveRPCError(t, done)
			if tc.valid {
				if rpc.Code() != tc.code || rpc.Message() != tc.message || string(rpc.Data()) != tc.data || peer.transport.MalformedMessageCount() != 0 {
					t.Fatalf("valid error changed: code=%d message=%q data=%s malformed=%d", rpc.Code(), rpc.Message(), rpc.Data(), peer.transport.MalformedMessageCount())
				}
			} else if rpc.Code() != p.ErrCodeParseError || rpc.Message() != "failed to parse server response" || len(rpc.Data()) != 0 || peer.transport.MalformedMessageCount() != 1 {
				t.Fatalf("malformed error published: code=%d message=%q data=%s malformed=%d", rpc.Code(), rpc.Message(), rpc.Data(), peer.transport.MalformedMessageCount())
			}
		})
	}
}
