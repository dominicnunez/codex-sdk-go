package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func approvalPolicyFixture(t *testing.T) (*p.Client, *MockTransport) {
	t.Helper()
	mock := NewMockTransport()
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		if err := mock.SetResponseData(method, validThreadLifecycleResponse(validProcessThreadPayload("approval-thread"))); err != nil {
			t.Fatal(err)
		}
	}
	mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"turn","status":"inProgress","items":[]}}`)})
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func callApprovalPolicyMethod(client *p.Client, method string, value p.AskForApproval) (any, error) {
	switch method {
	case "thread/start":
		return client.Thread.Start(context.Background(), p.ThreadStartParams{ApprovalPolicy: &value})
	case "thread/resume":
		return client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "approval-thread", ApprovalPolicy: &value})
	case "thread/fork":
		return client.Thread.Fork(context.Background(), p.ThreadForkParams{ThreadID: "approval-thread", ApprovalPolicy: &value})
	default:
		return client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "approval-thread", Input: []p.UserInput{}, ApprovalPolicy: &value})
	}
}

func TestRemovedApprovalPolicyRejectedBeforeDispatch(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "turn/start"} {
		for _, pointer := range []bool{false, true} {
			t.Run(method+"/"+map[bool]string{false: "value", true: "pointer"}[pointer], func(t *testing.T) {
				client, mock := approvalPolicyFixture(t)
				literal := p.ApprovalPolicyOnFailure
				var policy p.AskForApproval = literal
				if pointer {
					policy = &literal
				}
				result, err := callApprovalPolicyMethod(client, method, policy)
				if err == nil || !reflect.ValueOf(result).IsZero() || len(mock.SentRequests) != 0 {
					t.Errorf("obsolete approval policy dispatched: err=%v requests=%d", err, len(mock.SentRequests))
				}
				if _, present := client.ThreadStateSnapshot("approval-thread"); present {
					t.Error("rejected request published response-derived cache state")
				}
			})
		}
	}
}

func TestRemovedApprovalPolicyLiteralAndWrapperEncoding(t *testing.T) {
	literal := p.ApprovalPolicyOnFailure
	for _, value := range []any{literal, &literal, p.AskForApprovalWrapper{Value: literal}, p.AskForApprovalWrapper{Value: &literal}} {
		if data, err := json.Marshal(value); err == nil || len(data) != 0 {
			t.Errorf("obsolete known literal serialized: %s err=%v", data, err)
		}
	}
}

func nilApprovalPointer[T any](_ T) *T { return nil }

func TestApprovalPolicyCurrentAndForwardCompatibleRequests(t *testing.T) {
	policies := []struct {
		name  string
		value p.AskForApproval
		wire  string
	}{
		{"untrusted", p.ApprovalPolicyUntrusted, `"untrusted"`},
		{"on-request", p.ApprovalPolicyOnRequest, `"on-request"`},
		{"never", p.ApprovalPolicyNever, `"never"`},
		{"granular-value", p.ApprovalPolicyGranular{}, `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false}}`},
		{"granular-pointer", &p.ApprovalPolicyGranular{}, `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false}}`},
		{"unknown-value", p.UnknownAskForApproval{Raw: json.RawMessage(`{"future":42}`)}, `{"future":42}`},
		{"unknown-pointer", &p.UnknownAskForApproval{Raw: json.RawMessage(`{"future":42}`)}, `{"future":42}`},
		{"nullable", nil, `null`},
		{"nil-literal-pointer", nilApprovalPointer(p.ApprovalPolicyNever), `null`},
		{"nil-granular-pointer", (*p.ApprovalPolicyGranular)(nil), `null`},
		{"nil-unknown-pointer", (*p.UnknownAskForApproval)(nil), `null`},
	}
	for _, raw := range []string{`""`, `"future"`, `"ON-FAILURE"`, `"on-failure "`} {
		var wrapper p.AskForApprovalWrapper
		if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
			t.Fatal(err)
		}
		policies = append(policies, struct {
			name  string
			value p.AskForApproval
			wire  string
		}{raw, wrapper.Value, raw})
	}
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "turn/start"} {
		for _, policy := range policies {
			t.Run(method+"/"+policy.name, func(t *testing.T) {
				client, mock := approvalPolicyFixture(t)
				if _, err := callApprovalPolicyMethod(client, method, policy.value); err != nil {
					t.Fatal(err)
				}
				if len(mock.SentRequests) != 1 {
					t.Fatal("valid request did not dispatch once")
				}
				var wire map[string]json.RawMessage
				if err := json.Unmarshal(mock.GetSentRequest(0).Params, &wire); err != nil {
					t.Fatal(err)
				}
				if string(wire["approvalPolicy"]) != policy.wire {
					t.Fatalf("compatible policy changed: %s want%s", wire["approvalPolicy"], policy.wire)
				}
			})
		}
	}
}

func TestRemovedApprovalPolicyDecodeAndRecovery(t *testing.T) {
	var wrapper p.AskForApprovalWrapper
	if err := json.Unmarshal([]byte(`"on-failure"`), &wrapper); err != nil || wrapper.Value != p.ApprovalPolicyOnFailure {
		t.Fatal("inbound literal compatibility narrowed")
	}
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "turn/start"} {
		client, mock := approvalPolicyFixture(t)
		if _, err := callApprovalPolicyMethod(client, method, wrapper.Value); err == nil || len(mock.SentRequests) != 0 {
			t.Fatal("decoded removed literal was dispatched")
		}
		if _, err := callApprovalPolicyMethod(client, method, p.ApprovalPolicyNever); err != nil || len(mock.SentRequests) != 1 {
			t.Fatal("same-client valid recovery failed")
		}
	}
	client, mock := approvalPolicyFixture(t)
	body := validThreadLifecycleResponse(validProcessThreadPayload("approval-thread"))
	body["approvalPolicy"] = "on-failure"
	if err := mock.SetResponseData("thread/start", body); err != nil {
		t.Fatal(err)
	}
	result, err := client.Thread.Start(context.Background(), p.ThreadStartParams{})
	if err != nil || result.ApprovalPolicy.Value != p.ApprovalPolicyOnFailure {
		t.Fatal("inbound response compatibility narrowed")
	}
	if fmt.Sprint(p.ApprovalPolicyOnFailure) != "on-failure" {
		t.Fatal("exported constant value changed")
	}
}

func FuzzApprovalPolicyLiteralEncoding(f *testing.F) {
	for _, seed := range []string{"on-failure", "untrusted", "on-request", "never", "future", "", "ON-FAILURE", "on-failure "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, literal string) {
		if len(literal) > 1024 {
			literal = literal[:1024]
		}
		literal = strings.ToValidUTF8(literal, "\uFFFD")
		raw, err := json.Marshal(literal)
		if err != nil {
			t.Fatal(err)
		}
		var wrapper p.AskForApprovalWrapper
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			t.Fatal("permissive inbound literal rejected")
		}
		encoded, err := json.Marshal(wrapper)
		if literal == "on-failure" {
			if err == nil || len(encoded) != 0 {
				t.Fatal("removed known literal encoded")
			}
			return
		}
		if err != nil || string(encoded) != string(raw) {
			t.Fatal("forward-compatible literal narrowed or changed")
		}
	})
}
