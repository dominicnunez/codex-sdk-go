package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginListAcceptsVerticalMarketplace(t *testing.T) {
	mock := NewMockTransport()
	if err := mock.SetResponseData("plugin/list", map[string]any{"marketplaces": []any{}}); err != nil {
		t.Fatal(err)
	}
	_, err := p.NewClient(mock).Plugin.List(context.Background(), p.PluginListParams{MarketplaceKinds: []p.PluginListMarketplaceKind{p.PluginListMarketplaceKindVertical}})
	if err != nil {
		t.Fatalf("schema-valid vertical filter rejected: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["marketplaceKinds"]) != `["vertical"]` {
		t.Fatalf("vertical selector changed: %s", fields["marketplaceKinds"])
	}
}

func TestPluginListAcceptsEverySchemaMarketplaceKind(t *testing.T) {
	wire, err := os.ReadFile("schema/json/v2/PluginListParams.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions map[string]struct {
			Enum []string `json:"enum"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(wire, &schema); err != nil {
		t.Fatal(err)
	}
	values := schema.Definitions["PluginListMarketplaceKind"].Enum
	if len(values) == 0 {
		t.Fatal("schema enum missing")
	}
	filters := [][]string{nil, values, {"vertical", "local", "vertical"}}
	for _, value := range values {
		filters = append(filters, []string{value})
	}
	for _, names := range filters {
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData("plugin/list", map[string]any{"marketplaces": []any{}}); err != nil {
				t.Fatal(err)
			}
			var kinds []p.PluginListMarketplaceKind
			if names != nil {
				kinds = make([]p.PluginListMarketplaceKind, len(names))
			}
			for i, name := range names {
				kinds[i] = p.PluginListMarketplaceKind(name)
			}
			before := append([]p.PluginListMarketplaceKind(nil), kinds...)
			_, err := p.NewClient(mock).Plugin.List(context.Background(), p.PluginListParams{MarketplaceKinds: kinds})
			if err != nil {
				t.Fatalf("schema filter rejected: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
				t.Fatal(err)
			}
			if len(names) == 0 {
				if _, ok := fields["marketplaceKinds"]; ok {
					t.Fatal("omitted filter became populated")
				}
			} else {
				var got []string
				if err := json.Unmarshal(fields["marketplaceKinds"], &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, names) {
					t.Fatalf("filter changed: got=%v want=%v", got, names)
				}
			}
			if !reflect.DeepEqual(kinds, before) {
				t.Fatal("request preparation mutated caller filters")
			}
		})
	}
}

func TestPluginListRejectsInvalidSelectorsBeforeSend(t *testing.T) {
	for _, params := range []p.PluginListParams{
		{MarketplaceKinds: []p.PluginListMarketplaceKind{"future"}},
		{MarketplaceKinds: []p.PluginListMarketplaceKind{"local", "future"}},
		{MarketplaceKinds: []p.PluginListMarketplaceKind{p.PluginListMarketplaceKindVertical, ""}},
		{MarketplaceKinds: []p.PluginListMarketplaceKind{p.PluginListMarketplaceKindVertical}, Cwds: []string{"\x00"}},
	} {
		mock := NewMockTransport()
		if _, err := p.NewClient(mock).Plugin.List(context.Background(), params); err == nil {
			t.Fatal("invalid selector accepted")
		}
		if mock.GetSentRequest(0) != nil {
			t.Fatal("invalid filter reached transport")
		}
	}
}
