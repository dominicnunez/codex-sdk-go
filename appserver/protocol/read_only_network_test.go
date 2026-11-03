package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func assertReadOnlyNetworkWire(t *testing.T, value any, present, enabled bool) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	raw, exists := fields["networkAccess"]
	var actual bool
	if exists {
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
	}
	if exists != present || actual != enabled {
		t.Errorf("read-only network flag lost: wire=%s want present=%v enabled=%v", data, present, enabled)
	}
}

func TestReadOnlyNetworkAdmissionAndRoundTrip(t *testing.T) {
	for _, member := range []string{"", `,"networkAccess":false`, `,"networkAccess":true`} {
		var policy p.SandboxPolicyWrapper
		if err := json.Unmarshal([]byte(`{"type":"readOnly"`+member+`}`), &policy); err != nil {
			t.Fatal(err)
		}
		assertReadOnlyNetworkWire(t, policy, member != "", member == `,"networkAccess":true`)
	}
	for _, member := range []string{`null`, `null,"networkAccess":true`, `42`, `"true"`, `{}`, `[]`} {
		var policy p.SandboxPolicyWrapper
		if err := json.Unmarshal([]byte(`{"type":"readOnly","networkAccess":`+member+`}`), &policy); err == nil {
			t.Errorf("invalid recognized network flag accepted: %s", member)
		}
	}
}

func TestReadOnlyNetworkPublicResponseCarriers(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			body := validThreadLifecycleResponse(validProcessThreadPayload("network-thread"))
			body["sandbox"] = json.RawMessage(`{"type":"readOnly","networkAccess":true}`)
			if err := mock.SetResponseData(method, body); err != nil {
				t.Fatal(err)
			}
			var policy p.SandboxPolicyWrapper
			switch method {
			case "thread/start":
				response, err := client.Thread.Start(context.Background(), p.ThreadStartParams{})
				if err != nil {
					t.Fatal(err)
				}
				policy = response.Sandbox
			case "thread/resume":
				response, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "network-thread"})
				if err != nil {
					t.Fatal(err)
				}
				policy = response.Sandbox
			case "thread/fork":
				response, err := client.Thread.Fork(context.Background(), p.ThreadForkParams{ThreadID: "network-thread"})
				if err != nil {
					t.Fatal(err)
				}
				policy = response.Sandbox
			}
			assertReadOnlyNetworkWire(t, policy, true, true)
		})
	}
}

func TestReadOnlyNetworkSettingsCallbacks(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	calls := 0
	callback := func(notification p.ThreadSettingsUpdatedNotification) {
		calls++
		assertReadOnlyNetworkWire(t, notification.ThreadSettings.SandboxPolicy, true, true)
	}
	client.OnThreadSettingsUpdated(func(notification p.ThreadSettingsUpdatedNotification) {
		callback(notification)
		policy := notification.ThreadSettings.SandboxPolicy.Value.(p.SandboxPolicyReadOnly)
		*policy.NetworkAccess = false
	})
	remove := client.AddThreadSettingsUpdatedListener(callback)
	defer remove()
	params := `{"threadId":"network-thread","threadSettings":{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandboxPolicy":{"type":"readOnly","networkAccess":true},"collaborationMode":{"mode":"default","settings":{"model":"model"}}}}`
	mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: json.RawMessage(params)})
	if calls != 2 {
		t.Fatalf("callback count=%d want2", calls)
	}
}

func TestReadOnlyNetworkDecodedOutboundRequests(t *testing.T) {
	for _, method := range []string{"turn/start", "command/exec"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			if err := mock.SetResponseData("turn/start", map[string]any{"turn": map[string]any{"id": "turn", "status": "inProgress", "items": []any{}}}); err != nil {
				t.Fatal(err)
			}
			if err := mock.SetResponseData("command/exec", map[string]any{"exitCode": 0, "stdout": "", "stderr": ""}); err != nil {
				t.Fatal(err)
			}
			switch method {
			case "turn/start":
				var params p.TurnStartParams
				if err := json.Unmarshal([]byte(`{"threadId":"network-thread","input":[],"sandboxPolicy":{"type":"readOnly","networkAccess":true}}`), &params); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Turn.Start(context.Background(), params); err != nil {
					t.Fatal(err)
				}
			case "command/exec":
				var params p.CommandExecParams
				if err := json.Unmarshal([]byte(`{"command":["synthetic"],"sandboxPolicy":{"type":"readOnly","networkAccess":true}}`), &params); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Command.Exec(context.Background(), params); err != nil {
					t.Fatal(err)
				}
			}
			requests := mock.SentRequests
			if len(requests) != 1 {
				t.Fatalf("requests=%d", len(requests))
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(requests[0].Params, &fields); err != nil {
				t.Fatal(err)
			}
			var policy map[string]any
			if err := json.Unmarshal(fields["sandboxPolicy"], &policy); err != nil {
				t.Fatal(err)
			}
			if policy["networkAccess"] != true {
				t.Errorf("sent request lost network flag: %s", requests[0].Params)
			}
		})
	}
}

func TestReadOnlyNetworkConstructedRequests(t *testing.T) {
	for _, method := range []string{"turn/start", "command/exec"} {
		for _, pointer := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/pointer=%v/enabled=%v", method, pointer, enabled), func(t *testing.T) {
					mock := NewMockTransport()
					client := p.NewClient(mock)
					defer client.Close()
					mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"turn","items":[],"status":"inProgress"}}`)})
					mock.SetResponse("command/exec", p.Response{Result: json.RawMessage(`{"exitCode":0,"stdout":"","stderr":""}`)})
					policy := p.SandboxPolicyReadOnly{NetworkAccess: &enabled, Access: &p.ReadOnlyAccessWrapper{Value: p.ReadOnlyAccessRestricted{ReadableRoots: []string{"/tmp/../workspace"}}}}
					var value p.SandboxPolicy = policy
					if pointer {
						value = &policy
					}
					if method == "turn/start" {
						if _, err := client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "thread", Input: []p.UserInput{}, SandboxPolicy: &value}); err != nil {
							t.Fatal(err)
						}
					} else {
						if _, err := client.Command.Exec(context.Background(), p.CommandExecParams{Command: []string{"synthetic"}, SandboxPolicy: &p.SandboxPolicyWrapper{Value: value}}); err != nil {
							t.Fatal(err)
						}
					}
					var wire struct {
						Policy p.SandboxPolicyWrapper `json:"sandboxPolicy"`
					}
					if err := json.Unmarshal(mock.GetSentRequest(0).Params, &wire); err != nil {
						t.Fatal(err)
					}
					assertReadOnlyNetworkWire(t, wire.Policy, true, enabled)
					retained := wire.Policy.Value.(p.SandboxPolicyReadOnly)
					if retained.Access == nil || retained.Access.Value.(p.ReadOnlyAccessRestricted).ReadableRoots[0] != "/workspace" || policy.Access.Value.(p.ReadOnlyAccessRestricted).ReadableRoots[0] != "/tmp/../workspace" {
						t.Fatal("legacy access normalization or caller ownership changed")
					}
				})
			}
		}
	}
}

func FuzzReadOnlyNetworkOccurrences(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {1}, {2, 0}, {3, 1}, {0, 1}, {0, 2}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, choices []byte) {
		if len(choices) > 8 {
			choices = choices[:8]
		}
		values := []string{`false`, `true`, `null`, `42`, `"true"`, `[]`, `{}`}
		keys := []string{`"networkAccess"`, `"NETWORKACCESS"`, `"network\u0041ccess"`, `"networ\u212aAccess"`}
		members := make([]string, 0, len(choices))
		valid, enabled := true, false
		for i, choice := range choices {
			index := int(choice) % len(values)
			members = append(members, keys[i%len(keys)]+":"+values[index])
			valid = valid && index < 2
			enabled = index == 1
		}
		raw := `{"type":"readOnly"`
		if len(members) > 0 {
			raw += "," + strings.Join(members, ",")
		}
		raw += "}"
		var wrapper p.SandboxPolicyWrapper
		err := json.Unmarshal([]byte(raw), &wrapper)
		if !valid {
			if err == nil {
				t.Fatal("invalid earlier occurrence hidden by later field")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		assertReadOnlyNetworkWire(t, wrapper, len(choices) > 0, enabled)
	})
}
