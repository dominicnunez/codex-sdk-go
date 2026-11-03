package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRequiredNullThreadCarriers(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "thread/read", "thread/list", "thread/list-json", "thread/rollback", "thread/unarchive", "thread/metadata/update", "thread/revert"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			updates := 0
			remove := client.AddThreadStateListener("closed-thread", func(p.Thread) { updates++ }, nil)
			defer remove()
			for _, invalid := range []bool{true, false} {
				thread, err := json.Marshal(validProcessThreadPayload("closed-thread"))
				if err != nil {
					t.Fatal(err)
				}
				if invalid {
					thread = []byte(hiddenNullBody(string(thread), "ephemeral", true))
				}
				body := map[string]any{"thread": json.RawMessage(thread)}
				wireMethod := method
				switch method {
				case "thread/start", "thread/resume", "thread/fork":
					body = validThreadLifecycleResponse(validProcessThreadPayload("closed-thread"))
					body["thread"] = json.RawMessage(thread)
				case "thread/list", "thread/list-json":
					wireMethod = "thread/list"
					body = map[string]any{"data": []any{validProcessThreadPayload("other-good"), json.RawMessage(thread)}}
				}
				if err := mock.SetResponseData(wireMethod, body); err != nil {
					t.Fatal(err)
				}
				result, err := callClosedSourceCarrier(client, method)
				if invalid {
					if !errors.Is(err, p.ErrNullResultField) || !reflect.ValueOf(result).IsZero() || updates != 0 {
						t.Fatalf("invalid thread published: %v", err)
					}
					if _, present := client.ThreadStateSnapshot("other-good"); present {
						t.Fatal("list failure partially published")
					}
				} else if err != nil || updates != 1 {
					t.Fatalf("same-client recovery: updates=%d err=%v", updates, err)
				}
			}
		})
	}
}

func TestRequiredNullPluginCarriers(t *testing.T) {
	ctx := context.Background()
	summary, app, principal := "", "", ""
	for _, record := range requiredNullRecords() {
		switch record.name {
		case "PluginSummary":
			summary = hiddenNullBody(record.body, "enabled", true)
		case "AppSummary":
			app = hiddenNullBody(record.body, "id", true)
		case "PluginSharePrincipal":
			principal = hiddenNullBody(record.body, "name", true)
		}
	}
	marketplaces := `{"marketplaces":[{"name":"market","plugins":[` + summary + `]}]}`
	for _, tc := range []struct {
		method, body string
		call         func(*p.Client) (any, error)
	}{
		{"plugin/list", marketplaces, func(c *p.Client) (any, error) { return c.Plugin.List(ctx, p.PluginListParams{}) }},
		{"plugin/installed", marketplaces, func(c *p.Client) (any, error) { return c.Plugin.Installed(ctx, p.PluginInstalledParams{}) }},
		{"plugin/share/list", `{"data":[{"plugin":` + summary + `}]}`, func(c *p.Client) (any, error) { return c.Plugin.ShareList(ctx, p.PluginShareListParams{}) }},
		{"plugin/install", `{"authPolicy":"ON_INSTALL","appsNeedingAuth":[` + app + `]}`, func(c *p.Client) (any, error) {
			return c.Plugin.Install(ctx, p.PluginInstallParams{PluginName: "plugin"})
		}},
		{"plugin/share/updateTargets", `{"discoverability":"PRIVATE","principals":[` + principal + `]}`, func(c *p.Client) (any, error) {
			return c.Plugin.ShareUpdateTargets(ctx, p.PluginShareUpdateTargetsParams{RemotePluginID: "remote", Discoverability: p.PluginShareUpdateDiscoverability("PRIVATE"), ShareTargets: []p.PluginShareTarget{}})
		}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			mock.SetResponse(tc.method, p.Response{Result: json.RawMessage(tc.body)})
			result, err := tc.call(client)
			if !errors.Is(err, p.ErrNullResultField) || !reflect.ValueOf(result).IsZero() {
				t.Fatalf("alternate carrier published invalid record: %v", err)
			}
			// Remove only this test's inserted escaped null occurrence. The
			// original schema-derived record remains otherwise byte-for-byte.
			valid := tc.body
			for _, prefix := range []string{`"\u0065nabled":null,`, `"\u0069d":null,`, `"\u006eame":null,`} {
				valid = strings.ReplaceAll(valid, prefix, "")
			}
			if valid == tc.body {
				t.Fatal("valid control did not remove injected occurrence")
			}
			mock.SetResponse(tc.method, p.Response{Result: json.RawMessage(valid)})
			if _, err := tc.call(client); err != nil {
				t.Fatalf("schema-derived valid recovery: %v", err)
			}
			if len(mock.SentRequests) != 2 {
				t.Fatal("recovery did not send a second RPC")
			}
		})
	}
}
