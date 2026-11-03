package protocol_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginShareVisibilitySchemaValues(t *testing.T) {
	for _, path := range []string{"schema/json/ClientRequest.json", "schema/json/codex_app_server_protocol.v2.schemas.json", "schema/json/v2/PluginShareUpdateTargetsParams.json"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Definitions map[string]struct {
				Enum []string `json:"enum"`
			}
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		values := schema.Definitions["PluginShareUpdateDiscoverability"].Enum
		if len(values) != 3 {
			t.Fatal("unexpected current enum schema", values)
		}
		for _, value := range values {
			t.Run(path+"/"+value, func(t *testing.T) {
				encoded, _ := json.Marshal(value)
				var decoded p.PluginShareUpdateDiscoverability
				if err := json.Unmarshal(encoded, &decoded); err != nil || string(decoded) != value {
					t.Errorf("schema value rejected by decoder: %v", err)
				}
				typed := p.PluginShareUpdateDiscoverability(value)
				got, err := json.Marshal(typed)
				if err != nil || string(got) != string(encoded) {
					t.Errorf("schema value rejected by encoder: %v", err)
				}
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				response := map[string]any{"discoverability": value, "principals": []any{map[string]any{"name": "Reader", "principalId": "user-1", "principalType": "user", "role": "reader"}}}
				if err := mock.SetResponseData("plugin/share/updateTargets", response); err != nil {
					t.Fatal(err)
				}
				params := p.PluginShareUpdateTargetsParams{Discoverability: typed, RemotePluginID: "plugin", ShareTargets: []p.PluginShareTarget{{PrincipalID: "user-1", PrincipalType: p.PluginSharePrincipalTypeUser, Role: p.PluginShareTargetRoleReader}}}
				actual, err := c.Plugin.ShareUpdateTargets(context.Background(), params)
				if err != nil {
					t.Fatal("schema value rejected before dispatch:", err)
				}
				if string(actual.Discoverability) != value || len(actual.Principals) != 1 || actual.Principals[0].PrincipalID != "user-1" {
					t.Fatal("response metadata changed")
				}
				if len(mock.SentRequests) != 1 {
					t.Fatal("not dispatched once")
				}
				var captured p.PluginShareUpdateTargetsParams
				if err := json.Unmarshal(mock.GetSentRequest(0).Params, &captured); err != nil || !reflect.DeepEqual(captured, params) {
					t.Fatal("request identity, target, or visibility changed")
				}
			})
		}
	}
}

func TestPluginShareListedTargetValidation(t *testing.T) {
	if p.PluginShareUpdateDiscoverabilityListed != "LISTED" {
		t.Fatal("listed constant value changed")
	}
	for _, tc := range []struct {
		name, field string
		edit        func(*p.PluginShareUpdateTargetsParams)
	}{
		{"unknown visibility", "discoverability", func(v *p.PluginShareUpdateTargetsParams) { v.Discoverability = "bogus" }},
		{"empty visibility", "discoverability", func(v *p.PluginShareUpdateTargetsParams) { v.Discoverability = "" }},
		{"remote identity", "remotePluginId", func(v *p.PluginShareUpdateTargetsParams) { v.RemotePluginID = "" }},
		{"null targets", "shareTargets", func(v *p.PluginShareUpdateTargetsParams) { v.ShareTargets = nil }},
		{"principal identity", "principalId", func(v *p.PluginShareUpdateTargetsParams) { v.ShareTargets[0].PrincipalID = "" }},
		{"principal type", "principalType", func(v *p.PluginShareUpdateTargetsParams) { v.ShareTargets[0].PrincipalType = "bogus" }},
		{"target role", "role", func(v *p.PluginShareUpdateTargetsParams) { v.ShareTargets[0].Role = "owner" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			params := p.PluginShareUpdateTargetsParams{Discoverability: p.PluginShareUpdateDiscoverabilityListed, RemotePluginID: "plugin", ShareTargets: []p.PluginShareTarget{{PrincipalID: "user-1", PrincipalType: p.PluginSharePrincipalTypeUser, Role: p.PluginShareTargetRoleReader}}}
			tc.edit(&params)
			result, err := c.Plugin.ShareUpdateTargets(context.Background(), params)
			if err == nil || !strings.Contains(err.Error(), tc.field) || len(mock.SentRequests) != 0 || !reflect.DeepEqual(result, p.PluginShareUpdateTargetsResponse{}) {
				t.Fatalf("invalid share target dispatched or wrong check: %v requests=%d", err, len(mock.SentRequests))
			}
		})
	}
	for _, value := range []p.PluginShareUpdateDiscoverability{"bogus", "", "listed"} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatal("invalid visibility encoded")
		}
		before := p.PluginShareUpdateDiscoverabilityPrivate
		raw, _ := json.Marshal(string(value))
		if err := json.Unmarshal(raw, &before); err == nil || before != p.PluginShareUpdateDiscoverabilityPrivate {
			t.Fatal("invalid visibility decoded or changed receiver")
		}
	}
}
