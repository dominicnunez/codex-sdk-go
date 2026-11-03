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

func TestInboundMetadataEnumNotifications(t *testing.T) {
	for _, owner := range []struct {
		method, base, field, valid string
		on                         func(*p.Client, func(*string))
		add                        func(*p.Client, func(*string)) func()
	}{
		{"account/login/completed", `"success":true`, "onboardingEntrypoint", "life_sciences", func(c *p.Client, f func(*string)) {
			c.OnAccountLoginCompleted(func(n p.AccountLoginCompletedNotification) { f(n.OnboardingEntrypoint) })
		}, func(c *p.Client, f func(*string)) func() {
			return c.AddAccountLoginCompletedListener(func(n p.AccountLoginCompletedNotification) { f(n.OnboardingEntrypoint) })
		}},
		{"mcpServer/startupStatus/updated", `"name":"server","status":"failed"`, "failureReason", "reauthenticationRequired", func(c *p.Client, f func(*string)) {
			c.OnMcpServerStatusUpdated(func(n p.McpServerStatusUpdatedNotification) { f(n.FailureReason) })
		}, func(c *p.Client, f func(*string)) func() {
			return c.AddMcpServerStatusUpdatedListener(func(n p.McpServerStatusUpdatedNotification) { f(n.FailureReason) })
		}},
	} {
		validJSON, _ := json.Marshal(owner.valid)
		for _, tc := range []struct {
			name, suffix string
			valid        bool
		}{
			{"absent", "", true}, {"null", fmt.Sprintf(`,"%s":null`, owner.field), true}, {"current", fmt.Sprintf(`,"%s":%s`, owner.field, validJSON), true},
			{"bogus", fmt.Sprintf(`,"%s":"bogus"`, owner.field), false}, {"empty", fmt.Sprintf(`,"%s":""`, owner.field), false},
			{"hidden", fmt.Sprintf(`,"%s":"bogus","%s":%s`, owner.field, owner.field, validJSON), false},
			{"escaped", fmt.Sprintf(`,"\u%04x%s":%s`, owner.field[0], owner.field[1:], validJSON), true},
			{"escaped-invalid", fmt.Sprintf(`,"\u%04x%s":"bogus"`, owner.field[0], owner.field[1:]), false},
			{"ignored-uppercase", fmt.Sprintf(`,"%s":"bogus"`, strings.ToUpper(owner.field)), true},
			{"valid-reset", fmt.Sprintf(`,"%s":%s,"%s":null`, owner.field, validJSON, owner.field), true},
			{"valid-last", fmt.Sprintf(`,"%s":null,"%s":%s`, owner.field, owner.field, validJSON), true},
			{"reset", fmt.Sprintf(`,"%s":"bogus","%s":null`, owner.field, owner.field), false},
		} {
			t.Run(owner.field+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				diagnostics := 0
				calls := 0
				c := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, _ error) {
					if origin != owner.method {
						t.Errorf("origin %s", origin)
					}
					diagnostics++
				}))
				t.Cleanup(func() { _ = c.Close() })
				var received []*string
				capture := func(value *string) { calls++; received = append(received, value) }
				owner.on(c, capture)
				remove := owner.add(c, capture)
				defer remove()
				raw := json.RawMessage(`{` + owner.base + tc.suffix + `}`)
				mock.InjectServerNotification(context.Background(), p.Notification{Method: owner.method, Params: raw})
				if tc.valid {
					if calls != 2 || diagnostics != 0 {
						t.Fatalf("valid control calls=%d diagnostics=%d", calls, diagnostics)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(raw, &fields); err != nil {
						t.Fatal(err)
					}
					var want *string
					if field, ok := fields[owner.field]; ok {
						if err := json.Unmarshal(field, &want); err != nil {
							t.Fatal(err)
						}
					}
					for _, value := range received {
						if !reflect.DeepEqual(value, want) {
							t.Fatalf("value changed: %v vs %v", value, want)
						}
					}
				} else if calls != 0 || diagnostics == 0 {
					t.Errorf("invalid enum published: calls=%d diagnostics=%d payload=%s", calls, diagnostics, raw)
				} else {
					before := diagnostics
					recovery := json.RawMessage(`{` + owner.base + fmt.Sprintf(`,"%s":%s}`, owner.field, validJSON))
					mock.InjectServerNotification(context.Background(), p.Notification{Method: owner.method, Params: recovery})
					if calls != 2 || diagnostics != before {
						t.Fatal("same-client notification recovery failed")
					}
				}
			})
		}
	}
}

func TestRequirementsCredentialStoreEnum(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"absent", `{}`, true}, {"null", `{"cliAuthCredentialsStore":null}`, true},
		{"file", `{"cliAuthCredentialsStore":"file"}`, true}, {"keyring", `{"cliAuthCredentialsStore":"keyring"}`, true},
		{"auto", `{"cliAuthCredentialsStore":"auto"}`, true}, {"ephemeral", `{"cliAuthCredentialsStore":"ephemeral"}`, true},
		{"bogus", `{"cliAuthCredentialsStore":"bogus"}`, false}, {"empty", `{"cliAuthCredentialsStore":""}`, false},
		{"number", `{"cliAuthCredentialsStore":17}`, false}, {"object", `{"cliAuthCredentialsStore":{}}`, false},
		{"array", `{"cliAuthCredentialsStore":[]}`, false}, {"boolean", `{"cliAuthCredentialsStore":false}`, false},
		{"hidden", `{"cliAuthCredentialsStore":"bogus","cliAuthCredentialsStore":"file"}`, false},
		{"reset", `{"cliAuthCredentialsStore":"bogus","cliAuthCredentialsStore":null}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":` + tc.body + `}`)})
			got, err := c.Config.ReadRequirements(context.Background())
			if tc.valid {
				if err != nil || got.Requirements == nil {
					t.Fatalf("valid control result=%+v err=%v", got, err)
				}
			} else {
				if err == nil {
					t.Fatalf("invalid credential-store enum published: %s", tc.body)
				}
				if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
					t.Fatal("partial response")
				}
				mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"cliAuthCredentialsStore":"file"}}`)})
				if _, err = c.Config.ReadRequirements(context.Background()); err != nil {
					t.Fatalf("recovery: %v", err)
				}
			}
		})
	}
}
