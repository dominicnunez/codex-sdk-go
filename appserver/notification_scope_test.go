package appserver

import (
	"encoding/json"
	"testing"
)

func TestMalformedCompletionFallbackUsesExactOwner(t *testing.T) {
	item := json.RawMessage(`{"threadId":"a","ThreadID":"b","turnId":"turn-a","TurnID":"turn-b","item":{"type":"agentMessage","id":99,"text":"bad"},"Item":{"type":"agentMessage","id":"foreign","text":"foreign"}}`)
	if _, ok, _ := parseItemCompletedForThread(item, "b"); ok {
		t.Fatal("extra metadata attributed malformed item to another thread")
	}
	n, ok, err := parseItemCompletedForThread(item, "a")
	if !ok || err == nil || n.ThreadID != "a" || n.TurnID != "turn-a" {
		t.Fatalf("malformed item attribution = %#v, %v, %v", n, ok, err)
	}
	unknown, ok := n.Item.Value.(*UnknownThreadItem)
	if !ok || string(unknown.Raw) != `{"type":"agentMessage","id":99,"text":"bad"}` {
		t.Fatalf("fallback item = %#v", n.Item.Value)
	}
	turn := json.RawMessage(`{"threadId":"a","ThreadID":"b","turn":{"id":"turn-a","ID":"turn-b","status":"bad","items":[]},"Turn":{"id":"foreign","status":"completed","items":[]}}`)
	if _, ok, _ := parseTurnCompletedForThread(turn, "b", false); ok {
		t.Fatal("extra metadata attributed malformed completion to another thread")
	}
	candidate, ok, err := parseTurnCompletedForThread(turn, "a", false)
	if !ok || err == nil || candidate.notification.ThreadID != "a" || candidate.turnID != "turn-a" {
		t.Fatalf("malformed turn attribution = %#v, %v, %v", candidate, ok, err)
	}
}

func TestMalformedLifecycleAttributionIgnoresAliasTypes(t *testing.T) {
	for _, params := range []string{
		`{"threadId":"a","ThreadID":99,"turnId":"turn-a","TurnID":[]}`,
		`{"threadId":"a","ThreadID":null,"turnId":"turn-a"}`,
	} {
		carrier, ok := unmarshalThreadIDCarrier(json.RawMessage(params))
		if !ok || carrier.ThreadID != "a" || carrier.TurnID != "turn-a" {
			t.Fatalf("fallback identity = %#v, %v", carrier, ok)
		}
	}
	for _, params := range []string{`{"ThreadID":"b"}`, `{"threadId":99}`, `{"threadId":null}`, `[]`} {
		carrier, ok := unmarshalThreadIDCarrier(json.RawMessage(params))
		if ok && carrier.ThreadID != "" {
			t.Fatalf("unattributable payload selected %#v", carrier)
		}
	}
}
