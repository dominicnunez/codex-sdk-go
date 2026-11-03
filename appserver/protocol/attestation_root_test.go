package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAttestationRoot(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"empty-object", `{}`, true}, {"extra-object", `{"future":{"n":1e999}}`, true}, {"null", `null`, false}, {"spaced-null", ` null `, false}, {"array", `[]`, false}, {"number", `7`, false}, {"bool", `false`, false}, {"string", `"x"`, false}, {"missing", ``, false}, {"malformed", `{`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMockTransport()
			c := p.NewClient(m)
			defer c.Close()
			calls := 0
			c.SetApprovalHandlers(p.ApprovalHandlers{OnAttestationGenerate: func(context.Context, p.AttestationGenerateParams) (p.AttestationGenerateResponse, error) {
				calls++
				return p.AttestationGenerateResponse{Token: "synthetic-attestation"}, nil
			}})
			r, e := m.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "fixture"}, Method: "attestation/generate", Params: json.RawMessage(tc.body)})
			if tc.valid {
				if e != nil || r.Error != nil || calls != 1 {
					t.Fatalf("valid object rejected: %v", e)
				}
				var fields map[string]json.RawMessage
				if e := json.Unmarshal(r.Result, &fields); e != nil || string(fields["token"]) != `"synthetic-attestation"` {
					t.Fatal("reply changed")
				}
			} else if calls != 0 || len(r.Result) != 0 || !errors.Is(e, p.ErrInvalidParams) {
				t.Fatalf("invalid params invoked handler/published token: calls=%d result=%s error=%v", calls, r.Result, e)
			}
		})
	}
}
