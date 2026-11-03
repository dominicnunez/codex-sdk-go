package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestStructuredRequirementsAdmission(t *testing.T) {
	cases := []struct {
		Field, Name string
		Raw         *string
		Valid       bool
	}{
		{"allowedPermissionProfiles", "absent", nil, true},
		{"allowedPermissionProfiles", "null", requirementRaw(`null`), true},
		{"allowedPermissionProfiles", "valid-0", requirementRaw(`{}`), true},
		{"allowedPermissionProfiles", "valid-1", requirementRaw(`{"": false, "workspace": true}`), true},
		{"allowedPermissionProfiles", "invalid-0", requirementRaw(`true`), false},
		{"allowedPermissionProfiles", "invalid-1", requirementRaw(`[]`), false},
		{"allowedPermissionProfiles", "invalid-2", requirementRaw(`"text"`), false},
		{"allowedPermissionProfiles", "invalid-3", requirementRaw(`17`), false},
		{"allowedPermissionProfiles", "invalid-4", requirementRaw(`{"a": null}`), false},
		{"allowedPermissionProfiles", "invalid-5", requirementRaw(`{"a": "true"}`), false},
		{"allowedPermissionProfiles", "invalid-6", requirementRaw(`{"a": {}}`), false},
		{"allowedPermissionProfiles", "hidden-invalid", requirementRaw(`true,"allowedPermissionProfiles":{}`), false},
		{"autoReview", "absent", nil, true},
		{"autoReview", "null", requirementRaw(`null`), true},
		{"autoReview", "valid-0", requirementRaw(`{}`), true},
		{"autoReview", "valid-1", requirementRaw(`{"ignoreRules": ["", "rule"], "requiredOnModels": ["model"]}`), true},
		{"autoReview", "valid-2", requirementRaw(`{"ignoreRules": null, "requiredOnModels": null, "future": 17}`), true},
		{"autoReview", "invalid-0", requirementRaw(`true`), false},
		{"autoReview", "invalid-1", requirementRaw(`[]`), false},
		{"autoReview", "invalid-2", requirementRaw(`"text"`), false},
		{"autoReview", "invalid-3", requirementRaw(`17`), false},
		{"autoReview", "invalid-4", requirementRaw(`{"ignoreRules": true}`), false},
		{"autoReview", "invalid-5", requirementRaw(`{"ignoreRules": [null]}`), false},
		{"autoReview", "invalid-6", requirementRaw(`{"requiredOnModels": [17]}`), false},
		{"autoReview", "hidden-invalid", requirementRaw(`true,"autoReview":{}`), false},
		{"browserUse", "absent", nil, true},
		{"browserUse", "null", requirementRaw(`null`), true},
		{"browserUse", "valid-0", requirementRaw(`{}`), true},
		{"browserUse", "valid-1", requirementRaw(`{"allowGlobalPersistentApproval": false, "allowHistoryAccess": true, "allowWebmcp": false, "disableAutoReview": true, "defaultOriginPolicy": {"access": "allow", "accessApprovalLifetime": "thread", "persistentApproval": false}, "origins": {"https://example.invalid": {"uploads": "deny"}}, "extension": {"requestHeaders": [{"name": "", "value": ""}]}}`), true},
		{"browserUse", "invalid-0", requirementRaw(`true`), false},
		{"browserUse", "invalid-1", requirementRaw(`[]`), false},
		{"browserUse", "invalid-2", requirementRaw(`"text"`), false},
		{"browserUse", "invalid-3", requirementRaw(`17`), false},
		{"browserUse", "invalid-4", requirementRaw(`{"allowHistoryAccess": "true"}`), false},
		{"browserUse", "invalid-5", requirementRaw(`{"origins": {"site": null}}`), false},
		{"browserUse", "invalid-6", requirementRaw(`{"origins": {"site": {"access": "bogus"}}}`), false},
		{"browserUse", "invalid-7", requirementRaw(`{"defaultOriginPolicy": {"accessApprovalLifetime": "bogus"}}`), false},
		{"browserUse", "invalid-8", requirementRaw(`{"extension": {"requestHeaders": [{"name": "x"}]}}`), false},
		{"browserUse", "invalid-9", requirementRaw(`{"extension": {"requestHeaders": [null]}}`), false},
		{"browserUse", "hidden-invalid", requirementRaw(`true,"browserUse":{}`), false},
		{"feedback", "absent", nil, true},
		{"feedback", "null", requirementRaw(`null`), true},
		{"feedback", "valid-0", requirementRaw(`{}`), true},
		{"feedback", "valid-1", requirementRaw(`{"enabled": false}`), true},
		{"feedback", "valid-2", requirementRaw(`{"enabled": null, "future": []}`), true},
		{"feedback", "invalid-0", requirementRaw(`true`), false},
		{"feedback", "invalid-1", requirementRaw(`[]`), false},
		{"feedback", "invalid-2", requirementRaw(`"text"`), false},
		{"feedback", "invalid-3", requirementRaw(`17`), false},
		{"feedback", "invalid-4", requirementRaw(`{"enabled": "true"}`), false},
		{"feedback", "invalid-5", requirementRaw(`{"enabled": []}`), false},
		{"feedback", "hidden-invalid", requirementRaw(`true,"feedback":{}`), false},
		{"models", "absent", nil, true},
		{"models", "null", requirementRaw(`null`), true},
		{"models", "valid-0", requirementRaw(`{}`), true},
		{"models", "valid-1", requirementRaw(`{"newThread": {"model": "", "modelReasoningEffort": "custom", "serviceTier": "priority"}}`), true},
		{"models", "valid-2", requirementRaw(`{"newThread": null, "future": false}`), true},
		{"models", "invalid-0", requirementRaw(`true`), false},
		{"models", "invalid-1", requirementRaw(`[]`), false},
		{"models", "invalid-2", requirementRaw(`"text"`), false},
		{"models", "invalid-3", requirementRaw(`17`), false},
		{"models", "invalid-4", requirementRaw(`{"newThread": true}`), false},
		{"models", "invalid-5", requirementRaw(`{"newThread": {"model": 17}}`), false},
		{"models", "invalid-6", requirementRaw(`{"newThread": {"modelReasoningEffort": ""}}`), false},
		{"models", "hidden-invalid", requirementRaw(`true,"models":{}`), false},
		{"modelProviders", "absent", nil, true},
		{"modelProviders", "null", requirementRaw(`null`), true},
		{"modelProviders", "valid-0", requirementRaw(`{}`), true},
		{"modelProviders", "valid-1", requirementRaw(`{"arbitrary": [null, false, 1, "x", {}]}`), true},
		{"modelProviders", "invalid-0", requirementRaw(`true`), false},
		{"modelProviders", "invalid-1", requirementRaw(`[]`), false},
		{"modelProviders", "invalid-2", requirementRaw(`"text"`), false},
		{"modelProviders", "invalid-3", requirementRaw(`17`), false},
		{"modelProviders", "hidden-invalid", requirementRaw(`true,"modelProviders":{}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.Field+"/"+tc.Name, func(t *testing.T) {
			body := "{}"
			if tc.Raw != nil {
				body = `{"` + tc.Field + `":` + *tc.Raw + `}`
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":` + body + `}`)})
			got, err := client.Config.ReadRequirements(context.Background())
			if !tc.Valid {
				if err == nil {
					t.Fatalf("malformed requirement published: %s", body)
				}
				if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
					t.Fatalf("partial response: %+v", got)
				}
				mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{}}`)})
				if _, err = client.Config.ReadRequirements(context.Background()); err != nil {
					t.Fatalf("recovery: %v", err)
				}
				return
			}
			if err != nil || got.Requirements == nil {
				t.Fatalf("valid control failed: %v %+v", err, got)
			}
			encoded, err := json.Marshal(got.Requirements)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]json.RawMessage
			if err = json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			value, present := actual[tc.Field]
			if tc.Raw == nil {
				if present {
					t.Fatal("absent field materialized")
				}
				return
			}
			var want, have any
			if err = json.Unmarshal([]byte(*tc.Raw), &want); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(value, &have); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, have) {
				t.Fatalf("valid value changed: %s -> %s", *tc.Raw, value)
			}
		})
	}
}

func requirementRaw(s string) *string { return &s }

// Each case comes from a schema property, not a decoder helper. Templates also
// exercise arrays and maps at their containing admission boundary.
func TestStructuredRequirementsNestedContracts(t *testing.T) {
	tests := []struct {
		name, template, member string
		valid, invalid         []string
	}{
		{"permission value", `{"allowedPermissionProfiles":{"":%s}}`, "", []string{`false`, `true`}, []string{`null`, `1`, `[]`, `"false"`}},
		{"ignore rules", `{"autoReview":{"ignoreRules":%s}}`, "ignoreRules", []string{`null`, `[]`, `["","rule"]`}, []string{`true`, `{}`, `[null]`, `[1]`}},
		{"required models", `{"autoReview":{"requiredOnModels":%s}}`, "requiredOnModels", []string{`null`, `[]`, `[""]`}, []string{`0`, `[false]`}},
		{"global approval", `{"browserUse":{"allowGlobalPersistentApproval":%s}}`, "allowGlobalPersistentApproval", []string{`null`, `false`, `true`}, []string{`"true"`, `0`, `{}`}},
		{"history", `{"browserUse":{"allowHistoryAccess":%s}}`, "allowHistoryAccess", []string{`null`, `false`}, []string{`"false"`}},
		{"webmcp", `{"browserUse":{"allowWebmcp":%s}}`, "allowWebmcp", []string{`null`, `false`}, []string{`[]`}},
		{"disable review", `{"browserUse":{"disableAutoReview":%s}}`, "disableAutoReview", []string{`null`, `false`}, []string{`0`}},
		{"default policy", `{"browserUse":{"defaultOriginPolicy":%s}}`, "defaultOriginPolicy", []string{`null`, `{}`}, []string{`false`, `[]`}},
		{"origin map", `{"browserUse":{"origins":%s}}`, "origins", []string{`null`, `{}`, `{"not a URL":{}}`}, []string{`[]`, `{"site":null}`, `{"site":1}`}},
		{"access", `{"browserUse":{"origins":{"site":{"access":%s}}}}`, "access", []string{`null`, `"allow"`, `"deny"`}, []string{`true`, `"ALLOW"`, `""`}},
		{"lifetime", `{"browserUse":{"defaultOriginPolicy":{"accessApprovalLifetime":%s}}}`, "accessApprovalLifetime", []string{`null`, `"turn"`, `"thread"`}, []string{`0`, `"forever"`}},
		{"auto review", `{"browserUse":{"defaultOriginPolicy":{"autoReview":%s}}}`, "autoReview", []string{`null`, `"allow"`, `"deny"`}, []string{`"other"`}},
		{"downloads", `{"browserUse":{"defaultOriginPolicy":{"downloads":%s}}}`, "downloads", []string{`null`, `"allow"`, `"deny"`}, []string{`"other"`}},
		{"cdp", `{"browserUse":{"defaultOriginPolicy":{"fullCdpAccess":%s}}}`, "fullCdpAccess", []string{`null`, `"allow"`, `"deny"`}, []string{`"other"`}},
		{"uploads", `{"browserUse":{"defaultOriginPolicy":{"uploads":%s}}}`, "uploads", []string{`null`, `"allow"`, `"deny"`}, []string{`"other"`}},
		{"persistent approval", `{"browserUse":{"defaultOriginPolicy":{"persistentApproval":%s}}}`, "persistentApproval", []string{`null`, `false`, `true`}, []string{`"true"`}},
		{"extension", `{"browserUse":{"extension":%s}}`, "extension", []string{`null`, `{}`}, []string{`true`, `[]`}},
		{"headers", `{"browserUse":{"extension":{"requestHeaders":%s}}}`, "requestHeaders", []string{`null`, `[]`, `[{"name":"","value":""}]`}, []string{`{}`, `[null]`, `[{}]`, `[{"NAME":"x","value":"x"}]`, `[{"name":"x"}]`}},
		{"header name", `{"browserUse":{"extension":{"requestHeaders":[{"value":"","name":%s}]}}}`, "name", []string{`""`, `"x"`}, []string{`null`, `0`}},
		{"header value", `{"browserUse":{"extension":{"requestHeaders":[{"name":"","value":%s}]}}}`, "value", []string{`""`, `"x"`}, []string{`null`, `false`}},
		{"feedback", `{"feedback":{"enabled":%s}}`, "enabled", []string{`null`, `false`, `true`}, []string{`"true"`, `[]`}},
		{"thread defaults", `{"models":{"newThread":%s}}`, "newThread", []string{`null`, `{}`}, []string{`false`, `[]`}},
		{"model", `{"models":{"newThread":{"model":%s}}}`, "model", []string{`null`, `""`, `"future"`}, []string{`false`, `[]`}},
		{"effort", `{"models":{"newThread":{"modelReasoningEffort":%s}}}`, "modelReasoningEffort", []string{`null`, `" "`, `"custom"`, `"\u0078"`}, []string{`""`, `0`}},
		{"service tier", `{"models":{"newThread":{"serviceTier":%s}}}`, "serviceTier", []string{`null`, `""`, `"future"`}, []string{`false`, `{}`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, raw := range tc.valid {
				t.Run("valid/"+raw, func(t *testing.T) { checkRequirementAdmission(t, fmt.Sprintf(tc.template, raw), true) })
			}
			for _, raw := range tc.invalid {
				t.Run("invalid/"+raw, func(t *testing.T) {
					checkRequirementAdmission(t, fmt.Sprintf(tc.template, raw), false)
					// A valid later duplicate at this exact nested member cannot erase failure.
					duplicate := raw + `,` + strconv.Quote(tc.member) + `:` + tc.valid[0]
					checkRequirementAdmission(t, fmt.Sprintf(tc.template, duplicate), false)
					// Escaped spellings are the same schema property.
					if tc.member != "" {
						escaped := `"\u` + fmt.Sprintf("%04x", tc.member[0]) + tc.member[1:] + `"`
						checkRequirementAdmission(t, strings.ReplaceAll(fmt.Sprintf(tc.template, raw), strconv.Quote(tc.member)+`:`, escaped+`:`), false)
					}
				})
			}
		})
	}
}

func checkRequirementAdmission(t *testing.T, requirements string, valid bool) {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":` + requirements + `}`)})
	got, err := client.Config.ReadRequirements(context.Background())
	if (err == nil) != valid {
		t.Fatalf("valid=%v: %s: %v", valid, requirements, err)
	}
	if !valid && !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
		t.Fatal("partial result published")
	}
}

func TestStructuredRequirementsMatchingAndRetention(t *testing.T) {
	// Native matching applies only to the two existing outer Go records. Raw
	// objects are schema-shaped, with exact property names and open extensions.
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"REQUIREMENTS":{"AUTOReview":true}}`, false},
		{`{"requirements":{"MODELſ":[]}}`, false},
		{`{"requir\u0065ments":{"feedb\u0061ck":true}}`, false},
		{`{"requirements":{"feedback":true},"REQUIREMENTS":null}`, false},
		{`{"requirements":{"models":true},"requirements":{"models":{}}}`, false},
		{`{"requirements":{"browserUse":{"origins":{"site":{"access":"bad"},"site":{}}}}}`, false},
		{`{"requirements":{"feedback":{"enabled":0}},"requirements":{"feedback":null}}`, false},
		{`{"requirements":{"feedback":{"Enabled":0,"enabled":false,"ENABLED":{}}}}`, true},
		{`{"requirements":{"autoReview":{"IgnoreRules":false,"ignoreRules":[],"REQUIREDONMODELS":{}}}}`, true},
		{`{"requirements":{"browserUse":{"AllowWebmcp":[],"defaultOriginPolicy":{"ACCESS":false,"access":"allow","Access":[]},"origins":{"not a URL":{"Uploads":1}},"extension":{"RequestHeaders":0,"requestHeaders":[{"NAME":true,"name":"","value":"","VALUE":{}}]}}}}`, true},
		{`{"requirements":{"models":{"NEWTHREAD":false,"newThread":{"MODEL":{},"model":"","ModelReasoningEffort":false,"SERVICEtier":[]}}}}`, true},
		{`{"requirements":{"modelProviders":{"":null,"x":false,"x":[1],"feedback":{"enabled":[]}}}}`, true},
		{`{"requirements":{"feedback":{"enabled":false}},"requirements":null}`, true},
		{`{"requirements":{"models":{"newThread":{"modelReasoningEffort":"future"}}},"requirements":{"models":{}}}`, true},
		{`{"requirements":{"modelProviders":{"huge":1e10000},"feedback":{"future":1e10000},"browserUse":{"origins":{"":{"future":1e10000}}}}}`, true},
		{`{"requirements":null}`, true},
		{`{}`, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(tc.raw)})
			got, err := client.Config.ReadRequirements(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
			if !tc.valid {
				if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
					t.Fatal("partial response")
				}
				return
			}
			var native p.ConfigRequirementsReadResponse
			if err = json.Unmarshal([]byte(tc.raw), &native); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, native) {
				t.Fatalf("changed native selection or raw bytes: %+v vs %+v", got, native)
			}
		})
	}
	// Retained raw bytes include whitespace, escapes, duplicate map keys and
	// unknown extensions; publication must own them independently of transport.
	const value = `{ "newThread" : { "modelReasoningEffort" : "\u0078" }, "future": [1, null], "future": false }`
	body := json.RawMessage(`{"requirements":{"models":` + value + `}}`)
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("configRequirements/read", p.Response{Result: body})
	got, err := client.Config.ReadRequirements(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range body {
		body[i] = ' '
	}
	if string(got.Requirements.Models) != value {
		t.Fatalf("raw bytes changed or borrowed: %s", got.Requirements.Models)
	}
}

func TestStructuredRequirementsExistingErrors(t *testing.T) {
	for _, raw := range []string{
		`{"requirements":{"models":true,"allowedSandboxModes":["invalid"]}}`,
		`{"requirements":{"feedback":true,"allowLoginShell":"false"}}`,
		`{"requirements":{"autoReview":true},"supportsIndependentSpeedModes":[]}`,
		`{"requirements":{"browserUse":true,"featureRequirements":[]}}`,
		`{"requirements":[]}`,
		`{"requirements":{"modelProviders":true,"otel":{"exporter":"invalid"}}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var native p.ConfigRequirementsReadResponse
			want := json.Unmarshal([]byte(raw), &native)
			if want == nil {
				t.Fatal("control did not exercise existing error")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(raw)})
			got, err := client.Config.ReadRequirements(context.Background())
			if err == nil || err.Error() != `unmarshal response result for configRequirements/read: `+want.Error() {
				t.Fatalf("precedence changed: %v; want %v", err, want)
			}
			var wantType, gotType *json.UnmarshalTypeError
			if errors.As(want, &wantType) && (!errors.As(err, &gotType) || !reflect.DeepEqual(wantType, gotType)) {
				t.Fatalf("native type context changed: %#v vs %#v", gotType, wantType)
			}
			if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
				t.Fatal("partial response")
			}
		})
	}
}

func TestStructuredRequirementsPlainRecords(t *testing.T) {
	for _, value := range []any{p.ConfigRequirements{}, &p.ConfigRequirements{}, p.ConfigRequirementsReadResponse{}, &p.ConfigRequirementsReadResponse{}} {
		if _, ok := value.(json.Unmarshaler); ok {
			t.Fatalf("public record acquired UnmarshalJSON: %T", value)
		}
		if _, ok := value.(json.Marshaler); ok {
			t.Fatalf("public record acquired MarshalJSON: %T", value)
		}
	}
	type envelope struct {
		p.ConfigRequirements
		Extra string `json:"extra"`
	}
	got := envelope{Extra: "prior"}
	if err := json.Unmarshal([]byte(`{"models":true,"extra":"kept"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Extra != "kept" || string(got.Models) != "true" {
		t.Fatalf("ordinary JSON composition changed: %+v", got)
	}
	type responseEnvelope struct {
		p.ConfigRequirementsReadResponse
		Extra string `json:"extra"`
	}
	var response responseEnvelope
	if err := json.Unmarshal([]byte(`{"requirements":{"feedback":false},"extra":"kept"}`), &response); err != nil {
		t.Fatal(err)
	}
	if response.Extra != "kept" || response.Requirements == nil || string(response.Requirements.Feedback) != "false" {
		t.Fatalf("response embedding changed: %+v", response)
	}
}

func FuzzStructuredRequirementsOccurrences(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {1}, {2, 0}, {3, 1}, {0, 1, 0, 1}, {0, 2, 1}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, sequence []byte) {
		if len(sequence) > 64 {
			sequence = sequence[:64]
		}
		values := []string{`false`, `true`, `null`, `"true"`}
		key, err := json.Marshal(string(sequence))
		if err != nil {
			t.Fatal(err)
		}
		var members []string
		valid := true
		for _, step := range sequence {
			choice := int(step) % len(values)
			valid = valid && choice < 2
			members = append(members, string(key)+`:`+values[choice])
		}
		raw := `{` + strings.Join(members, ",") + `}`
		// Model: every boolean-map occurrence must be a boolean. Later duplicates
		// cannot repair an invalid earlier occurrence.
		// The identical values are unconstrained in provider maps and open extras.
		checkRequirementAdmission(t, `{"allowedPermissionProfiles":`+raw+`,"modelProviders":`+raw+`,"feedback":{"future":`+raw+`}}`, valid)
		checkRequirementAdmission(t, `{"modelProviders":`+raw+`,"feedback":{"future":`+raw+`}}`, true)
	})
}

func TestStructuredRequirementsBoundedDiagnostics(t *testing.T) {
	key := strings.Repeat("peer-", 20000)
	encoded, _ := json.Marshal(key)
	for _, raw := range []string{
		`{"allowedPermissionProfiles":{` + string(encoded) + `:null}}`,
		`{"browserUse":{"origins":{` + string(encoded) + `:{"access":` + string(encoded) + `}}}}`,
	} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":` + raw + `}`)})
		got, err := client.Config.ReadRequirements(context.Background())
		if err == nil {
			t.Fatal("published invalid requirements")
		}
		if len(err.Error()) > 1024 || strings.Contains(err.Error(), key) {
			t.Fatalf("unbounded peer-value diagnostic: %d bytes", len(err.Error()))
		}
		if !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
			t.Fatal("partial response")
		}
	}
}
