package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadStatusPointerEncoding(t *testing.T) {
	for _, tc := range []struct {
		value p.ThreadStatus
		wire  string
	}{
		{&p.ThreadStatusNotLoaded{}, `{"type":"notLoaded"}`},
		{&p.ThreadStatusIdle{}, `{"type":"idle"}`},
		{&p.ThreadStatusSystemError{}, `{"type":"systemError"}`},
		{&p.ThreadStatusActive{ActiveFlags: []p.ThreadActiveFlag{}}, `{"type":"active","activeFlags":[]}`},
		{&p.UnknownThreadStatus{Type: "future", Raw: json.RawMessage(`{"type":"future","payload":42}`)}, `{"type":"future","payload":42}`},
	} {
		t.Run(reflect.TypeOf(tc.value).Elem().Name(), func(t *testing.T) {
			assertUnionJSON(t, p.ThreadStatusWrapper{Value: tc.value}, tc.wire)
			assertUnionJSON(t, p.ThreadStatusWrapper{Value: reflect.ValueOf(tc.value).Elem().Interface().(p.ThreadStatus)}, tc.wire)
			var decoded p.ThreadStatusWrapper
			if err := json.Unmarshal([]byte(tc.wire), &decoded); err != nil {
				t.Fatal(err)
			}
			assertUnionJSON(t, decoded, tc.wire)
		})
	}
}

func TestSessionSourcePointerEncoding(t *testing.T) {
	literal := p.SessionSourceCLI
	for _, tc := range []struct {
		value p.SessionSource
		wire  string
	}{
		{&literal, `"cli"`},
		{&p.SessionSourceSubAgent{SubAgent: p.SubAgentSourceReview}, `{"subAgent":"review"}`},
		{&p.UnknownSessionSource{Raw: json.RawMessage(`{"future":{"payload":42}}`)}, `{"future":{"payload":42}}`},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			assertUnionJSON(t, p.SessionSourceWrapper{Value: tc.value}, tc.wire)
			assertUnionJSON(t, p.SessionSourceWrapper{Value: reflect.ValueOf(tc.value).Elem().Interface().(p.SessionSource)}, tc.wire)
		})
	}
}

func TestReasoningSummaryPointerRequest(t *testing.T) {
	mode := p.ReasoningSummaryModeConcise
	transport := NewMockTransport()
	transport.SetResponse("turn/start", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"turn":{"id":"u","status":"inProgress","items":[]}}`)})
	_, err := p.NewClient(transport).Turn.Start(context.Background(), p.TurnStartParams{
		ThreadID: "t", Input: []p.UserInput{&p.TextUserInput{Text: "hello"}}, Summary: &p.ReasoningSummaryWrapper{Value: &mode},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUnionJSON(t, transport.GetSentRequest(0).Params, `{"threadId":"t","input":[{"type":"text","text":"hello"}],"summary":"concise"}`)
}

func TestUnionPointerTypedNilEncoding(t *testing.T) {
	literal := p.SessionSourceCLI
	for _, value := range []any{
		p.ThreadStatusWrapper{Value: (*p.ThreadStatusNotLoaded)(nil)},
		p.ThreadStatusWrapper{Value: (*p.ThreadStatusIdle)(nil)},
		p.ThreadStatusWrapper{Value: (*p.ThreadStatusSystemError)(nil)},
		p.ThreadStatusWrapper{Value: (*p.ThreadStatusActive)(nil)},
		p.ThreadStatusWrapper{Value: (*p.UnknownThreadStatus)(nil)},
		p.SessionSourceWrapper{Value: reflect.Zero(reflect.TypeOf(&literal)).Interface().(p.SessionSource)},
		p.SessionSourceWrapper{Value: (*p.SessionSourceSubAgent)(nil)},
		p.SessionSourceWrapper{Value: (*p.UnknownSessionSource)(nil)},
		p.ReasoningSummaryWrapper{Value: (*p.ReasoningSummaryMode)(nil)},
	} {
		t.Run(reflect.TypeOf(value).Name(), func(t *testing.T) { assertUnionJSON(t, value, `null`) })
	}
}

type unsupportedPointerUnion struct {
	p.ThreadStatus
	p.SessionSource
	p.ReasoningSummary
}

func TestUnionPointerUnsupportedValues(t *testing.T) {
	for _, value := range []any{
		p.ThreadStatusWrapper{Value: unsupportedPointerUnion{}},
		p.ThreadStatusWrapper{Value: &unsupportedPointerUnion{}},
		p.ThreadStatusWrapper{Value: (*unsupportedPointerUnion)(nil)},
		p.SessionSourceWrapper{Value: unsupportedPointerUnion{}},
		p.SessionSourceWrapper{Value: &unsupportedPointerUnion{}},
		p.SessionSourceWrapper{Value: (*unsupportedPointerUnion)(nil)},
		p.ReasoningSummaryWrapper{Value: unsupportedPointerUnion{}},
		p.ReasoningSummaryWrapper{Value: &unsupportedPointerUnion{}},
		p.ReasoningSummaryWrapper{Value: (*unsupportedPointerUnion)(nil)},
	} {
		if _, err := json.Marshal(value); err == nil || !strings.Contains(err.Error(), "unknown ") {
			t.Errorf("unsupported %T admitted or error changed: %v", value, err)
		}
	}
}

func TestReasoningSummaryPointerValidation(t *testing.T) {
	mode := p.ReasoningSummaryMode("unsupported")
	for _, summary := range []p.ReasoningSummary{mode, &mode} {
		transport := NewMockTransport()
		_, err := p.NewClient(transport).Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{}, Summary: &p.ReasoningSummaryWrapper{Value: summary}})
		if err == nil || !strings.Contains(err.Error(), "reasoningSummary") {
			t.Errorf("invalid summary error = %v", err)
		}
		if len(transport.SentRequests) != 0 {
			t.Fatal("invalid enum reached transport")
		}
	}
}

func TestUnionPointersInThreadSnapshot(t *testing.T) {
	source := p.SessionSourceSubAgent{SubAgent: &p.SubAgentSourceOther{Other: "worker"}}
	thread := p.Thread{ID: "t", SessionID: "s", CLIVersion: "0", CreatedAt: 1, UpdatedAt: 1, Cwd: "/work", ModelProvider: "provider", Source: p.SessionSourceWrapper{Value: &source}, Status: p.ThreadStatusWrapper{Value: &p.ThreadStatusActive{ActiveFlags: []p.ThreadActiveFlag{}}}, Turns: []p.Turn{}}
	client := p.NewClient(NewMockTransport())
	client.CacheThreadState(thread)
	snapshot, ok := client.ThreadStateSnapshot("t")
	if !ok {
		t.Fatal("missing snapshot")
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		t.Fatal(err)
	}
	assertUnionJSON(t, members["source"], `{"subAgent":{"other":"worker"}}`)
	assertUnionJSON(t, members["status"], `{"type":"active","activeFlags":[]}`)
	var decoded p.Thread
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("thread round trip: %v", err)
	}
}
