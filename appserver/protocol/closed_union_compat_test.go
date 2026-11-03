package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestClosedUnionReceiverPreservation(t *testing.T) {
	prior := json.RawMessage(`{"future":{"prior":true}}`)
	for _, raw := range []string{`{"custom":"name","extra":true}`, `{"subAgent":"review","extra":true}`, `{"subAgent":{"other":"name","extra":true}}`, `{"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":"p"},"extra":true}}`, `{"custom":"name","subAgent":"review"}`} {
		value := p.SessionSourceWrapper{Value: p.UnknownSessionSource{Raw: prior}}
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatal("closed source accepted")
		}
		retained, ok := value.Value.(p.UnknownSessionSource)
		if !ok || &retained.Raw[0] != &prior[0] || string(retained.Raw) != string(prior) {
			t.Fatal("new rejection replaced prior union references")
		}
	}
	policy := p.ApprovalPolicyGranular{}
	approval := p.AskForApprovalWrapper{Value: p.UnknownAskForApproval{Raw: prior}}
	if err := json.Unmarshal([]byte(`{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"extra":true}`), &approval); err == nil {
		t.Fatal("closed approval object accepted")
	}
	retained := approval.Value.(p.UnknownAskForApproval)
	if &retained.Raw[0] != &prior[0] {
		t.Fatal("new approval rejection replaced prior union references")
	}
	policy.Granular.RequestPermissions = p.Ptr(true)
	before := policy
	if err := json.Unmarshal([]byte(`{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"extra":true}`), &policy); err == nil || !reflect.DeepEqual(policy, before) || policy.Granular.RequestPermissions != before.Granular.RequestPermissions {
		t.Fatal("closed granular failure changed receiver/reference")
	}
	var spawn p.SubAgentSourceThreadSpawn
	spawn.ThreadSpawn.AgentPath = p.Ptr("prior")
	spawnBefore := spawn
	if err := json.Unmarshal([]byte(`{"thread_spawn":{"depth":0,"parent_thread_id":"p"},"extra":true}`), &spawn); err == nil || !reflect.DeepEqual(spawn, spawnBefore) || spawn.ThreadSpawn.AgentPath != spawnBefore.ThreadSpawn.AgentPath {
		t.Fatal("closed spawn failure changed receiver/reference")
	}
	other := p.SubAgentSourceOther{Other: "prior"}
	if err := json.Unmarshal([]byte(`{"other":"new","extra":true}`), &other); err == nil || other.Other != "prior" {
		t.Fatal("closed other failure changed receiver")
	}
}

func FuzzClosedUnionObjectLevels(f *testing.F) {
	for _, seed := range []string{"plain", "escaped\n\"", "日本語", "custom", "subAgent", "granular"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed string) {
		if len(seed) > 256 {
			seed = seed[:256]
		}
		seed = strings.ToValidUTF8(seed, "\uFFFD")
		key := "extra-" + seed
		payload := map[string]any{"nested": []any{seed, true, nil, 0}}
		for _, tc := range []struct {
			body   string
			target any
		}{
			{`{"custom":""}`, &p.SessionSourceWrapper{}},
			{`{"subAgent":"review"}`, &p.SessionSourceWrapper{}},
			{`{"other":""}`, &p.SubAgentSourceOther{}},
			{`{"thread_spawn":{"depth":0,"parent_thread_id":""}}`, &p.SubAgentSourceThreadSpawn{}},
			{`{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false}}`, &p.AskForApprovalWrapper{}},
		} {
			var object map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.body), &object); err != nil {
				t.Fatal(err)
			}
			value, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			object[key] = value
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, tc.target); err == nil {
				t.Fatal("closed known outer object accepted arbitrary extra property")
			}
		}
		spawn := map[string]any{"depth": 0, "parent_thread_id": "", key: payload}
		granular := map[string]any{"mcp_elicitations": false, "rules": false, "sandbox_approval": false, key: payload}
		for _, tc := range []struct {
			body   any
			target any
		}{
			{map[string]any{"subAgent": map[string]any{"thread_spawn": spawn}}, &p.SessionSourceWrapper{}},
			{map[string]any{"granular": granular}, &p.AskForApprovalWrapper{}},
			{map[string]any{"future": payload, key: payload}, &p.SessionSourceWrapper{}},
		} {
			raw, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, tc.target); err != nil {
				t.Fatal("open nested or unknown root object was narrowed")
			}
		}
	})
}

func TestClosedUnionPriorErrorsAndEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		target    any
		want      error
		native    bool
	}{
		{"subagent-null", `{"subAgent":null,"extra":true}`, &p.SessionSourceWrapper{}, p.ErrNullResultField, false},
		{"other-null", `{"subAgent":{"other":null,"extra":true}}`, &p.SessionSourceWrapper{}, p.ErrNullResultField, false},
		{"spawn-required", `{"thread_spawn":{"parent_thread_id":"p"},"extra":true}`, &p.SubAgentSourceThreadSpawn{}, p.ErrMissingResultField, false},
		{"granular-required", `{"granular":{"rules":false,"sandbox_approval":false},"extra":true}`, &p.ApprovalPolicyGranular{}, p.ErrMissingResultField, false},
		{"granular-null", `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false,"request_permissions":null},"extra":true}`, &p.ApprovalPolicyGranular{}, p.ErrNullResultField, false},
		{"other-native", `{"other":42,"extra":true}`, &p.SubAgentSourceOther{}, nil, true},
		{"spawn-native", `{"thread_spawn":{"depth":"bad","parent_thread_id":"p"},"extra":true}`, &p.SubAgentSourceThreadSpawn{}, nil, true},
		{"granular-native", `{"granular":{"mcp_elicitations":42,"rules":false,"sandbox_approval":false},"extra":true}`, &p.ApprovalPolicyGranular{}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tc.raw), tc.target)
			var mismatch *json.UnmarshalTypeError
			if tc.native {
				if !errors.As(err, &mismatch) {
					t.Fatalf("older native error lost priority: %v", err)
				}
			} else if !errors.Is(err, tc.want) {
				t.Fatalf("older required/null error lost priority: %v", err)
			}
		})
	}
	var custom p.SessionSourceWrapper
	customErr := json.Unmarshal([]byte(`{"custom":null,"extra":true}`), &custom)
	if customErr == nil || !strings.Contains(customErr.Error(), `required field "custom" must not be null`) {
		t.Fatalf("existing custom-owner null error lost priority: %v", customErr)
	}
	var outer struct {
		Before int                    `json:"before"`
		Source p.SessionSourceWrapper `json:"source"`
		After  string                 `json:"after"`
	}
	err := json.Unmarshal([]byte(`{"before":"bad","source":{"custom":"x","extra":true},"after":"visited"}`), &outer)
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || outer.After != "visited" {
		t.Fatal("new source restriction displaced first enclosing error or traversal")
	}
}

func TestClosedUnionPlainCustomAndUnknownRoot(t *testing.T) {
	var envelope struct {
		p.SessionSourceCustom
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"custom":"name","after":"visited"}`), &envelope); err != nil || envelope.Custom != "name" || envelope.After != "visited" {
		t.Fatal("plain custom record acquired union-owner restrictions")
	}
	if _, ok := any(&p.SessionSourceCustom{}).(json.Unmarshaler); ok {
		t.Fatal("plain custom record acquired UnmarshalJSON")
	}
	if _, ok := any(p.SessionSourceCustom{}).(json.Marshaler); ok {
		t.Fatal("plain custom record acquired MarshalJSON")
	}
	data := []byte(`{"future":{"extra":{"preserved":true}},"CUSTOM":"future-alias"}`)
	var source p.SessionSourceWrapper
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	unknown, ok := source.Value.(p.UnknownSessionSource)
	if !ok || string(unknown.Raw) != string(data) {
		t.Fatal("future root shape changed")
	}
	data[2] = 'X'
	if strings.Contains(string(unknown.Raw), "Xuture") {
		t.Fatal("future source borrows input bytes")
	}
	large := `{"custom":"name","` + strings.Repeat("x", 128<<10) + `":{"ignored":"` + strings.Repeat("z", 128<<10) + `"}}`
	if err := json.Unmarshal([]byte(large), &source); err == nil || len(err.Error()) > 200 {
		t.Fatal("unknown field diagnostic is missing or retains the large peer name")
	}
}
