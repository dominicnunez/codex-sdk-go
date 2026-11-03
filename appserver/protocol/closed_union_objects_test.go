package protocol_test

import (
	"context"
	"encoding/json"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"testing"
)

func closedSourceRead(t *testing.T, raw string) error {
	t.Helper()
	thread := validThreadPayload("thread")
	thread["source"] = json.RawMessage(raw)
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	_, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "thread"})
	return err
}
func closedApprovalRead(t *testing.T, raw string) error {
	t.Helper()
	response := validThreadLifecycleResponse(validThreadPayload("thread"))
	response["approvalPolicy"] = json.RawMessage(raw)
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	if err := mock.SetResponseData("thread/start", response); err != nil {
		t.Fatal(err)
	}
	_, err := client.Thread.Start(context.Background(), p.ThreadStartParams{})
	return err
}

func TestClosedUnionValidAlternatives(t *testing.T) {
	for _, raw := range []string{`{"custom":""}`, `{"subAgent":"review"}`, `{"subAgent":{"other":""}}`, `{"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":"","extra":{"unconstrained":true}}}}`, `{"future":{"extra":true}}`} {
		var source p.SessionSourceWrapper
		if err := json.Unmarshal([]byte(raw), &source); err != nil {
			t.Fatalf("valid source: %s %v", raw, err)
		}
		if err := closedSourceRead(t, raw); err != nil {
			t.Fatalf("valid source public: %s %v", raw, err)
		}
	}
	raw := `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false,"extra":{"allowed":true}}}`
	var policy p.AskForApprovalWrapper
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatal(err)
	}
	if err := closedApprovalRead(t, raw); err != nil {
		t.Fatal(err)
	}
}

func TestClosedUnionEscapedFieldsAndDuplicates(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"custom":"first","cu\u0073tom":"last"}`, `{"custom":"last"}`},
		{`{"subAgent":"review","sub\u0041gent":{"other":"last"}}`, `{"subAgent":{"other":"last"}}`},
		{`{"subAgent":{"other":"first","oth\u0065r":"last"}}`, `{"subAgent":{"other":"last"}}`},
	} {
		var source p.SessionSourceWrapper
		if err := json.Unmarshal([]byte(tc.raw), &source); err != nil {
			t.Fatal(err)
		}
		assertUnionJSON(t, source, tc.want)
	}
	for _, raw := range []string{`{"custom":"name","sub\u0041gent":"review"}`, `{"subAgent":{"other":"name","thread_\u0073pawn":{"depth":0,"parent_thread_id":"p"}}}`, `{"custom":"name","ex\u0074ra":true}`} {
		var source p.SessionSourceWrapper
		if err := json.Unmarshal([]byte(raw), &source); err == nil {
			t.Fatal("escaped forbidden property bypassed closed shape")
		}
	}
	var policy p.AskForApprovalWrapper
	raw := `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"gr\u0061nular":{"mcp_elicitations":true,"rules":true,"sandbox_approval":true}}`
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatal(err)
	}
	assertUnionJSON(t, policy, `{"granular":{"mcp_elicitations":true,"rules":true,"sandbox_approval":true}}`)
}

func TestClosedSourceObjectsRejectExtras(t *testing.T) {
	for _, raw := range []string{
		`{"custom":"name","extra":true}`,
		`{"subAgent":"review","extra":true}`,
		`{"subAgent":{"other":"name","extra":true}}`,
		`{"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":"p"},"extra":true}}`,
		`{"custom":"name","subAgent":"review"}`,
		`{"subAgent":{"other":"name","thread_spawn":{"depth":0,"parent_thread_id":"p"}}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var source p.SessionSourceWrapper
			if err := json.Unmarshal([]byte(raw), &source); err == nil {
				t.Errorf("closed source object admitted: %s", raw)
			}
			if err := closedSourceRead(t, raw); err == nil {
				t.Errorf("closed source object published by Thread.Read: %s", raw)
			}
		})
	}
}

func TestClosedGranularObjectRejectsExtras(t *testing.T) {
	raw := `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"extra":true}`
	var policy p.AskForApprovalWrapper
	if err := json.Unmarshal([]byte(raw), &policy); err == nil {
		t.Error("closed granular wrapper admitted extra key")
	}
	if err := closedApprovalRead(t, raw); err == nil {
		t.Error("closed granular wrapper published by Thread.Start")
	}
}
