package protocol_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginAvailabilityContract(t *testing.T) {
	for _, tc := range []struct {
		name, members, want string
	}{
		{"omitted", `"extra":true`, "AVAILABLE"},
		{"available", `"availability":"AVAILABLE"`, "AVAILABLE"},
		{"disabled", `"availability":"DISABLED_BY_ADMIN"`, "DISABLED_BY_ADMIN"},
		{"folded", `"AVAILABILITY":"DISABLED_BY_ADMIN"`, "DISABLED_BY_ADMIN"},
		{"escaped", `"availab\u0069lity":"DISABLED_BY_ADMIN"`, "DISABLED_BY_ADMIN"},
		{"null", `"availability":null`, ""},
		{"null before valid", `"availability":null,"availability":"AVAILABLE"`, ""},
		{"null after valid", `"availability":"AVAILABLE","availability":null`, ""},
		{"folded null", `"AVAILABILITY":null,"availability":"AVAILABLE"`, ""},
		{"escaped null", `"availab\u0069lity":null,"availability":"AVAILABLE"`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := pluginEnumSummary(t, "availability", tc.members)
			for _, carrier := range pluginEnumCarriers(t) {
				t.Run(carrier.method, func(t *testing.T) {
					mock := NewMockTransport()
					mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(raw))})
					client := p.NewClient(mock)
					summary, err := carrier.call(client)
					if tc.want == "" {
						if err == nil {
							t.Fatal("accepted null availability")
						}
						if !reflect.DeepEqual(summary, p.PluginSummary{}) {
							t.Fatal("failed admission published a summary")
						}
						mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(pluginEnumSummary(t, "availability", `"extra":true`)))})
						summary, err = carrier.call(client)
						if err != nil {
							t.Fatalf("recovery: %v", err)
						}
						assertAvailability(t, summary, "AVAILABLE")
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					assertAvailability(t, summary, tc.want)
					encoded, err := json.Marshal(summary)
					if err != nil {
						t.Fatal(err)
					}
					var roundTrip p.PluginSummary
					if err := json.Unmarshal(encoded, &roundTrip); err != nil {
						t.Fatal(err)
					}
					assertAvailability(t, roundTrip, tc.want)
				})
			}
		})
	}
}

func assertAvailability(t *testing.T, summary p.PluginSummary, want string) {
	t.Helper()
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := json.Unmarshal(object["availability"], &got); err != nil {
		t.Fatalf("availability was not serialized as a string: %s, %v", encoded, err)
	}
	if got != want {
		t.Fatalf("availability=%q, want %q", got, want)
	}
}

func TestPluginAvailabilityReceiver(t *testing.T) {
	seed := pluginEnumSummary(t, "availability", `"availability":"DISABLED_BY_ADMIN","keywords":["retained"],"version":"old"`)
	var summary p.PluginSummary
	if err := json.Unmarshal([]byte(seed), &summary); err != nil {
		t.Fatal(err)
	}
	before := summary
	for _, members := range []string{
		`"availability":null`,
		`"availability":null,"availability":"AVAILABLE"`,
		`"availability":"AVAILABLE","availability":null`,
	} {
		raw := pluginEnumSummary(t, "availability", members)
		if err := json.Unmarshal([]byte(raw), &summary); err == nil {
			t.Fatal("accepted null availability")
		}
		if !reflect.DeepEqual(summary, before) || summary.Version != before.Version || &summary.Keywords[0] != &before.Keywords[0] {
			t.Fatal("new null rejection changed the prior receiver or its references")
		}
	}
	if err := json.Unmarshal([]byte(pluginEnumSummary(t, "availability", `"extra":true`)), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Availability != p.PluginAvailabilityAvailable {
		t.Fatalf("omission retained old availability: %s", summary.Availability)
	}
}

func TestPluginAvailabilityErrorPrecedence(t *testing.T) {
	raw := pluginEnumSummary(t, "availability", `"availability":null,"availability":"future"`)
	var summary p.PluginSummary
	err := json.Unmarshal([]byte(raw), &summary)
	if err == nil || !strings.Contains(err.Error(), `"future"`) {
		t.Fatalf("final enum error lost precedence: %v", err)
	}
	if summary.Availability != "future" {
		t.Fatal("established final-error partial updates changed")
	}
	for _, tc := range []struct{ name, raw, want string }{
		{"missing", strings.Replace(raw, `"enabled":true,`, "", 1), "missing plugin.summary.enabled"},
		{"native", strings.Replace(raw, `"enabled":true`, `"enabled":1`, 1), "cannot unmarshal number"},
		{"keywords", strings.TrimSuffix(strings.Replace(raw, `"future"`, `"AVAILABLE"`, 1), "}") + `,"keywords":[null]}`, "must not be null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tc.raw), &summary)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "availability must not be null") {
				t.Fatalf("established error lost precedence: %v", err)
			}
		})
	}
}

func TestPluginAvailabilityEnvelope(t *testing.T) {
	var envelope struct {
		Summary p.PluginSummary `json:"summary"`
		After   string          `json:"after"`
	}
	seed := pluginEnumSummary(t, "availability", `"availability":"DISABLED_BY_ADMIN"`)
	if err := json.Unmarshal([]byte(`{"summary":`+seed+`}`), &envelope); err != nil {
		t.Fatal(err)
	}
	before := envelope.Summary
	raw := pluginEnumSummary(t, "availability", `"availability":null`)
	if err := json.Unmarshal([]byte(`{"summary":`+raw+`,"after":"visited"}`), &envelope); err == nil {
		t.Fatal("envelope accepted null")
	}
	if !reflect.DeepEqual(envelope.Summary, before) || envelope.After != "visited" {
		t.Fatal("summary restoration or enclosing traversal changed")
	}
}

func TestPluginAvailabilityConstructed(t *testing.T) {
	var summary p.PluginSummary
	if err := json.Unmarshal([]byte(pluginEnumSummary(t, "availability", `"extra":true`)), &summary); err != nil {
		t.Fatal(err)
	}
	// A zero optional field omits the member, selecting the schema default.
	summary.Availability = ""
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Availability != p.PluginAvailabilityAvailable {
		t.Fatal("constructed zero did not select the omitted-field default")
	}
}

func TestPluginAvailabilityOuterReset(t *testing.T) {
	null := pluginEnumSummary(t, "availability", `"availability":null`)
	valid := pluginEnumSummary(t, "availability", `"extra":true`)
	for _, carrier := range pluginEnumCarriers(t) {
		t.Run(carrier.method, func(t *testing.T) {
			mock := NewMockTransport()
			first, replacement := carrier.wrap(null), carrier.wrap(valid)
			raw := strings.TrimSuffix(first, "}") + "," + replacement[1:]
			mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(raw)})
			if _, err := carrier.call(p.NewClient(mock)); err == nil {
				t.Fatal("outer replacement hid null availability")
			}
		})
	}
}
