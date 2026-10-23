package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestStringObjectUnionsRejectNull(t *testing.T) {
	for _, input := range []string{`null`, " \n null \t"} {
		for _, tc := range []struct {
			name      string
			value     json.Unmarshaler
			unchanged string
		}{
			{"session", &p.SessionSourceWrapper{Value: p.SessionSourceCLI}, `"cli"`},
			{"approval", &p.AskForApprovalWrapper{Value: p.ApprovalPolicyNever}, `"never"`},
		} {
			t.Run(tc.name+input, func(t *testing.T) {
				if err := tc.value.UnmarshalJSON([]byte(input)); !errors.Is(err, p.ErrNullResultField) {
					t.Errorf("decode null = %v, want ErrNullResultField", err)
				}
				assertUnionJSON(t, tc.value, tc.unchanged)
			})
		}
	}
}

func TestStringObjectUnionsKeepMalformedNullErrors(t *testing.T) {
	for _, input := range []string{"\vnull", "\u00a0null"} {
		for _, target := range []json.Unmarshaler{&p.SessionSourceWrapper{}, &p.AskForApprovalWrapper{}} {
			err := target.UnmarshalJSON([]byte(input))
			if err == nil || errors.Is(err, p.ErrNullResultField) {
				t.Errorf("malformed input %q classified as null: %v", input, err)
			}
		}
	}
}

func TestThreadReadRejectsNullSubAgentBeforeCache(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cached bool
	}{{"fresh", false}, {"existing", true}} {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewMockTransport()
			thread := validThreadPayload("t")
			thread["source"] = map[string]any{"subAgent": nil}
			if err := transport.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
				t.Fatal(err)
			}
			client := p.NewClient(transport)
			if tc.cached {
				client.CacheThreadState(p.Thread{ID: "t", Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}})
			}
			updates := 0
			remove := client.AddThreadStateListener("t", func(p.Thread) { updates++ }, nil)
			defer remove()
			updates = 0 // Ignore the initial replay of a preexisting valid snapshot.
			if _, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"}); !errors.Is(err, p.ErrNullResultField) {
				t.Errorf("Thread.Read = %v, want null error", err)
			}
			snapshot, ok := client.ThreadStateSnapshot("t")
			if ok != tc.cached || updates != 0 {
				t.Errorf("invalid source published: cached=%v, updates=%d", ok, updates)
			}
			if ok {
				assertUnionJSON(t, snapshot.Source, `"cli"`)
			}
		})
	}
}

func TestSessionSourceNullDuplicateCannotBeRepaired(t *testing.T) {
	for _, raw := range []string{
		`{"subAgent":null,"subAgent":"review"}`,
		`{"subAgent":"review","subAgent":null}`,
		`{"subAgent":null,"sub\u0041gent":"review"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			value := p.SessionSourceWrapper{Value: p.SessionSourceCLI}
			if err := json.Unmarshal([]byte(raw), &value); !errors.Is(err, p.ErrNullResultField) {
				t.Errorf("decode = %v, want null error", err)
			}
			assertUnionJSON(t, value, `"cli"`)
		})
	}
}

func TestThreadStartedRejectsNullSubAgentBeforeDelivery(t *testing.T) {
	transport := NewMockTransport()
	var failures []error
	client := p.NewClient(transport, p.WithHandlerErrorCallback(func(_ string, err error) { failures = append(failures, err) }))
	typed := 0
	client.OnThreadStarted(func(p.ThreadStartedNotification) { typed++ })
	thread := validThreadPayload("t")
	thread["source"] = map[string]any{"subAgent": nil}
	data, err := json.Marshal(map[string]any{"thread": thread})
	if err != nil {
		t.Fatal(err)
	}
	transport.InjectServerNotification(context.Background(), p.Notification{Method: "thread/started", Params: data})
	if _, cached := client.ThreadStateSnapshot("t"); cached || typed != 0 {
		t.Errorf("invalid notification published: cached=%v, handlers=%d", cached, typed)
	}
	if len(failures) == 0 {
		t.Fatal("missing decode diagnostic")
	}
	for _, failure := range failures {
		if !errors.Is(failure, p.ErrNullResultField) {
			t.Errorf("unexpected decode diagnostic: %v", failure)
		}
	}
}

func TestConfigRequirementsRejectsNullPolicyElement(t *testing.T) {
	for _, policies := range []string{`[null]`, `["never",null]`, `[null,"never"]`} {
		t.Run(policies, func(t *testing.T) {
			transport := NewMockTransport()
			transport.SetResponse("configRequirements/read", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"requirements":{"allowedApprovalPolicies":` + policies + `}}`)})
			if _, err := p.NewClient(transport).Config.ReadRequirements(context.Background()); !errors.Is(err, p.ErrNullResultField) {
				t.Errorf("ReadRequirements = %v, want null error", err)
			}
		})
	}
}

func TestStringObjectUnionNullableCarriers(t *testing.T) {
	var config p.Config
	if err := json.Unmarshal([]byte(`{"approval_policy":null}`), &config); err != nil || config.ApprovalPolicy != nil {
		t.Fatalf("nullable policy = %#v, %v", config.ApprovalPolicy, err)
	}
	for _, tc := range []struct {
		raw    string
		length int
	}{{`{"allowedApprovalPolicies":null}`, -1}, {`{"allowedApprovalPolicies":[]}`, 0}, {`{"allowedApprovalPolicies":["never"]}`, 1}} {
		var requirements p.ConfigRequirements
		if err := json.Unmarshal([]byte(tc.raw), &requirements); err != nil {
			t.Fatal(err)
		}
		policies := requirements.AllowedApprovalPolicies
		if tc.length < 0 {
			if policies != nil {
				t.Fatal("null list became present")
			}
		} else if policies == nil || *policies == nil || len(*policies) != tc.length {
			t.Fatalf("list presence or length lost: %#v", policies)
		}
	}
	for _, raw := range []string{`""`, `"future"`} {
		var session p.SessionSourceWrapper
		if err := json.Unmarshal([]byte(raw), &session); err != nil {
			t.Fatal(err)
		}
		assertUnionJSON(t, session, raw)
		var approval p.AskForApprovalWrapper
		if err := json.Unmarshal([]byte(raw), &approval); err != nil {
			t.Fatal(err)
		}
		assertUnionJSON(t, approval, raw)
	}
	var future p.SessionSourceWrapper
	if err := json.Unmarshal([]byte(`{"future":42}`), &future); err != nil {
		t.Fatal(err)
	}
	assertUnionJSON(t, future, `{"future":42}`)
}
