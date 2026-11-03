package protocol_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestWebSearchConfigRejectsInvalid(t *testing.T) {
	for _, tc := range []struct{ name, config string }{
		{"boolean", `{"tools":{"web_search":true}}`},
		{"number", `{"tools":{"web_search":42}}`},
		{"array", `{"tools":{"web_search":[]}}`},
		{"context enum", `{"tools":{"web_search":{"context_size":"tiny"}}}`},
		{"empty context", `{"tools":{"web_search":{"context_size":""}}}`},
		{"context kind", `{"tools":{"web_search":{"context_size":42}}}`},
		{"domain null element", `{"tools":{"web_search":{"allowed_domains":[null]}}}`},
		{"domain kind", `{"tools":{"web_search":{"allowed_domains":[42]}}}`},
		{"domains kind", `{"tools":{"web_search":{"allowed_domains":42}}}`},
		{"search extra", `{"tools":{"web_search":{"future":{}}}}`},
		{"search case extra", `{"tools":{"web_search":{"ALLOWED_DOMAINS":[]}}}`},
		{"location extra", `{"tools":{"web_search":{"location":{"future":{}}}}}`},
		{"location case extra", `{"tools":{"web_search":{"location":{"CITY":"x"}}}}`},
		{"location kind", `{"tools":{"web_search":{"location":true}}}`},
		{"location city kind", `{"tools":{"web_search":{"location":{"city":42}}}}`},
		{"hidden domains", `{"tools":{"web_search":{"allowed_domains":[null],"allowed_domains":[]}}}`},
		{"hidden domains null reset", `{"tools":{"web_search":{"allowed_domains":[null],"allowed_domains":null}}}`},
		{"hidden context", `{"tools":{"web_search":{"context_size":"tiny","context_size":"low"}}}`},
		{"hidden location", `{"tools":{"web_search":{"location":{"future":{}},"location":null}}}`},
		{"hidden search", `{"tools":{"web_search":{"future":{}},"web_search":null}}`},
		{"hidden tools", `{"tools":{"web_search":{"allowed_domains":[null]}},"tools":null}`},
		{"escaped array", `{"to\u006fls":{"web_se\u0061rch":{"allowed_dom\u0061ins":[null]}}}`},
		{"outer aliases", `{"TOOLS":{"WEB_SEARCH":{"allowed_domains":[null]}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := readWebSearchConfig(t, tc.config)
			if err == nil || !reflect.DeepEqual(response, p.ConfigReadResponse{}) {
				t.Fatalf("invalid tool config published: %+v %v", response, err)
			}
			if !strings.Contains(err.Error(), "web_search") && !strings.Contains(err.Error(), "WEB_SEARCH") {
				var native *json.UnmarshalTypeError
				if !errors.As(err, &native) {
					t.Fatalf("field context or native cause lost: %v", err)
				}
			}
		})
	}
}

func TestWebSearchConfigPriorErrors(t *testing.T) {
	bad := `{"future":null}`
	control := `{` + strings.Repeat(" ", len(bad)-2) + `}`
	for _, tail := range []string{
		`,"model":42`,
		`,"web_search":"invalid-mode"`,
		`,"sandbox_workspace_write":{"writable_roots":[null]}`,
		`,"sandbox_workspace_write":{"network_access":null}`,
		`,"browser_use":{"origins":{"invalid":null}}`,
	} {
		t.Run(tail, func(t *testing.T) {
			decode := func(search string) error {
				var c p.Config
				return json.Unmarshal([]byte(`{"tools":{"web_search":`+search+`}`+tail+`}`), &c)
			}
			expected, actual := decode(control), decode(bad)
			if expected == nil || actual == nil || !reflect.DeepEqual(expected, actual) {
				t.Fatalf("existing error priority changed: %v vs %v", expected, actual)
			}
		})
	}
}

func TestWebSearchConfigReceiverAndSelection(t *testing.T) {
	makeConfig := func() *p.Config {
		domains := []string{"old", "retained"}
		return &p.Config{Model: p.Ptr("old"), Tools: &p.ToolsV2{ViewImage: p.Ptr(true), WebSearch: &p.WebSearchToolConfig{AllowedDomains: &domains, ContextSize: p.Ptr(p.WebSearchContextSizeLow), Location: &p.WebSearchLocation{City: p.Ptr("old")}}}}
	}
	for _, search := range []string{`{"allowed_domains":[null],"location":{"city":"new"}}`, `{"future":null,"location":{"city":"new"}}`, `{"location":{"future":null,"city":"new"}}`} {
		t.Run(search, func(t *testing.T) {
			actual := makeConfig()
			tools, web := actual.Tools, actual.Tools.WebSearch
			location, domains, city := web.Location, web.AllowedDomains, web.Location.City
			type native p.Config
			expected := native(*makeConfig())
			raw := []byte(`{"tools":{"web_search":` + search + `},"model":"updated"}`)
			if err := json.Unmarshal(raw, &expected); err != nil {
				t.Fatal("plain reference should decode:", err)
			}
			if err := actual.UnmarshalJSON(raw); err == nil {
				t.Fatal("invalid structured config admitted")
			}
			if !reflect.DeepEqual(*actual, p.Config(expected)) || actual.Tools != tools || actual.Tools.WebSearch != web || web.Location != location || web.AllowedDomains != domains || web.Location.City != city {
				t.Fatal("native partial updates or retained references changed")
			}
		})
	}
	for _, config := range []string{
		`{"TOOLS":{"WEB_SEARCH":{"allowed_domains":[],"location":{"city":""}}}}`,
		`{"tools":{"web_search":{"allowed_domains":["old"]}},"tools":{"web_search":{"allowed_domains":[],"context_size":"high"}}}`,
		`{"tools":{"web_search":{"location":{"city":"old"}},"web_search":null}}`,
		`{"tools":{"web_search":{"location":{"city":"old"}}},"tools":null}`,
	} {
		t.Run(config, func(t *testing.T) {
			var actual p.Config
			type native p.Config
			var expected native
			if err := json.Unmarshal([]byte(config), &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(config), &actual); err != nil || !reflect.DeepEqual(actual, p.Config(expected)) {
				t.Fatalf("native selection changed: %v", err)
			}
		})
	}
}

func TestWebSearchPlainRecordsAndDiagnostics(t *testing.T) {
	for _, v := range []any{p.ToolsV2{}, &p.ToolsV2{}, p.WebSearchToolConfig{}, &p.WebSearchToolConfig{}, p.WebSearchLocation{}, &p.WebSearchLocation{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatalf("plain record gained decoder: %T", v)
		}
		if _, ok := v.(json.Marshaler); ok {
			t.Fatalf("plain record gained encoder: %T", v)
		}
	}
	var envelope struct {
		p.WebSearchLocation
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"city":"","Extra":"kept"}`), &envelope); err != nil || envelope.City == nil || *envelope.City != "" || envelope.Extra != "kept" {
		t.Fatal("plain anonymous record composition changed")
	}
	var named struct {
		Config p.Config
		Extra  string
	}
	if err := json.Unmarshal([]byte(`{"Config":{"tools":{"web_search":{"future":null}},"model":"kept"},"Extra":"visited"}`), &named); err == nil || named.Extra != "visited" || named.Config.Model == nil || *named.Config.Model != "kept" {
		t.Fatal("Config wrapper error traversal changed")
	}
	for _, object := range []string{`{"` + strings.Repeat("peer-", 20000) + `":null}`, `{"context_size":"` + strings.Repeat("peer-", 20000) + `"}`} {
		_, err := readWebSearchConfig(t, `{"tools":{"web_search":`+object+`}}`)
		if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "web_search") {
			t.Fatal("unbounded or missing diagnostic")
		}
	}
	for _, value := range []p.WebSearchContextSize{p.WebSearchContextSizeLow, p.WebSearchContextSizeMedium, p.WebSearchContextSizeHigh} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var actual p.WebSearchContextSize
		if err := json.Unmarshal(raw, &actual); err != nil || actual != value {
			t.Fatal("context enum roundtrip changed")
		}
	}
	if _, err := json.Marshal(p.WebSearchContextSize("future")); err == nil {
		t.Fatal("invalid constructed context encoded")
	}
	var c p.Config
	err := json.Unmarshal([]byte(`{"TOOLS":{"WEB_SEARCH":{"location":{"city":42}}}}`), &c)
	var native *json.UnmarshalTypeError
	if !errors.As(err, &native) || !strings.Contains(native.Field, "city") || native.Type.Kind() != reflect.String {
		t.Fatalf("native field/kind cause lost: %v", err)
	}
}

func TestWebSearchSchemaFieldKinds(t *testing.T) {
	bindings := map[string]reflect.Type{"ToolsV2": reflect.TypeFor[p.ToolsV2](), "WebSearchToolConfig": reflect.TypeFor[p.WebSearchToolConfig](), "WebSearchLocation": reflect.TypeFor[p.WebSearchLocation](), "WebSearchContextSize": reflect.TypeFor[p.WebSearchContextSize]()}
	for _, path := range []string{"schema/json/v2/ConfigReadResponse.json", "schema/json/codex_app_server_protocol.v2.schemas.json"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Definitions map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for name, record := range bindings {
			if record.Kind() != reflect.Struct {
				continue
			}
			definition, ok := schema.Definitions[name]
			if !ok {
				t.Fatal("missing schema record", name)
			}
			for wire, node := range definition.Properties {
				var property struct {
					Type  json.RawMessage `json:"type"`
					AnyOf []struct {
						Ref  string `json:"$ref"`
						Type string `json:"type"`
					} `json:"anyOf"`
					Items struct {
						Type string `json:"type"`
					} `json:"items"`
				}
				if err := json.Unmarshal(node, &property); err != nil {
					t.Fatal(err)
				}
				var field reflect.Type
				for index := range record.NumField() {
					candidate := record.Field(index)
					if strings.Split(candidate.Tag.Get("json"), ",")[0] == wire {
						field = candidate.Type
						break
					}
				}
				if field == nil || field.Kind() != reflect.Pointer {
					t.Fatalf("%s.%s lost nullable field", name, wire)
				}
				field = field.Elem()
				if len(property.AnyOf) > 0 {
					for _, branch := range property.AnyOf {
						if branch.Ref != "" {
							target := strings.TrimPrefix(branch.Ref, "#/definitions/")
							if field != bindings[target] {
								t.Fatalf("%s.%s scalar/record mismatch: %v for %s", name, wire, field, target)
							}
						}
					}
					continue
				}
				var types []string
				if err := json.Unmarshal(property.Type, &types); err != nil {
					t.Fatal(err)
				}
				if len(types) != 2 || types[1] != "null" {
					t.Fatal("unexpected nullable schema shape", name, wire)
				}
				switch types[0] {
				case "string":
					if field.Kind() != reflect.String {
						t.Fatal("string field kind mismatch")
					}
				case "array":
					if field.Kind() != reflect.Slice || field.Elem().Kind() != reflect.String || property.Items.Type != "string" {
						t.Fatal("array field kind mismatch")
					}
				default:
					t.Fatal("unhandled schema field kind", types)
				}
			}
		}
	}
}

func FuzzWebSearchConfig(f *testing.F) {
	for _, seed := range []struct {
		text string
		mode uint8
	}{{"", 0}, {"example.com", 1}, {"city", 2}, {"é\x00", 3}, {string([]byte{0xff}), 6}, {"", 7}} {
		f.Add(seed.text, seed.mode)
	}
	f.Fuzz(func(t *testing.T, text string, mode uint8) {
		if len(text) > 1024 {
			text = text[:1024]
		}
		search := map[string]any{}
		expected := map[string]any{}
		switch mode % 4 {
		case 0:
			search["future"] = nil
		case 1:
			search["allowed_domains"] = []any{text, nil}
		case 2:
			size := []string{"low", "medium", "high"}[int(mode)%3]
			search["allowed_domains"] = []string{text}
			search["context_size"] = size
			search["location"] = map[string]any{"city": text, "country": "", "timezone": nil}
			expected["allowed_domains"] = []string{text}
			expected["context_size"] = size
			expected["location"] = map[string]any{"city": text, "country": ""}
		case 3:
			search["allowed_domains"] = []string{}
			search["context_size"] = nil
			search["location"] = map[string]any{"city": nil}
			expected["allowed_domains"] = []string{}
			expected["location"] = map[string]any{}
		}
		config, err := json.Marshal(map[string]any{"tools": map[string]any{"web_search": search}})
		if err != nil {
			t.Fatal(err)
		}
		response, err := readWebSearchConfig(t, string(config))
		if mode%4 < 2 {
			if err == nil || !reflect.DeepEqual(response, p.ConfigReadResponse{}) {
				t.Fatal("invalid generated config published")
			}
			return
		}
		if err != nil || response.Config == nil || response.Config.Tools == nil {
			t.Fatalf("valid generated config rejected: %v", err)
		}
		got, err := json.Marshal(response.Config.Tools.WebSearch)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		var gotValue, wantValue any
		if err := json.Unmarshal(got, &gotValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(want, &wantValue); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Fatalf("generated structured metadata changed: %s want %s", got, want)
		}
	})
}
