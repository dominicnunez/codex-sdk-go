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

func TestRequirementEnumMatchingAndRawOwnership(t *testing.T) {
	for _, tc := range []struct{ field, valid, bad string }{
		{"cliAuthCredentialsStore", `"file"`, `"bogus"`},
		{"allowedLoginMethods", `["chatgpt","api"]`, `[null]`},
		{"allowedWindowsSandboxImplementations", `["elevated","mxc"]`, `["bogus"]`},
	} {
		t.Run(tc.field, func(t *testing.T) {
			escaped := `"\u` + fmt.Sprintf("%04x", tc.field[0]) + tc.field[1:] + `"`
			for _, body := range []string{
				`{"REQUIREMENTS":{"` + strings.ToUpper(tc.field) + `":` + tc.bad + `}}`,
				`{"requirements":{` + escaped + `:` + tc.bad + `}}`,
				`{"requirements":{"` + tc.field + `":` + tc.bad + `},"requirements":null}`,
				`{"requirements":{"` + tc.field + `":` + tc.bad + `},"REQUIREMENTS":{"` + tc.field + `":` + tc.valid + `}}`,
			} {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(body)})
				got, err := c.Config.ReadRequirements(context.Background())
				if err == nil || !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
					t.Fatalf("invalid occurrence published: %s %v", body, err)
				}
			}
			for _, body := range []string{
				`{"requirements":{"` + tc.field + `":null,"` + strings.ToUpper(tc.field) + `": ` + tc.valid + `}}`,
				`{"REQUIREMENTS":{` + escaped + `: ` + tc.valid + `},"requirements":{"future":1e10000}}`,
				`{"requirements":{"` + tc.field + `":` + tc.valid + `},"requirements":null}`,
			} {
				wire := json.RawMessage(body)
				var want p.ConfigRequirementsReadResponse
				if err := json.Unmarshal(wire, &want); err != nil {
					t.Fatal(err)
				}
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				mock.SetResponse("configRequirements/read", p.Response{Result: wire})
				got, err := c.Config.ReadRequirements(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				for i := range wire {
					wire[i] = ' '
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("valid native selection/raw bytes changed: %+v vs %+v", got, want)
				}
			}
		})
	}
}

func TestMetadataEnumReceiverAndPriorErrors(t *testing.T) {
	loginValue, mcpValue := "prior-login", "prior-failure"
	login := p.AccountLoginCompletedNotification{Success: true, OnboardingEntrypoint: &loginValue}
	mcp := p.McpServerStatusUpdatedNotification{Name: "prior", FailureReason: &mcpValue}
	loginBefore, mcpBefore := login, mcp
	if err := json.Unmarshal([]byte(`{"success":false,"onboardingEntrypoint":"bogus","onboardingEntrypoint":null}`), &login); err == nil {
		t.Fatal("invalid login accepted")
	}
	if err := json.Unmarshal([]byte(`{"name":"new","status":"failed","failureReason":"bogus","failureReason":null}`), &mcp); err == nil {
		t.Fatal("invalid startup accepted")
	}
	if !reflect.DeepEqual(login, loginBefore) || login.OnboardingEntrypoint != loginBefore.OnboardingEntrypoint || !reflect.DeepEqual(mcp, mcpBefore) || mcp.FailureReason != mcpBefore.FailureReason {
		t.Fatal("new rejection changed receiver references")
	}
	for _, tc := range []struct {
		body, valid, bad string
		decode           func([]byte) error
	}{
		{`{"success":42,"onboardingEntrypoint":@VALUE@}`, `"life_sciences"`, `"bogus"`, login.UnmarshalJSON},
		{`{"onboardingEntrypoint":@VALUE@}`, `"life_sciences"`, `"bogus"`, login.UnmarshalJSON},
		{`{"name":"s","failureReason":@VALUE@,"status":"bad"}`, `"reauthenticationRequired"`, `"bogus"`, mcp.UnmarshalJSON},
	} {
		control := tc.decode([]byte(strings.Replace(tc.body, "@VALUE@", tc.valid, 1)))
		mixed := tc.decode([]byte(strings.Replace(tc.body, "@VALUE@", tc.bad, 1)))
		if control == nil || mixed == nil || control.Error() != mixed.Error() {
			t.Fatalf("old notification check changed: %v vs %v", control, mixed)
		}
	}
	// Requirements native and structured metadata checks precede the new raw
	// enum checks, even when the new malformed member appears earlier on the wire.
	for _, suffix := range []string{`"models":true`, `"allowLoginShell":42`, `"browserUse":{"origins":{"site":null}}`} {
		var errs []error
		for _, value := range []string{`"file"`, `"bogus"`} {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"cliAuthCredentialsStore":` + value + `,` + suffix + `}}`)})
			_, err := c.Config.ReadRequirements(context.Background())
			errs = append(errs, err)
		}
		if errs[0] == nil || errs[1] == nil || errs[0].Error() != errs[1].Error() {
			t.Fatalf("old requirements check changed: %v vs %v", errs[0], errs[1])
		}
	}
	// Public requirements records remain ordinary raw representations outside
	// their service admission owner, including anonymous application envelopes.
	var plain struct {
		p.ConfigRequirements
		Extra bool
	}
	if err := json.Unmarshal([]byte(`{"cliAuthCredentialsStore":false,"allowedLoginMethods":[null],"allowedWindowsSandboxImplementations":42,"Extra":true}`), &plain); err != nil || !plain.Extra {
		t.Fatalf("plain records changed: %v", err)
	}
}

func TestMetadataEnumBoundedDiagnostics(t *testing.T) {
	raw, _ := json.Marshal(strings.Repeat("peer-", 20000))
	for _, tc := range []struct {
		body   string
		decode func([]byte) error
	}{
		{`{"success":true,"onboardingEntrypoint":` + string(raw) + `}`, new(p.AccountLoginCompletedNotification).UnmarshalJSON},
		{`{"name":"s","status":"failed","failureReason":` + string(raw) + `}`, new(p.McpServerStatusUpdatedNotification).UnmarshalJSON},
	} {
		err := tc.decode([]byte(tc.body))
		if err == nil || len(err.Error()) > 2048 {
			t.Fatal("unbounded enum diagnostic")
		}
	}
	for _, field := range []string{"cliAuthCredentialsStore", "allowedLoginMethods", "allowedWindowsSandboxImplementations"} {
		value := string(raw)
		if field != "cliAuthCredentialsStore" {
			value = `[` + value + `]`
		}
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"` + field + `":` + value + `}}`)})
		got, err := c.Config.ReadRequirements(context.Background())
		if err == nil || len(err.Error()) > 2048 || !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
			t.Fatal("unbounded requirements diagnostic or partial result")
		}
	}
}
