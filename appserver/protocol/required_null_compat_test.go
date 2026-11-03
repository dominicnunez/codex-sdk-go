package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRequiredNullPriorErrorAndTraversal(t *testing.T) {
	var envelope struct {
		Before int            `json:"before"`
		Skill  p.SkillSummary `json:"skill"`
		After  string         `json:"after"`
	}
	raw := `{"before":"bad","skill":{"enabled":null,"description":"d","enabled":true,"name":"s"},"after":"visited"}`
	var mismatch *json.UnmarshalTypeError
	err := json.Unmarshal([]byte(raw), &envelope)
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || envelope.After != "visited" {
		t.Fatalf("first error/traversal changed: %v", err)
	}
	var skill p.SkillSummary
	err = json.Unmarshal([]byte(`{"enabled":null,"enabled":true,"name":"s"}`), &skill)
	if err == nil || !strings.Contains(err.Error(), "missing plugin.skill.description") {
		t.Fatalf("old required error lost priority: %v", err)
	}
	err = json.Unmarshal([]byte(`{"enabled":null,"enabled":true,"description":42,"name":"s"}`), &skill)
	if !errors.As(err, &mismatch) || mismatch.Field != "description" {
		t.Fatalf("old native error lost priority: %v", err)
	}
}

func TestInitializeNullCarrierCompatibility(t *testing.T) {
	var envelope struct {
		p.InitializeResponse
		After string `json:"after"`
	}
	raw := `{"codexHome":null,"codexHome":"/workspace","platformFamily":"family","platformOs":"target","userAgent":"agent","after":"visited"}`
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || envelope.After != "visited" || envelope.CodexHome != "/workspace" {
		t.Fatalf("plain response embedding changed: %v", err)
	}
	if _, ok := any(&p.InitializeResponse{}).(json.Unmarshaler); ok {
		t.Fatal("public response acquired JSON method")
	}
	for _, tc := range []struct {
		raw, old string
		native   bool
	}{
		{`{"codexHome":"/workspace","platformFamily":"family","platformOs":"target","userAgent":null,"userAgent":"agent","platformFamily":42}`, "", true},
		{`{"codexHome":"/workspace","platformFamily":"family","platformOs":"target","userAgent":null,"userAgent":"agent","codexHome":""}`, "missing codexHome", false},
	} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		mock.SetResponse("initialize", p.Response{Result: json.RawMessage(tc.raw)})
		result, err := client.Initialize(context.Background(), p.InitializeParams{ClientInfo: p.ClientInfo{Name: "fixture", Version: "1"}})
		var mismatch *json.UnmarshalTypeError
		if result != (p.InitializeResponse{}) || err == nil {
			t.Fatal("invalid init published")
		}
		if tc.native {
			if !errors.As(err, &mismatch) || mismatch.Struct != "InitializeResponse" {
				t.Fatalf("native public error context changed: %v", err)
			}
		} else if !strings.Contains(err.Error(), tc.old) {
			t.Fatalf("old init semantic error lost priority: %v", err)
		}
		if _, latched := client.InitializedParams(); latched {
			t.Fatal("failed initialization latched")
		}
		client.Close()
	}
}

func TestNestedRequiredNullReceiverReferences(t *testing.T) {
	raw := json.RawMessage(`{"future":{"prior":true}}`)
	source := p.SessionSourceWrapper{Value: p.UnknownSessionSource{Raw: raw}}
	sourceBad := `{"subAgent":{"thread_spawn":{"depth":null}},"subAgent":{"thread_spawn":{"depth":0,"parent_thread_id":"p"}}}`
	if err := json.Unmarshal([]byte(sourceBad), &source); !errors.Is(err, p.ErrNullResultField) {
		t.Fatal(err)
	}
	retained, ok := source.Value.(p.UnknownSessionSource)
	if !ok || &retained.Raw[0] != &raw[0] {
		t.Fatal("nested source rejection replaced prior union reference")
	}
	approval := p.AskForApprovalWrapper{Value: p.UnknownAskForApproval{Raw: raw}}
	bad := `{"granular":{"rules":null},"granular":{"mcp_elicitations":false,"rules":true,"sandbox_approval":false}}`
	if err := json.Unmarshal([]byte(bad), &approval); !errors.Is(err, p.ErrNullResultField) {
		t.Fatal(err)
	}
	policy, ok := approval.Value.(p.UnknownAskForApproval)
	if !ok || &policy.Raw[0] != &raw[0] {
		t.Fatal("nested approval rejection replaced prior union reference")
	}
	var granular p.ApprovalPolicyGranular
	prior := p.Ptr(true)
	granular.Granular.RequestPermissions = prior
	if err := json.Unmarshal([]byte(bad), &granular); !errors.Is(err, p.ErrNullResultField) || granular.Granular.RequestPermissions != prior {
		t.Fatal("granular receiver reference changed")
	}
	var spawn p.SubAgentSourceThreadSpawn
	path := p.Ptr("prior")
	spawn.ThreadSpawn.AgentPath = path
	bad = `{"thread_spawn":{"depth":null},"thread_spawn":{"depth":0,"parent_thread_id":"p"}}`
	if err := json.Unmarshal([]byte(bad), &spawn); !errors.Is(err, p.ErrNullResultField) || spawn.ThreadSpawn.AgentPath != path {
		t.Fatal("spawn receiver reference changed")
	}
}
