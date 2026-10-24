package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginListPreservesMarketplaceFilterPresence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []p.PluginListMarketplaceKind
		want  string
	}{
		{"omitted", nil, ""},
		{"empty", []p.PluginListMarketplaceKind{}, "[]"},
		{"populated", []p.PluginListMarketplaceKind{p.PluginListMarketplaceKindVertical}, `["vertical"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := p.PluginListParams{MarketplaceKinds: tc.kinds, ForceRefetch: true}
			for _, encode := range []struct {
				name string
				call func() ([]byte, error)
			}{
				{"direct JSON", func() ([]byte, error) { return json.Marshal(params) }},
				{"public method", func() ([]byte, error) {
					mock := NewMockTransport()
					if err := mock.SetResponseData("plugin/list", map[string]any{"marketplaces": []any{}}); err != nil {
						return nil, err
					}
					if _, err := p.NewClient(mock).Plugin.List(context.Background(), params); err != nil {
						return nil, err
					}
					return mock.GetSentRequest(0).Params, nil
				}},
			} {
				t.Run(encode.name, func(t *testing.T) {
					wire, err := encode.call()
					if err != nil {
						t.Fatal(err)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(wire, &fields); err != nil {
						t.Fatal(err)
					}
					if string(fields["marketplaceKinds"]) != tc.want {
						t.Fatalf("filter presence changed: got %s, want member %q", wire, tc.want)
					}
					if string(fields["forceRefetch"]) != "true" {
						t.Fatalf("sibling field lost: %s", wire)
					}
				})
			}
		})
	}
}
