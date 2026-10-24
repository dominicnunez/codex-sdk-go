package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type pluginEnumCase struct {
	field    string
	values   []string
	nullable bool
}

func pluginEnumCases() []pluginEnumCase {
	return []pluginEnumCase{
		{"authPolicy", []string{"ON_INSTALL", "ON_USE"}, false},
		{"installPolicy", []string{"NOT_AVAILABLE", "AVAILABLE", "INSTALLED_BY_DEFAULT"}, false},
		{"availability", []string{"DISABLED_BY_ADMIN", "AVAILABLE"}, false},
	}
}

func pluginEnumSummary(t *testing.T, field, members string) string {
	t.Helper()
	fields := map[string]string{"authPolicy": `"ON_USE"`, "installPolicy": `"AVAILABLE"`, "availability": `"AVAILABLE"`}
	delete(fields, field)
	var prefix strings.Builder
	for _, key := range []string{"authPolicy", "installPolicy", "availability"} {
		if value, ok := fields[key]; ok {
			fmt.Fprintf(&prefix, "%q:%s,", key, value)
		}
	}
	return fmt.Sprintf(`{%s"enabled":true,"id":"plugin","installed":true,"name":"calendar","source":{"path":%q,"type":"local"},%s}`, prefix.String(), filepath.Join(t.TempDir(), "plugin"), members)
}

type pluginEnumCarrier struct {
	method string
	wrap   func(string) string
	call   func(*p.Client) (p.PluginSummary, error)
}

func pluginEnumCarriers(t *testing.T) []pluginEnumCarrier {
	t.Helper()
	marketplace := func(s string) string { return `{"marketplaces":[{"name":"official","plugins":[` + s + `]}]}` }
	return []pluginEnumCarrier{
		{"plugin/list", marketplace, func(c *p.Client) (p.PluginSummary, error) {
			r, e := c.Plugin.List(context.Background(), p.PluginListParams{})
			if e != nil {
				if !reflect.DeepEqual(r, p.PluginListResponse{}) {
					t.Error("failed public call returned a nonzero response")
				}
				return p.PluginSummary{}, e
			}
			if len(r.Marketplaces) == 0 || len(r.Marketplaces[0].Plugins) == 0 {
				t.Fatal("successful public call omitted the expected plugin")
			}
			return r.Marketplaces[0].Plugins[0], nil
		}},
		{"plugin/installed", marketplace, func(c *p.Client) (p.PluginSummary, error) {
			r, e := c.Plugin.Installed(context.Background(), p.PluginInstalledParams{})
			if e != nil {
				if !reflect.DeepEqual(r, p.PluginInstalledResponse{}) {
					t.Error("failed public call returned a nonzero response")
				}
				return p.PluginSummary{}, e
			}
			if len(r.Marketplaces) == 0 || len(r.Marketplaces[0].Plugins) == 0 {
				t.Fatal("successful public call omitted the expected plugin")
			}
			return r.Marketplaces[0].Plugins[0], nil
		}},
		{"plugin/read", func(s string) string {
			return `{"plugin":{"apps":[],"appTemplates":[],"hooks":[],"marketplaceName":"official","mcpServers":[],"skills":[],"summary":` + s + `}}`
		}, func(c *p.Client) (p.PluginSummary, error) {
			r, e := c.Plugin.Read(context.Background(), p.PluginReadParams{MarketplacePath: filepath.Join(t.TempDir(), "marketplace"), PluginName: "calendar"})
			if e != nil && !reflect.DeepEqual(r, p.PluginReadResponse{}) {
				t.Error("failed public call returned a nonzero response")
			}
			return r.Plugin.Summary, e
		}},
		{"plugin/share/list", func(s string) string { return `{"data":[{"plugin":` + s + `}]}` }, func(c *p.Client) (p.PluginSummary, error) {
			r, e := c.Plugin.ShareList(context.Background(), p.PluginShareListParams{})
			if e != nil {
				if !reflect.DeepEqual(r, p.PluginShareListResponse{}) {
					t.Error("failed public call returned a nonzero response")
				}
				return p.PluginSummary{}, e
			}
			if len(r.Data) == 0 {
				t.Fatal("successful public call omitted the expected plugin")
			}
			return r.Data[0].Plugin, nil
		}},
	}
}

func TestPluginSummaryRejectsHiddenEnum(t *testing.T) {
	for _, tc := range pluginEnumCases() {
		t.Run(tc.field, func(t *testing.T) {
			raw := pluginEnumSummary(t, tc.field, fmt.Sprintf(`%q:"future",%q:%q`, tc.field, tc.field, tc.values[0]))
			var summary p.PluginSummary
			if err := json.Unmarshal([]byte(raw), &summary); err == nil {
				t.Error("summary accepted hidden invalid enum")
			}
			for _, carrier := range pluginEnumCarriers(t) {
				t.Run(carrier.method, func(t *testing.T) {
					mock := NewMockTransport()
					mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(raw))})
					if _, err := carrier.call(p.NewClient(mock)); err == nil {
						t.Error("public call accepted hidden invalid enum")
					}
				})
			}
		})
	}
}

func TestPluginSummaryEnumContract(t *testing.T) {
	for _, owner := range pluginEnumCases() {
		t.Run(owner.field, func(t *testing.T) {
			key := fmt.Sprintf("%q", owner.field)
			last := owner.values[len(owner.values)-1]
			final := fmt.Sprintf(`%s:%q`, key, last)
			cases := []struct {
				name, members, want string
				valid               bool
			}{
				{"duplicates", fmt.Sprintf(`%s:%q,%s`, key, owner.values[0], final), last, true},
				{"escaped", fmt.Sprintf(`"\u%04x%s":"future",%s`, owner.field[0], owner.field[1:], final), "", false},
				{"valid folded", fmt.Sprintf(`%q:%q,%s`, strings.ToUpper(owner.field), owner.values[0], final), last, true},
				{"folded", fmt.Sprintf(`%q:"future",%s`, strings.ToUpper(owner.field), final), "", false},
				{"invalid final", fmt.Sprintf(`%s,%s:"future"`, final, key), "", false},
				{"empty earlier", fmt.Sprintf(`%s:"",%s`, key, final), "", false},
				{"type earlier", fmt.Sprintf(`%s:1,%s`, key, final), "", false},
				{"null earlier", fmt.Sprintf(`%s:null,%s`, key, final), last, owner.nullable},
				{"null final", fmt.Sprintf(`%s,%s:null`, final, key), "", owner.nullable},
				{"nested extra", fmt.Sprintf(`"extra":{%s:"future"},%s`, key, final), last, true},
				{"long invalid", fmt.Sprintf(`%s:%q,%s`, key, strings.Repeat("x", 16000), final), "", false},
			}
			for _, v := range owner.values {
				cases = append(cases, struct {
					name, members, want string
					valid               bool
				}{v, fmt.Sprintf(`%s:%q`, key, v), v, true})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					raw := pluginEnumSummary(t, owner.field, tc.members)
					for _, carrier := range pluginEnumCarriers(t) {
						t.Run(carrier.method, func(t *testing.T) {
							mock := NewMockTransport()
							mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(raw))})
							client := p.NewClient(mock)
							result, err := carrier.call(client)
							if (err == nil) != tc.valid {
								t.Fatalf("error=%v; valid=%v", err, tc.valid)
							}
							if tc.valid {
								checkPluginEnum(t, result, owner.field, tc.want)
							} else {
								if !reflect.DeepEqual(result, p.PluginSummary{}) {
									t.Fatal("failed call published summary")
								}
								if len(err.Error()) > 4096 {
									t.Fatal("unbounded diagnostic")
								}
								recovery := pluginEnumSummary(t, owner.field, final)
								mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(recovery))})
								result, err = carrier.call(client)
								if err != nil {
									t.Fatal(err)
								}
								checkPluginEnum(t, result, owner.field, last)
							}
						})
					}
				})
			}
		})
	}
}

func checkPluginEnum(t *testing.T, s p.PluginSummary, field, want string) {
	t.Helper()
	got := ""
	switch field {
	case "authPolicy":
		got = string(s.AuthPolicy)
	case "installPolicy":
		got = string(s.InstallPolicy)
	case "availability":
		got = string(s.Availability)
	}
	if got != want {
		t.Fatalf("%s=%q; want %q", field, got, want)
	}
}

func TestPluginSummaryEnumReceiverAndErrors(t *testing.T) {
	for _, owner := range pluginEnumCases() {
		t.Run(owner.field, func(t *testing.T) {
			valid := fmt.Sprintf(`%q:%q`, owner.field, owner.values[0])
			seed := pluginEnumSummary(t, owner.field, valid+`,"keywords":["retained"],"version":"old"`)
			var summary p.PluginSummary
			if err := json.Unmarshal([]byte(seed), &summary); err != nil {
				t.Fatal(err)
			}
			before := summary
			// The final semantic error has always retained the decoded field updates.
			invalidFinal := pluginEnumSummary(t, owner.field, fmt.Sprintf(`%q:"earlier",%q:"later"`, owner.field, owner.field))
			err := json.Unmarshal([]byte(invalidFinal), &summary)
			if err == nil || !strings.Contains(err.Error(), `"later"`) || strings.Contains(err.Error(), `"earlier"`) {
				t.Fatalf("final error changed: %v", err)
			}
			checkPluginEnum(t, summary, owner.field, "later")
			if summary.Version != nil || summary.Keywords != nil {
				t.Fatal("old partial update semantics changed")
			}
			summary = before
			hidden := pluginEnumSummary(t, owner.field, fmt.Sprintf(`%q:"future",%s`, owner.field, valid))
			if err := json.Unmarshal([]byte(hidden), &summary); err == nil {
				t.Fatal("hidden enum accepted")
			}
			if !reflect.DeepEqual(summary, before) || summary.Version != before.Version || &summary.Keywords[0] != &before.Keywords[0] {
				t.Fatal("new rejection changed retained receiver references")
			}
			if err := json.Unmarshal([]byte(seed), &summary); err != nil {
				t.Fatalf("receiver recovery: %v", err)
			}
			// Existing missing-field, native type and array errors win over hidden enums.
			for _, tc := range []struct{ name, raw, want string }{
				{"missing", strings.Replace(hidden, `"enabled":true,`, "", 1), "missing plugin.summary.enabled"},
				{"type", strings.Replace(hidden, `"enabled":true`, `"enabled":"yes"`, 1), "cannot unmarshal string"},
				{"keywords", strings.TrimSuffix(hidden, "}") + `,"keywords":[1]}`, "cannot unmarshal number"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					err := json.Unmarshal([]byte(tc.raw), &summary)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("existing error replaced: %v", err)
					}
				})
			}
		})
	}
}

func TestPluginSummaryEnumOuterReplacement(t *testing.T) {
	for _, owner := range pluginEnumCases() {
		invalid := pluginEnumSummary(t, owner.field, fmt.Sprintf(`%q:"future",%q:%q`, owner.field, owner.field, owner.values[0]))
		for _, carrier := range pluginEnumCarriers(t) {
			t.Run(owner.field+"/"+carrier.method, func(t *testing.T) {
				// A rejected summary must not be hidden by replacing the outer response field.
				raw := carrier.wrap(invalid)
				valid := pluginEnumSummary(t, owner.field, fmt.Sprintf(`%q:%q`, owner.field, owner.values[0]))
				replacement := carrier.wrap(valid)
				raw = strings.TrimSuffix(raw, "}") + "," + replacement[1:]
				mock := NewMockTransport()
				mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(raw)})
				if _, err := carrier.call(p.NewClient(mock)); err == nil {
					t.Fatal("outer replacement hid invalid summary")
				}
			})
		}
	}
}

func TestPluginSummaryEnumEnvelope(t *testing.T) {
	for _, owner := range pluginEnumCases() {
		t.Run(owner.field, func(t *testing.T) {
			valid := fmt.Sprintf(`%q:%q`, owner.field, owner.values[0])
			seed := pluginEnumSummary(t, owner.field, valid+`,"keywords":["retained"]`)
			var envelope struct {
				Summary p.PluginSummary `json:"summary"`
				After   string          `json:"after"`
			}
			if err := json.Unmarshal([]byte(`{"summary":`+seed+`}`), &envelope); err != nil {
				t.Fatal(err)
			}
			before := envelope.Summary
			hidden := pluginEnumSummary(t, owner.field, fmt.Sprintf(`%q:"future",%s`, owner.field, valid))
			err := json.Unmarshal([]byte(`{"summary":`+hidden+`,"after":"visited"}`), &envelope)
			if err == nil {
				t.Fatal("envelope accepted hidden enum")
			}
			if !reflect.DeepEqual(envelope.Summary, before) || &envelope.Summary.Keywords[0] != &before.Keywords[0] {
				t.Fatal("new rejection changed the summary or retained references")
			}
			if envelope.After != "visited" {
				t.Fatal("default backend did not continue through the enclosing record")
			}
		})
	}
}

func TestPluginEnumScalarCompatibility(t *testing.T) {
	var auth p.PluginAuthPolicy
	var install p.PluginInstallPolicy
	var availability p.PluginAvailability
	for _, value := range []any{&auth, &install, &availability} {
		if err := json.Unmarshal([]byte(`"future"`), value); err != nil {
			t.Fatalf("ordinary scalar decoding changed: %v", err)
		}
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != `"future"` {
			t.Fatalf("ordinary scalar encoding changed: %s, %v", encoded, err)
		}
	}
}
