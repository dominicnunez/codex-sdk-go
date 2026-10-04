package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpCatalogIconsRequireArrayOrNull(t *testing.T) {
	wrongRoots := []struct {
		name  string
		value any
	}{
		{name: "string", value: "glyph"},
		{name: "object", value: map[string]any{"src": "glyph"}},
		{name: "number", value: 7},
		{name: "boolean", value: false},
	}

	for _, target := range []string{"resource", "tool"} {
		for _, root := range wrongRoots {
			t.Run(target+"/"+root.name, func(t *testing.T) {
				item := map[string]any{"name": "item", "icons": root.value}
				var directErr error
				if target == "resource" {
					item["uri"] = "mcp://item"
					var got codex.Resource
					directErr = json.Unmarshal(mustJSON(t, item), &got)
				} else {
					item["inputSchema"] = map[string]any{}
					var got codex.Tool
					directErr = json.Unmarshal(mustJSON(t, item), &got)
				}
				if directErr == nil {
					t.Errorf("direct decode accepted schema-invalid %s icons root", root.name)
				}

				server := map[string]any{
					"authStatus":        "notLoggedIn",
					"name":              "synthetic",
					"resourceTemplates": []any{},
					"resources":         []any{},
					"tools":             map[string]any{},
				}
				if target == "resource" {
					server["resources"] = []any{item}
				} else {
					server["tools"] = map[string]any{"item": item}
				}
				mock := NewMockTransport()
				client := codex.NewClient(mock)
				if err := mock.SetResponseData("mcpServerStatus/list", map[string]any{"data": []any{server}}); err != nil {
					t.Fatalf("SetResponseData: %v", err)
				}
				response, err := client.Mcp.ListServerStatus(context.Background(), codex.ListMcpServerStatusParams{})
				if err == nil {
					t.Errorf("ListServerStatus published schema-invalid %s icons root", root.name)
				}
				if len(response.Data) != 0 {
					t.Errorf("ListServerStatus returned %d data entries on icon validation error", len(response.Data))
				}
			})
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test fixture: %v", err)
	}
	return encoded
}

func TestMcpCatalogIconsAcceptArrayItemsWithoutAdditionalRestrictions(t *testing.T) {
	wireItems := []any{
		nil,
		map[string]any{"src": "nested", "metadata": []any{true, "value"}},
		[]any{},
		"glyph",
		float64(3),
		false,
	}

	for _, target := range []string{"resource", "tool"} {
		t.Run(target, func(t *testing.T) {
			item := map[string]any{"name": "item", "icons": wireItems}
			if target == "resource" {
				item["uri"] = "mcp://item"
				var got codex.Resource
				if err := json.Unmarshal(mustJSON(t, item), &got); err != nil {
					t.Fatalf("decode schema-valid arbitrary icon items: %v", err)
				}
				if !reflect.DeepEqual(got.Icons, wireItems) {
					t.Fatalf("Resource.Icons = %#v, want %#v", got.Icons, wireItems)
				}
				assertCatalogIconsListResponse(t, "resource", item, wireItems)
			} else {
				item["inputSchema"] = map[string]any{}
				var got codex.Tool
				if err := json.Unmarshal(mustJSON(t, item), &got); err != nil {
					t.Fatalf("decode schema-valid arbitrary icon items: %v", err)
				}
				if !reflect.DeepEqual(got.Icons, wireItems) {
					t.Fatalf("Tool.Icons = %#v, want %#v", got.Icons, wireItems)
				}
				assertCatalogIconsListResponse(t, "tool", item, wireItems)
			}
		})
	}
}

func assertCatalogIconsListResponse(t *testing.T, target string, item map[string]any, want []any) {
	t.Helper()
	server := map[string]any{
		"authStatus":        "notLoggedIn",
		"name":              "synthetic",
		"resourceTemplates": []any{},
		"resources":         []any{},
		"tools":             map[string]any{},
	}
	if target == "resource" {
		server["resources"] = []any{item}
	} else {
		server["tools"] = map[string]any{"item": item}
	}
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	if err := mock.SetResponseData("mcpServerStatus/list", map[string]any{"data": []any{server}}); err != nil {
		t.Fatalf("SetResponseData: %v", err)
	}
	response, err := client.Mcp.ListServerStatus(context.Background(), codex.ListMcpServerStatusParams{})
	if err != nil {
		t.Fatalf("ListServerStatus: %v", err)
	}
	var got any
	if target == "resource" {
		got = response.Data[0].Resources[0].Icons
	} else {
		got = response.Data[0].Tools["item"].Icons
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListServerStatus %s icons = %#v, want %#v", target, got, want)
	}
}

func TestMcpCatalogIconsValidateEveryFoldedOccurrence(t *testing.T) {
	for _, target := range []string{"resource", "tool"} {
		for _, key := range []string{`ICONS`, `ic\u006Fns`, `icon\u017F`} {
			for _, tc := range []struct{ name, values string }{
				{name: "invalid-single", values: `"bad"`},
				{name: "valid-then-invalid", values: `[],"icons":"bad"`},
				{name: "invalid-then-valid", values: `"bad","icons":[]`},
			} {
				t.Run(target+"/"+key+"/"+tc.name, func(t *testing.T) {
					body := catalogItemPrefix(target) + `,"` + key + `":` + tc.values + `}`
					var err error
					if target == "resource" {
						var got codex.Resource
						err = json.Unmarshal([]byte(body), &got)
					} else {
						var got codex.Tool
						err = json.Unmarshal([]byte(body), &got)
					}
					if err == nil || !strings.Contains(err.Error(), "icons") {
						t.Fatalf("decode %s alias with invalid occurrence error = %v, want icons validation error", key, err)
					}
				})
			}
		}
		// Dotless-i is a non-alias under encoding/json's field folding rules.
		t.Run(target+"/dotless-i-is-not-an-alias", func(t *testing.T) {
			body := catalogItemPrefix(target) + `,"\u0131cons":"ignored"}`
			if target == "resource" {
				var got codex.Resource
				if err := json.Unmarshal([]byte(body), &got); err != nil || got.Icons != nil {
					t.Fatalf("dotless-i key result = (%#v, %v), want ignored key", got.Icons, err)
				}
			} else {
				var got codex.Tool
				if err := json.Unmarshal([]byte(body), &got); err != nil || got.Icons != nil {
					t.Fatalf("dotless-i key result = (%#v, %v), want ignored key", got.Icons, err)
				}
			}
		})
	}
}

func TestMcpCatalogIconsPreserveValidDuplicateResetSemantics(t *testing.T) {
	cases := []struct {
		name string
		body string
		want any
	}{
		{name: "valid-duplicate-last-array", body: `,"icons":["first"],"ICONS":["last"]`, want: []any{"last"}},
		{name: "valid-then-null-clears", body: `,"icons":["old"],"ICONS":null`, want: nil},
		{name: "null-then-empty-array", body: `,"icons":null,"ICONS":[]`, want: []any{}},
		{name: "array-reset-empty-array", body: `,"icons":["old"],"ICONS":[]`, want: []any{}},
	}
	for _, target := range []string{"resource", "tool"} {
		for _, tc := range cases {
			t.Run(target+"/"+tc.name, func(t *testing.T) {
				body := catalogItemPrefix(target) + tc.body + `}`
				if target == "resource" {
					var got codex.Resource
					if err := json.Unmarshal([]byte(body), &got); err != nil {
						t.Fatalf("decode: %v", err)
					}
					if !reflect.DeepEqual(got.Icons, tc.want) {
						t.Fatalf("Resource.Icons = %#v, want %#v", got.Icons, tc.want)
					}
				} else {
					var got codex.Tool
					if err := json.Unmarshal([]byte(body), &got); err != nil {
						t.Fatalf("decode: %v", err)
					}
					if !reflect.DeepEqual(got.Icons, tc.want) {
						t.Fatalf("Tool.Icons = %#v, want %#v", got.Icons, tc.want)
					}
				}
			})
		}
	}
}

func TestMcpCatalogIconsResetSeededReceiverOnSuccessfulDecode(t *testing.T) {
	for _, target := range []string{"resource", "tool"} {
		for _, tc := range []struct {
			name string
			body string
			want any
		}{
			{name: "absent", body: "", want: nil},
			{name: "null", body: `,"icons":null`, want: nil},
			{name: "empty-array", body: `,"icons":[]`, want: []any{}},
		} {
			t.Run(target+"/"+tc.name, func(t *testing.T) {
				body := catalogItemPrefix(target) + tc.body + `}`
				old := map[string]any{"held": "old"}
				if target == "resource" {
					got := codex.Resource{Icons: old}
					if err := json.Unmarshal([]byte(body), &got); err != nil {
						t.Fatalf("decode: %v", err)
					}
					if !reflect.DeepEqual(got.Icons, tc.want) || old["held"] != "old" {
						t.Fatalf("Resource reset result = (%#v, held=%#v), want (%#v, held unchanged)", got.Icons, old, tc.want)
					}
				} else {
					got := codex.Tool{Icons: old}
					if err := json.Unmarshal([]byte(body), &got); err != nil {
						t.Fatalf("decode: %v", err)
					}
					if !reflect.DeepEqual(got.Icons, tc.want) || old["held"] != "old" {
						t.Fatalf("Tool reset result = (%#v, held=%#v), want (%#v, held unchanged)", got.Icons, old, tc.want)
					}
				}
			})
		}
	}
}

func TestMcpCatalogIconsErrorsDoNotPublishSeededReceivers(t *testing.T) {
	description := "held description"
	resourceIcons := map[string]any{"held": true}
	resource := codex.Resource{Name: "seed", URI: "mcp://seed", Description: &description, Icons: resourceIcons}
	resourceBefore := resource
	if err := resource.UnmarshalJSON([]byte(`{"name":"replacement","uri":"mcp://replacement","icons":{"bad":true}}`)); err == nil {
		t.Fatal("Resource accepted object icons root")
	}
	gotResourceIcons, ok := resource.Icons.(map[string]any)
	if resource.Name != resourceBefore.Name || resource.URI != resourceBefore.URI || resource.Description != resourceBefore.Description || !ok || !reflect.DeepEqual(gotResourceIcons, map[string]any{"held": true}) || resourceIcons["held"] != true {
		t.Fatalf("Resource receiver changed on error: %#v", resource)
	}
	resourceIcons["identity-probe"] = true
	if gotResourceIcons["identity-probe"] != true {
		t.Fatal("Resource receiver no longer references its pre-call Icons map")
	}

	toolIcons := map[string]any{"held": true}
	inputSchema := map[string]any{"type": "object"}
	tool := codex.Tool{Name: "seed", InputSchema: inputSchema, Icons: toolIcons}
	if err := tool.UnmarshalJSON([]byte(`{"name":"replacement","inputSchema":{"type":"string"},"icons":false}`)); err == nil {
		t.Fatal("Tool accepted boolean icons root")
	}
	gotToolSchema, schemaOK := tool.InputSchema.(map[string]any)
	gotToolIcons, iconsOK := tool.Icons.(map[string]any)
	if tool.Name != "seed" || !schemaOK || !iconsOK || !reflect.DeepEqual(gotToolSchema, map[string]any{"type": "object"}) || !reflect.DeepEqual(gotToolIcons, map[string]any{"held": true}) || toolIcons["held"] != true || inputSchema["type"] != "object" {
		t.Fatalf("Tool receiver changed on error: %#v", tool)
	}
	toolIcons["identity-probe"] = true
	inputSchema["identity-probe"] = true
	if gotToolIcons["identity-probe"] != true || gotToolSchema["identity-probe"] != true {
		t.Fatal("Tool receiver no longer references its pre-call maps")
	}
}

func TestMcpCatalogIconsKeepExistingSyntaxAndCoreErrorsFirst(t *testing.T) {
	for _, target := range []string{"resource", "tool"} {
		cases := []struct {
			name string
			body string
			want string
		}{
			{name: "malformed-syntax", body: catalogItemPrefix(target) + `,"icons":[}`, want: "SyntaxError"},
			{name: "wrong-name-type", body: `{"name":17,"uri":"mcp://item","inputSchema":{},"icons":"bad"}`, want: "UnmarshalTypeError"},
		}
		for _, tc := range cases {
			t.Run(target+"/"+tc.name, func(t *testing.T) {
				var err error
				if target == "resource" {
					var got codex.Resource
					err = got.UnmarshalJSON([]byte(tc.body))
				} else {
					var got codex.Tool
					err = got.UnmarshalJSON([]byte(tc.body))
				}
				if err == nil {
					t.Fatal("expected existing syntax/core-type error")
				}
				var syntaxErr *json.SyntaxError
				var typeErr *json.UnmarshalTypeError
				if tc.want == "SyntaxError" && !errors.As(err, &syntaxErr) {
					t.Fatalf("error = %T %v, want syntax error", err, err)
				}
				if tc.want == "UnmarshalTypeError" && !errors.As(err, &typeErr) {
					t.Fatalf("error = %T %v, want native type error", err, err)
				}
				if strings.Contains(err.Error(), "icons:") {
					t.Fatalf("icon error preempted existing failure: %v", err)
				}
			})
		}
	}
}

func TestMcpCatalogIconsPreserveValidEnvelopeErrorsAndRejectInvalidRoots(t *testing.T) {
	type resourceEnvelope struct {
		Before   int            `json:"before"`
		Resource codex.Resource `json:"resource"`
		After    int            `json:"after"`
	}
	type toolEnvelope struct {
		Before int        `json:"before"`
		Tool   codex.Tool `json:"tool"`
		After  int        `json:"after"`
	}
	for _, target := range []string{"resource", "tool"} {
		for _, tc := range []struct {
			name       string
			before     string
			icons      string
			after      string
			wantSubstr string
		}{
			{name: "valid-icons-preserve-before-error", before: `"bad"`, icons: `[]`, after: `1`, wantSubstr: "before"},
			{name: "valid-icons-preserve-after-error", before: `1`, icons: `[]`, after: `"bad"`, wantSubstr: "after"},
			{name: "invalid-icons-preempt-before-error", before: `"bad"`, icons: `"bad"`, after: `1`, wantSubstr: "icons"},
			{name: "invalid-icons-preempt-after-error", before: `1`, icons: `"bad"`, after: `"bad"`, wantSubstr: "icons"},
		} {
			t.Run(target+"/"+tc.name, func(t *testing.T) {
				member := `"resource":` + catalogItemPrefix(target) + `,"icons":` + tc.icons + `}`
				if target == "tool" {
					member = `"tool":` + catalogItemPrefix(target) + `,"icons":` + tc.icons + `}`
				}
				body := `{"before":` + tc.before + `,` + member + `,"after":` + tc.after + `}`
				var err error
				var icons any
				if target == "resource" {
					var got resourceEnvelope
					err = json.Unmarshal([]byte(body), &got)
					icons = got.Resource.Icons
				} else {
					var got toolEnvelope
					err = json.Unmarshal([]byte(body), &got)
					icons = got.Tool.Icons
				}
				if err == nil || !strings.Contains(err.Error(), tc.wantSubstr) {
					t.Fatalf("envelope error = %v, want error containing %q", err, tc.wantSubstr)
				}
				if tc.wantSubstr == "icons" && icons != nil {
					t.Fatalf("invalid icon value was published through envelope: %#v", icons)
				}
				if tc.icons == `[]` && !reflect.DeepEqual(icons, []any{}) {
					t.Fatalf("valid icons were not retained through envelope: %#v", icons)
				}
			})
		}
	}
}

func TestMcpServerInfoIconsRemainNullableRawArrays(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "absent", body: `{"name":"server","version":"1"}`, want: true},
		{name: "null", body: `{"name":"server","version":"1","icons":null}`, want: true},
		{name: "arbitrary-array-items", body: `{"name":"server","version":"1","icons":[null,{"src":"x"},false]}`, want: true},
		{name: "string-root-rejected", body: `{"name":"server","version":"1","icons":"x"}`, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got codex.McpServerInfo
			err := json.Unmarshal([]byte(tc.body), &got)
			if (err == nil) != tc.want {
				t.Fatalf("decode error = %v, want success=%t", err, tc.want)
			}
		})
	}
}

func catalogItemPrefix(target string) string {
	if target == "resource" {
		return `{"name":"item","uri":"mcp://item"`
	}
	return `{"name":"item","inputSchema":{}`
}
