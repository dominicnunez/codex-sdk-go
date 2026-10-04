package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type namedCitationEnvelope struct {
	Before   string                  `json:"before"`
	Citation protocol.MemoryCitation `json:"citation"`
	After    string                  `json:"after"`
}

type anonymousCitationEnvelope struct {
	Before string `json:"before"`
	protocol.MemoryCitation
	After string `json:"after"`
}

type anonymousEntryEnvelope struct {
	Before string `json:"before"`
	protocol.MemoryCitationEntry
	After string `json:"after"`
}

type anonymousAgentEnvelope struct {
	Before string `json:"before"`
	protocol.AgentMessageThreadItem
	After string `json:"after"`
}

func TestMemoryCitationPlainUnmarshalMethodSetsRemainNative(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(protocol.MemoryCitationEntry{}),
		reflect.TypeOf(protocol.MemoryCitation{}),
		reflect.TypeOf(protocol.AgentMessageThreadItem{}),
	} {
		if method, ok := typ.MethodByName("UnmarshalJSON"); ok {
			t.Errorf("%s unexpectedly defines UnmarshalJSON: %s", typ, method.Name)
		}
	}
}

func TestMemoryCitationPlainRepresentationsStayNative(t *testing.T) {
	const partial = `{"entries":[{}],"threadIds":[]}`
	var citation protocol.MemoryCitation
	if err := json.Unmarshal([]byte(partial), &citation); err != nil {
		t.Fatalf("plain citation acquired SDK admission behavior: %v", err)
	}
	if !reflect.DeepEqual(citation, protocol.MemoryCitation{
		Entries: []protocol.MemoryCitationEntry{{}}, ThreadIDs: []string{},
	}) {
		t.Fatalf("plain citation decoded unexpected value: %#v", citation)
	}
	var agent protocol.AgentMessageThreadItem
	if err := json.Unmarshal([]byte(`{"id":"m","text":"t","memoryCitation":`+partial+`}`), &agent); err != nil {
		t.Fatalf("plain AgentMessageThreadItem acquired union admission behavior: %v", err)
	}
	if agent.MemoryCitation == nil || !reflect.DeepEqual(*agent.MemoryCitation, citation) {
		t.Fatalf("plain agent citation = %#v, want %#v", agent.MemoryCitation, citation)
	}
	var named namedCitationEnvelope
	if err := json.Unmarshal([]byte(`{"before":"b","citation":`+partial+`,"after":"a"}`), &named); err != nil {
		t.Fatalf("named application envelope changed: %v", err)
	}
	if named.Before != "b" || named.After != "a" || !reflect.DeepEqual(named.Citation, citation) {
		t.Fatalf("named envelope = %#v", named)
	}
	var embedded anonymousCitationEnvelope
	if err := json.Unmarshal([]byte(`{"before":"b","entries":[{}],"threadIds":[],"after":"a"}`), &embedded); err != nil {
		t.Fatalf("anonymous plain citation envelope changed: %v", err)
	}
	if embedded.Before != "b" || embedded.After != "a" || !reflect.DeepEqual(embedded.MemoryCitation, citation) {
		t.Fatalf("anonymous envelope = %#v", embedded)
	}
	var embeddedEntry anonymousEntryEnvelope
	if err := json.Unmarshal([]byte(`{"before":"b","lineEnd":0,"lineStart":0,"note":"","path":"relative","after":"a"}`), &embeddedEntry); err != nil {
		t.Fatalf("anonymous entry envelope changed: %v", err)
	}
	if embeddedEntry.Before != "b" || embeddedEntry.After != "a" || embeddedEntry.Path != "relative" {
		t.Fatalf("anonymous entry envelope = %#v", embeddedEntry)
	}
	var embeddedAgent anonymousAgentEnvelope
	if err := json.Unmarshal([]byte(`{"before":"b","id":"m","text":"t","memoryCitation":`+partial+`,"after":"a"}`), &embeddedAgent); err != nil {
		t.Fatalf("anonymous agent envelope changed: %v", err)
	}
	if embeddedAgent.Before != "b" || embeddedAgent.After != "a" || embeddedAgent.MemoryCitation == nil || !reflect.DeepEqual(*embeddedAgent.MemoryCitation, citation) {
		t.Fatalf("anonymous agent envelope = %#v", embeddedAgent)
	}
}

func TestMemoryCitationPlainReceiverMatchesNativeDecoding(t *testing.T) {
	seed := `{"entries":[{"lineEnd":9,"lineStart":4,"note":"old","path":"old-path"}],"threadIds":["old-id"]}`
	var actual protocol.MemoryCitation
	var native nativeCitationReference
	for _, target := range []any{&actual, &native} {
		if err := json.Unmarshal([]byte(seed), target); err != nil {
			t.Fatal(err)
		}
	}
	partial := `{"entries":[{"note":"updated"}],"threadIds":["new-id"]}`
	for _, target := range []any{&actual, &native} {
		if err := json.Unmarshal([]byte(partial), target); err != nil {
			t.Fatal(err)
		}
	}
	want := protocol.MemoryCitation{Entries: []protocol.MemoryCitationEntry{{LineEnd: 9, LineStart: 4, Note: "updated", Path: "old-path"}}, ThreadIDs: []string{"new-id"}}
	if !reflect.DeepEqual(actual, want) || native.Entries[0].LineEnd != want.Entries[0].LineEnd || native.Entries[0].LineStart != want.Entries[0].LineStart || native.Entries[0].Note != want.Entries[0].Note || native.Entries[0].Path != want.Entries[0].Path || !reflect.DeepEqual(native.ThreadIDs, want.ThreadIDs) {
		t.Fatalf("partial receiver merge actual=%#v native=%#v want=%#v", actual, native, want)
	}

	typeErrorRaw := `{"entries":[{"lineEnd":"wrong","note":"after-error"}],"threadIds":[42]}`
	var gotTypeError, nativeTypeError error
	gotTypeError = json.Unmarshal([]byte(typeErrorRaw), &actual)
	nativeTypeError = json.Unmarshal([]byte(typeErrorRaw), &native)
	var gotUTE, nativeUTE *json.UnmarshalTypeError
	if !errors.As(gotTypeError, &gotUTE) || !errors.As(nativeTypeError, &nativeUTE) {
		t.Fatalf("type errors actual=%v native=%v", gotTypeError, nativeTypeError)
	}
	if gotUTE.Type != nativeUTE.Type || gotUTE.Value != nativeUTE.Value || gotUTE.Offset != nativeUTE.Offset || gotUTE.Field != nativeUTE.Field || gotUTE.Struct != "MemoryCitationEntry" {
		t.Fatalf("native diagnostic metadata changed: actual=%+v native=%+v", gotUTE, nativeUTE)
	}
	if actual.Entries[0].Note != "after-error" || native.Entries[0].Note != "after-error" || !reflect.DeepEqual(actual.ThreadIDs, want.ThreadIDs) || !reflect.DeepEqual(native.ThreadIDs, want.ThreadIDs) {
		t.Fatalf("type error changed receiver traversal: actual=%#v native=%#v", actual, native)
	}
}
