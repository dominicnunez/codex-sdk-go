package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type optionalTextInput struct{ value codex.UserInput }

func (v *optionalTextInput) UnmarshalJSON(data []byte) error {
	value, err := codex.UnmarshalUserInput(data)
	if err == nil {
		v.value = value
	}
	return err
}

func optionalArrayValue(t *testing.T, receiver any, name string) reflect.Value {
	t.Helper()
	switch value := receiver.(type) {
	case *codex.ThreadItemWrapper:
		receiver = value.Value
	case *codex.SandboxPolicyWrapper:
		receiver = value.Value
	case *optionalTextInput:
		receiver = value.value
	}
	value := reflect.ValueOf(receiver)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	for i := 0; i < value.NumField(); i++ {
		if strings.Split(value.Type().Field(i).Tag.Get("json"), ",")[0] == name {
			return value.Field(i)
		}
	}
	t.Fatalf("missing array %s in %T", name, receiver)
	return reflect.Value{}
}

func TestOptionalPluginResponseArrays(t *testing.T) {
	for _, tc := range []struct {
		method, field, populated string
	}{
		{"plugin/list", "featuredPluginIds", `["plugin-id",""]`},
		{"plugin/list", "marketplaceLoadErrors", `[{"marketplacePath":"/tmp/plugins","message":"unavailable"}]`},
		{"plugin/installed", "marketplaceLoadErrors", `[{"marketplacePath":"/tmp/plugins","message":"unavailable"}]`},
	} {
		t.Run(tc.method+"/"+tc.field, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			call := func(body string) (any, error) {
				transport.SetResponse(tc.method, codex.Response{JSONRPC: "2.0", Result: json.RawMessage(body)})
				if tc.method == "plugin/list" {
					return client.Plugin.List(context.Background(), codex.PluginListParams{})
				}
				return client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
			}
			for _, value := range []string{"", "[]", tc.populated} {
				body := `{"marketplaces":[]}`
				if value != "" {
					body = `{"marketplaces":[],"` + tc.field + `":` + value + `}`
				}
				result, err := call(body)
				if err != nil {
					t.Fatalf("valid %s: %v", body, err)
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				// Omitted and empty default arrays retain the existing omitempty
				// representation. Populated arrays must preserve their elements.
				if value == tc.populated && string(fields[tc.field]) != value {
					t.Fatalf("array = %s, want %s", fields[tc.field], value)
				}
			}
			invalid := []string{"null", `true`, `{}`, `"text"`, `[null]`, `[42]`}
			if tc.field == "marketplaceLoadErrors" {
				invalid = append(invalid, `[{}]`, `[{"message":"missing path"}]`, `[{"marketplacePath":null,"message":"bad path"}]`, `[{"marketplacePath":"/tmp/plugins","message":null}]`)
			}
			for _, value := range invalid {
				for _, suffix := range []string{"", `,"` + tc.field + `":[]`} {
					body := `{"marketplaces":[],"` + tc.field + `":` + value + suffix + `}`
					if _, err := call(body); err == nil {
						t.Errorf("accepted malformed occurrence %s", body)
					}
				}
			}
			var receiver any = &codex.PluginListResponse{}
			if tc.method == "plugin/installed" {
				receiver = &codex.PluginInstalledResponse{}
			}
			populated := `{"marketplaces":[],"` + tc.field + `":` + tc.populated + `}`
			if err := json.Unmarshal([]byte(populated), receiver); err != nil {
				t.Fatal(err)
			}
			before := reflect.ValueOf(receiver).Elem().Interface()
			if err := json.Unmarshal([]byte(`{"marketplaces":[],"`+tc.field+`":null}`), receiver); err == nil {
				t.Fatal("reused receiver accepted null")
			}
			if !reflect.DeepEqual(before, reflect.ValueOf(receiver).Elem().Interface()) {
				t.Fatal("failed admission changed prior response")
			}
			if err := json.Unmarshal([]byte(`{"marketplaces":[]}`), receiver); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(receiver)
			if err != nil || strings.Contains(string(encoded), tc.field) {
				t.Fatalf("omitted field did not clear prior response: %s, %v", encoded, err)
			}
		})
	}
}

func TestOptionalArraySiblingOwners(t *testing.T) {
	lifecycle, err := json.Marshal(validThreadLifecycleResponse(validProcessThreadPayload("array-thread")))
	if err != nil {
		t.Fatal(err)
	}
	const model = `{"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]}`
	for _, tc := range []struct {
		name, base, field, populated string
		newReceiver                  func() any
	}{
		{"app", `{"id":"a","name":"A"}`, "pluginDisplayNames", `["", "Plugin"]`, func() any { return &codex.AppInfo{} }},
		{"plugin", issue74PluginSummary, "keywords", `["", "keyword"]`, func() any { return &codex.PluginSummary{} }},
		{"model", model, "inputModalities", `["text","image"]`, func() any { return &codex.Model{} }},
		{"start-plugins", string(lifecycle), "disabledPluginIds", `["", "plugin"]`, func() any { return &codex.ThreadStartResponse{} }},
		{"start-instructions", string(lifecycle), "instructionSources", `["", "source"]`, func() any { return &codex.ThreadStartResponse{} }},
		{"resume-plugins", string(lifecycle), "disabledPluginIds", `["", "plugin"]`, func() any { return &codex.ThreadResumeResponse{} }},
		{"resume-instructions", string(lifecycle), "instructionSources", `["", "source"]`, func() any { return &codex.ThreadResumeResponse{} }},
		{"fork-plugins", string(lifecycle), "disabledPluginIds", `["", "plugin"]`, func() any { return &codex.ThreadForkResponse{} }},
		{"fork-instructions", string(lifecycle), "instructionSources", `["", "source"]`, func() any { return &codex.ThreadForkResponse{} }},
		{"reasoning-summary", `{"type":"reasoning","id":"r"}`, "summary", `["", "summary"]`, func() any { return &codex.ThreadItemWrapper{} }},
		{"reasoning-content", `{"type":"reasoning","id":"r"}`, "content", `["", "content"]`, func() any { return &codex.ThreadItemWrapper{} }},
		{"reasoning-direct", `{"id":"r"}`, "summary", `["", "summary"]`, func() any { return &codex.ReasoningThreadItem{} }},
		{"text", `{"type":"text","text":"hello"}`, "text_elements", `[{"byteRange":{"start":0,"end":1}}]`, func() any { return &optionalTextInput{} }},
		{"text-direct", `{"text":"hello"}`, "text_elements", `[{"byteRange":{"start":0,"end":1}}]`, func() any { return &codex.TextUserInput{} }},
		{"config-workspace", `{}`, "writable_roots", `["", "relative"]`, func() any { return &codex.SandboxWorkspaceWrite{} }},
		{"workspace-policy", `{"type":"workspaceWrite"}`, "writableRoots", `["/tmp/project"]`, func() any { return &codex.SandboxPolicyWrapper{} }},
		{"workspace-direct", `{}`, "writableRoots", `["/tmp/project"]`, func() any { return &codex.SandboxPolicyWorkspaceWrite{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := func(value, extra string) string {
				return strings.TrimSuffix(tc.base, "}") + `,"` + tc.field + `":` + value + extra + `}`
			}
			// Empty objects need no comma before their first member.
			if tc.base == "{}" {
				body = func(value, extra string) string { return `{"` + tc.field + `":` + value + extra + `}` }
			}
			for _, valid := range []string{tc.base, body("[]", ""), body(tc.populated, ""), body("[]", `,"`+tc.field+`":`+tc.populated)} {
				if err := json.Unmarshal([]byte(valid), tc.newReceiver()); err != nil {
					t.Errorf("valid %s: %v", valid, err)
				}
			}
			for _, value := range []string{"null", "true", `{}`, `"text"`, `[null]`, `[42]`} {
				for _, extra := range []string{"", `,"` + tc.field + `":[]`} {
					if err := json.Unmarshal([]byte(body(value, extra)), tc.newReceiver()); err == nil {
						t.Errorf("accepted invalid array %s", body(value, extra))
					}
				}
			}
			for _, key := range []string{strings.ToUpper(tc.field), fmt.Sprintf(`\u%04x%s`, tc.field[0], tc.field[1:])} {
				aliased := strings.Replace(body("null", ""), `"`+tc.field+`":`, `"`+key+`":`, 1)
				if err := json.Unmarshal([]byte(aliased), tc.newReceiver()); err == nil {
					t.Errorf("alias bypassed null guard: %s", aliased)
				}
			}
			receiver := tc.newReceiver()
			if err := json.Unmarshal([]byte(body(tc.populated, "")), receiver); err != nil {
				t.Fatal(err)
			}
			before := optionalArrayValue(t, receiver, tc.field).Interface()
			if err := json.Unmarshal([]byte(body("null", "")), receiver); err == nil {
				t.Fatal("reused receiver accepted null")
			}
			if !reflect.DeepEqual(before, optionalArrayValue(t, receiver, tc.field).Interface()) {
				t.Fatal("forbidden array changed prior storage")
			}
			if err := json.Unmarshal([]byte(tc.base), receiver); err != nil {
				t.Fatal(err)
			}
			retains := tc.name == "reasoning-direct" || tc.name == "text-direct" || tc.name == "config-workspace" || tc.name == "workspace-direct"
			array := optionalArrayValue(t, receiver, tc.field)
			if retains && !reflect.DeepEqual(before, array.Interface()) {
				t.Fatal("direct stdlib omission no longer retains prior array")
			}
			if !retains && array.Len() != 0 {
				t.Fatal("fresh owner omission retained prior array")
			}
			if err := json.Unmarshal([]byte(body("[]", "")), receiver); err != nil {
				t.Fatal(err)
			}
			array = optionalArrayValue(t, receiver, tc.field)
			if array.IsNil() || array.Len() != 0 {
				t.Fatal("explicit empty array lost its decoded shape")
			}
		})
	}
}
