package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestResponseEnumRejectsHiddenInvalidOccurrence(t *testing.T) {
	for _, tc := range responseEnumCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			wire := json.RawMessage(fmt.Sprintf(`{%s%q:"future",%q:%q}`, tc.prefix, tc.field, tc.field, tc.values[0]))
			if err := json.Unmarshal(wire, tc.newResponse()); err == nil {
				t.Error("record accepted hidden invalid enum")
			}
			mock := NewMockTransport()
			mock.SetResponse(tc.method, p.Response{Result: wire})
			if _, err := tc.call(p.NewClient(mock)); err == nil {
				t.Error("public method accepted hidden invalid enum")
			}
		})
	}
}

func TestResponseEnumOccurrenceContract(t *testing.T) {
	for _, owner := range responseEnumCases(t) {
		t.Run(owner.name, func(t *testing.T) {
			key, _ := json.Marshal(owner.field)
			escaped := fmt.Sprintf(`"\u%04x%s"`, owner.field[0], owner.field[1:])
			first, last := owner.values[0], owner.values[len(owner.values)-1]
			final := fmt.Sprintf(`%s:%q`, key, last)
			cases := []struct {
				name, fields, want string
			}{
				{"valid duplicates", fmt.Sprintf(`%s:%q,%s`, key, first, final), last},
				{"escaped invalid", fmt.Sprintf(`%s:"future",%s`, escaped, final), ""},
				{"invalid last", fmt.Sprintf(`%s,%s:"future"`, final, key), ""},
				{"empty first", fmt.Sprintf(`%s:"",%s`, key, final), ""},
				{"null first", fmt.Sprintf(`%s:null,%s`, key, final), ""},
				{"type first", fmt.Sprintf(`%s:1,%s`, key, final), ""},
				{"long invalid", fmt.Sprintf(`%s:%q,%s`, key, strings.Repeat("x", 16000), final), ""},
				{"missing", `"extra":true`, ""},
				{"nested extra", fmt.Sprintf(`"extra":{%s:"future"},%s`, key, final), last},
			}
			aliasWant := ""
			if owner.field == "outcome" {
				aliasWant = last // This owner matches exact decoded names only.
			}
			cases = append(cases, struct{ name, fields, want string }{"case alias", fmt.Sprintf(`%q:"future",%s`, strings.ToUpper(owner.field), final), aliasWant})
			for _, value := range owner.values {
				cases = append(cases, struct{ name, fields, want string }{value, fmt.Sprintf(`%s:%q`, key, value), value})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					wire := json.RawMessage("{" + owner.prefix + tc.fields + "}")
					decoded := owner.newResponse()
					valid := json.RawMessage("{" + owner.prefix + final + "}")
					if err := json.Unmarshal(valid, decoded); err != nil {
						t.Fatal(err)
					}
					before, err := json.Marshal(decoded)
					if err != nil {
						t.Fatal(err)
					}
					err = json.Unmarshal(wire, decoded)
					if (err == nil) != (tc.want != "") {
						t.Fatalf("record error=%v, want enum %q", err, tc.want)
					}
					after, marshalErr := json.Marshal(decoded)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					if err != nil && string(before) != string(after) {
						t.Fatal("rejected response changed receiver")
					}
					if err != nil && len(err.Error()) > 4096 {
						t.Fatal("unbounded enum diagnostic")
					}
					mock := NewMockTransport()
					mock.SetResponse(owner.method, p.Response{Result: wire})
					client := p.NewClient(mock)
					result, err := owner.call(client)
					if (err == nil) != (tc.want != "") {
						t.Fatalf("public call error=%v, want enum %q", err, tc.want)
					}
					if tc.want != "" {
						checkResponseEnumValue(t, result, owner.field, tc.want)
						checkResponseEnumValue(t, decoded, owner.field, tc.want)
					} else {
						checkResponseEnumValue(t, result, owner.field, "")
						mock.SetResponse(owner.method, p.Response{Result: valid})
						result, err = owner.call(client)
						if err != nil {
							t.Fatalf("valid response after rejection: %v", err)
						}
						checkResponseEnumValue(t, result, owner.field, last)
					}
				})
			}
		})
	}
}

func checkResponseEnumValue(t *testing.T, response any, field, want string) {
	t.Helper()
	wire, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := json.Unmarshal(fields[field], &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s=%q, want %q", field, got, want)
	}
}

func TestResponseEnumPreservesExistingErrors(t *testing.T) {
	for _, tc := range responseEnumCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			wire := fmt.Sprintf(`{%s%q:"earlier",%q:"later"}`, tc.prefix, tc.field, tc.field)
			err := json.Unmarshal([]byte(wire), tc.newResponse())
			if err == nil || !strings.Contains(err.Error(), `"later"`) || strings.Contains(err.Error(), `"earlier"`) {
				t.Fatalf("existing final-value error changed: %v", err)
			}
			if strings.HasPrefix(tc.name, "config") {
				wire = `{"filePath":"relative","version":"v1","status":"future","status":"ok"}`
				if err := json.Unmarshal([]byte(wire), tc.newResponse()); err == nil || !strings.Contains(err.Error(), "filePath") {
					t.Fatalf("existing path error replaced: %v", err)
				}
			}
		})
	}
}

type responseEnumCase struct {
	name, method, field, prefix string
	values                      []string
	newResponse                 func() any
	call                        func(*p.Client) (any, error)
}

func responseEnumCases(t *testing.T) []responseEnumCase {
	t.Helper()
	configPrefix := fmt.Sprintf(`"filePath":%q,"version":"v1",`, filepath.Join(t.TempDir(), "config.toml"))
	return []responseEnumCase{
		{"plugin install", "plugin/install", "authPolicy", `"appsNeedingAuth":[],`, []string{"ON_INSTALL", "ON_USE"},
			func() any { return new(p.PluginInstallResponse) },
			func(c *p.Client) (any, error) {
				return c.Plugin.Install(context.Background(), p.PluginInstallParams{MarketplacePath: filepath.Join(t.TempDir(), "marketplace"), PluginName: "test"})
			}},
		{"nudge email", "account/sendAddCreditsNudgeEmail", "status", "", []string{"sent", "cooldown_active"},
			func() any { return new(p.SendAddCreditsNudgeEmailResponse) },
			func(c *p.Client) (any, error) {
				return c.Account.SendAddCreditsNudgeEmail(context.Background(), p.SendAddCreditsNudgeEmailParams{CreditType: p.AddCreditsNudgeCreditTypeCredits})
			}},
		{"reset credit", "account/rateLimitResetCredit/consume", "outcome", "", []string{"reset", "nothingToReset", "noCredit", "alreadyRedeemed"},
			func() any { return new(p.ConsumeAccountRateLimitResetCreditResponse) },
			func(c *p.Client) (any, error) {
				return c.Account.ConsumeRateLimitResetCredit(context.Background(), p.ConsumeAccountRateLimitResetCreditParams{IdempotencyKey: "test"})
			}},
		{"unsubscribe", "thread/unsubscribe", "status", "", []string{"notLoaded", "notSubscribed", "unsubscribed"},
			func() any { return new(p.ThreadUnsubscribeResponse) },
			func(c *p.Client) (any, error) {
				return c.Thread.Unsubscribe(context.Background(), p.ThreadUnsubscribeParams{ThreadID: "thread"})
			}},
		{"cancel login", "account/login/cancel", "status", "", []string{"canceled", "notFound"},
			func() any { return new(p.CancelLoginAccountResponse) },
			func(c *p.Client) (any, error) {
				return c.Account.CancelLogin(context.Background(), p.CancelLoginAccountParams{LoginId: "login"})
			}},
		{"config write", "config/value/write", "status", configPrefix, []string{"ok", "okOverridden"},
			func() any { return new(p.ConfigWriteResponse) },
			func(c *p.Client) (any, error) {
				return c.Config.Write(context.Background(), p.ConfigValueWriteParams{KeyPath: "test", MergeStrategy: p.MergeStrategyReplace, Value: json.RawMessage(`true`)})
			}},
		{"config batch write", "config/batchWrite", "status", configPrefix, []string{"ok", "okOverridden"},
			func() any { return new(p.ConfigWriteResponse) },
			func(c *p.Client) (any, error) {
				return c.Config.BatchWrite(context.Background(), p.ConfigBatchWriteParams{Edits: []p.ConfigEdit{}})
			}},
	}
}
