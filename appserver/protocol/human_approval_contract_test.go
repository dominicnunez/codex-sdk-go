package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestHumanApprovalHandshakeIdentity(t *testing.T) {
	for _, experimental := range []bool{false, true} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		if err := mock.SetResponseData("initialize", validInitializeResponseData("fixture")); err != nil {
			t.Fatal(err)
		}
		var params p.InitializeParams
		body := `{"clientInfo":{"name":"fixture","version":"1"},"capabilities":{"requiredHumanApproval":true,"experimentalApi":false}}`
		if experimental {
			body = `{"clientInfo":{"name":"fixture","version":"1"},"capabilities":{"requiredHumanApproval":true,"experimentalApi":true}}`
		}
		if err := json.Unmarshal([]byte(body), &params); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Initialize(context.Background(), params); err != nil {
			t.Fatal(err)
		}
		var sent struct {
			Capabilities map[string]json.RawMessage `json:"capabilities"`
		}
		if err := json.Unmarshal(mock.GetSentRequest(0).Params, &sent); err != nil {
			t.Fatal(err)
		}
		if string(sent.Capabilities["requiredHumanApproval"]) != "true" {
			t.Error("human approval opt-in disappeared")
		}
		var changed p.InitializeParams
		if err := json.Unmarshal([]byte(`{"clientInfo":{"name":"fixture","version":"1"},"capabilities":{"requiredHumanApproval":false,"experimentalApi":`+map[bool]string{false: "false", true: "true"}[experimental]+`}}`), &changed); err != nil {
			t.Fatal(err)
		}
		_, err := client.Initialize(context.Background(), changed)
		var mismatch *p.InitializeParamsMismatchError
		if !errors.As(err, &mismatch) {
			t.Errorf("different human approval capability reused handshake: %v", err)
		}
		if mock.CallCount() != 1 {
			t.Errorf("mismatch dispatched another initialize: %d", mock.CallCount())
		}
	}
}

func TestHumanCommandOneTimeDecisions(t *testing.T) {
	for _, kind := range []string{"command", "writeStdin", "humanCommand"} {
		for _, decision := range []any{"accept", p.Ptr("decline"), "cancel", "acceptForSession", p.AcceptWithExecpolicyAmendmentDecision{ExecpolicyAmendment: []string{}}} {
			mock := NewMockTransport()
			calls := 0
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			client.SetApprovalHandlers(p.ApprovalHandlers{OnCommandExecutionRequestApproval: func(_ context.Context, params p.CommandExecutionRequestApprovalParams) (p.CommandExecutionRequestApprovalResponse, error) {
				calls++
				if string(params.Kind) != kind {
					t.Errorf("lost kind %q", params.Kind)
				}
				return p.CommandExecutionRequestApprovalResponse{Decision: p.CommandExecutionApprovalDecisionWrapper{Value: decision}}, nil
			}})
			resp, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "approval"}, Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"kind":"` + kind + `","itemId":"i","threadId":"t","turnId":"u","startedAtMs":0}`)})
			allowed := kind != "humanCommand" || decision == "accept" || decision == "cancel"
			if _, ok := decision.(*string); ok {
				allowed = true
			}
			if calls != 1 {
				t.Errorf("%s did not reach human handler: %v", kind, err)
			}
			if allowed && (err != nil || len(resp.Result) == 0) || !allowed && (err == nil || len(resp.Result) != 0) {
				t.Errorf("%s/%T:%v response=%s err=%v", kind, decision, decision, resp.Result, err)
			}
		}
	}
}
