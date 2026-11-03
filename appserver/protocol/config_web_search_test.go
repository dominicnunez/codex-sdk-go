package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func readWebSearchConfig(t *testing.T, config string) (p.ConfigReadResponse, error) {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("config/read", p.Response{Result: json.RawMessage(`{"config":` + config + `,"origins":{}}`)})
	return client.Config.Read(context.Background(), p.ConfigReadParams{})
}

func TestStructuredWebSearchConfig(t *testing.T) {
	for _, tc := range []struct{ name, object, want string }{
		{"empty", `{}`, `{}`},
		{"context", `{"context_size":"low"}`, `{"context_size":"low"}`},
		{"empty domains", `{"allowed_domains":[]}`, `{"allowed_domains":[]}`},
		{"domains", `{"allowed_domains":["example.com",""]}`, `{"allowed_domains":["example.com",""]}`},
		{"location", `{"location":{"city":"","country":"US","region":"CA","timezone":"UTC"}}`, `{"location":{"city":"","country":"US","region":"CA","timezone":"UTC"}}`},
		{"all null", `{"allowed_domains":null,"context_size":null,"location":null}`, `{}`},
		{"mixed", `{"allowed_domains":["example.com"],"context_size":"high","location":{"city":"","country":null}}`, `{"allowed_domains":["example.com"],"context_size":"high","location":{"city":""}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var tools p.ToolsV2
			if err := json.Unmarshal([]byte(`{"web_search":`+tc.object+`}`), &tools); err != nil {
				t.Error("direct structured tool rejected:", err)
			}
			response, err := readWebSearchConfig(t, `{"tools":{"web_search":`+tc.object+`}}`)
			if err != nil {
				t.Fatal("public structured tool rejected:", err)
			}
			if response.Config == nil || response.Config.Tools == nil {
				t.Fatal("tool metadata lost")
			}
			actual, err := json.Marshal(response.Config.Tools.WebSearch)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(actual, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("structured metadata changed: %s want %s", actual, tc.want)
			}
		})
	}
}

func TestWebSearchConfigControls(t *testing.T) {
	for _, config := range []string{`{}`, `{"tools":null}`, `{"tools":{}}`, `{"tools":{"web_search":null}}`, `{"web_search":"live"}`, `{"tools":{"view_image":true}}`, `{"tools":{"view_image":false}}`, `{"tools":{"view_image":null}}`, `{"tools":{"future":42,"web_search":{}}}`} {
		t.Run(config, func(t *testing.T) {
			response, err := readWebSearchConfig(t, config)
			if err != nil {
				t.Fatal(err)
			}
			if config == `{"web_search":"live"}` && (response.Config.WebSearch == nil || string(*response.Config.WebSearch) != "live") {
				t.Fatal("root web-search mode changed")
			}
			if config == `{"tools":{"view_image":true}}` || config == `{"tools":{"view_image":false}}` {
				want := config == `{"tools":{"view_image":true}}`
				if response.Config.Tools == nil || response.Config.Tools.ViewImage == nil || *response.Config.Tools.ViewImage != want {
					t.Fatal("legacy view-image flag changed")
				}
			}
		})
	}
}
