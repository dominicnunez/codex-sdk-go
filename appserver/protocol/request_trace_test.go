package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestClientSendRequestTrace(t *testing.T) {
	parent, state := "opaque parent", "opaque state"
	request := p.Request{ID: p.RequestID{Value: "trace"}, Method: "fixture/trace", Params: json.RawMessage(`{}`), Trace: &p.W3cTraceContext{Traceparent: &parent, Tracestate: &state}}
	mock := NewMockTransport()
	mock.SetResponse(request.Method, p.Response{ID: request.ID, Result: json.RawMessage(`{}`)})
	client := p.NewClient(mock)
	defer client.Close()
	response, err := client.Send(context.Background(), request)
	if err != nil || !response.ID.Equal(request.ID) {
		t.Fatalf("Send: %+v %v", response, err)
	}
	sent := mock.GetSentRequest(0)
	if sent == nil || !reflect.DeepEqual(sent.Trace, request.Trace) {
		t.Fatal("client forwarding lost trace")
	}
}

func TestRequestTraceRecordComposition(t *testing.T) {
	var envelope struct {
		p.W3cTraceContext
		Extra string `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"traceparent":"","tracestate":null,"extra":"retained"}`), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Traceparent == nil || *envelope.Traceparent != "" || envelope.Tracestate != nil || envelope.Extra != "retained" {
		t.Fatal("trace record changed anonymous envelope or nullable fields")
	}
	var request p.Request
	if err := json.Unmarshal([]byte(`{"trace":{"traceparent":"parent"}}`), &request); err != nil {
		t.Fatal(err)
	}
	before := request.Trace
	if err := json.Unmarshal([]byte(`{}`), &request); err != nil || request.Trace != before {
		t.Fatal("ordinary omitted-field receiver semantics changed")
	}
	if err := json.Unmarshal([]byte(`{"trace":null}`), &request); err != nil || request.Trace != nil {
		t.Fatal("nullable trace did not reset receiver")
	}
}
