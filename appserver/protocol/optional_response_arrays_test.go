package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

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
