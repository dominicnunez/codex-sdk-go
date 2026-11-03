package transport_test

import (
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRPCErrorFieldOccurrences(t *testing.T) {
	for _, tc := range []struct {
		name, raw     string
		valid         bool
		code          int
		message, data string
	}{
		{"aliases", `{"CODE":0,"MESSAGE":""}`, true, 0, "", ""},
		{"escaped", `{"c\u006fde":17,"messa\u0067e":"fixture"}`, true, 17, "fixture", ""},
		{"unicode-alias", `{"CODE":17,"meſſage":"fixture"}`, true, 17, "fixture", ""},
		{"valid-duplicates", `{"code":1,"code":2,"message":"first","message":"last"}`, true, 2, "last", ""},
		{"data-selection", `{"code":1,"message":"fixture","data":false,"DATA":{"n":9007199254740993}}`, true, 1, "fixture", `{"n":9007199254740993}`},
		{"nullable-data", `{"code":1,"message":"fixture","data":{"old":true},"Data":null}`, true, 1, "fixture", `null`},
		{"open-record", `{"code":0,"message":"","future":{"n":1e10000,"x":null}}`, true, 0, "", ""},
		{"early-code-null", `{"code":null,"CODE":1,"message":"fixture"}`, false, 0, "", ""},
		{"late-code-null", `{"code":1,"CODE":null,"message":"fixture"}`, false, 0, "", ""},
		{"early-message-null", `{"code":1,"message":null,"MESSAGE":"fixture"}`, false, 0, "", ""},
		{"late-message-null", `{"code":1,"message":"fixture","meſſage":null}`, false, 0, "", ""},
		{"bad-code-hidden", `{"code":false,"code":1,"message":"fixture"}`, false, 0, "", ""},
		{"bad-message-hidden", `{"code":1,"message":42,"message":"fixture"}`, false, 0, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := newRPCErrorPeer(t)
			done := peer.start(t, int64(42))
			peer.send(t, int64(42), `"error":`+tc.raw)
			rpc := receiveRPCError(t, done)
			if tc.valid {
				if rpc.Code() != tc.code || rpc.Message() != tc.message || string(rpc.Data()) != tc.data || peer.transport.MalformedMessageCount() != 0 {
					t.Fatalf("valid native selection changed: code=%d message=%q data=%s", rpc.Code(), rpc.Message(), rpc.Data())
				}
			} else {
				if rpc.Code() != p.ErrCodeParseError || rpc.Message() != "failed to parse server response" || len(rpc.Data()) != 0 || peer.transport.MalformedMessageCount() != 1 {
					t.Fatal("invalid occurrence escaped malformed response path")
				}
				recovery := peer.start(t, "recovery")
				peer.send(t, "recovery", `"error":{"code":0,"message":""}`)
				valid := receiveRPCError(t, recovery)
				if valid.Code() != 0 || valid.Message() != "" || peer.transport.MalformedMessageCount() != 1 {
					t.Fatal("same-connection recovery failed")
				}
			}
		})
	}
}

func TestRPCErrorCorrelationAndEnvelopeSelection(t *testing.T) {
	peer := newRPCErrorPeer(t)
	first := peer.start(t, "malformed")
	second := peer.start(t, int64(7))
	peer.send(t, "malformed", `"error":{"message":"missing code","data":{"invalid":true}}`)
	if got := receiveRPCError(t, first); got.Code() != p.ErrCodeParseError || len(got.Data()) != 0 || peer.transport.MalformedMessageCount() != 1 {
		t.Fatal("malformed error correlation or accounting lost")
	}
	select {
	case got := <-second:
		t.Fatalf("unrelated pending request completed: %+v", got)
	default:
	}
	peer.send(t, int64(7), `"error":{"code":-32001,"message":"other","data":true}`)
	if got := receiveRPCError(t, second); got.Code() != -32001 || got.Message() != "other" || string(got.Data()) != "true" || peer.transport.MalformedMessageCount() != 1 {
		t.Fatal("unrelated request changed")
	}
	// The enclosing envelope keeps its established final-error selection. The
	// new required-member rule applies within that selected error record.
	for _, members := range []string{
		`"error":{},"ERROR":{"code":0,"message":""}`,
		`"error":null,"error":{"code":0,"message":""}`,
	} {
		done := peer.start(t, "selected")
		peer.send(t, "selected", members)
		if got := receiveRPCError(t, done); got.Code() != 0 || got.Message() != "" || peer.transport.MalformedMessageCount() != 1 {
			t.Fatal("outer envelope selection changed")
		}
	}
}

func TestRPCErrorPublicRecordContracts(t *testing.T) {
	for _, v := range []any{p.Error{}, &p.Error{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatal("public Error gained JSON decoder")
		}
		if _, ok := v.(json.Marshaler); ok {
			t.Fatal("public Error gained JSON encoder")
		}
	}
	type envelope struct {
		p.Error
		Extra string
	}
	value := envelope{Error: p.Error{Code: 7, Message: "prior", Data: json.RawMessage(`{"prior":true}`)}, Extra: "prior"}
	if err := json.Unmarshal([]byte(`{"CODE":null,"message":null,"Data":null,"Extra":"kept"}`), &value); err != nil {
		t.Fatal(err)
	}
	if value.Code != 7 || value.Message != "prior" || string(value.Data) != "null" || value.Extra != "kept" {
		t.Fatal("ordinary scalar null-no-clear or anonymous composition changed")
	}
	before := value
	if err := json.Unmarshal([]byte(`null`), &value); err != nil || !reflect.DeepEqual(value, before) {
		t.Fatal("ordinary root null behavior changed")
	}
	if err := json.Unmarshal([]byte(`{"code":0,"message":"first","MESSAGE":"last","extra":"kept"}`), &value); err != nil || value.Code != 0 || value.Message != "last" {
		t.Fatal("ordinary valid duplicate behavior changed")
	}
}
