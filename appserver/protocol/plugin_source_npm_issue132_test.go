package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func issue132PluginSource(t *testing.T, service, sourceJSON string) (codex.PluginSource, error) {
	t.Helper()
	summary := `{"authPolicy":"ON_USE","enabled":true,"id":"plugin-132","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":` + sourceJSON + `}`
	var method string
	var result string
	switch service {
	case "read":
		method = "plugin/read"
		result = `{"plugin":{"appTemplates":[],"apps":[],"hooks":[],"marketplaceName":"official","mcpServers":[],"skills":[],"summary":` + summary + `}}`
	case "list":
		method = "plugin/list"
		result = `{"marketplaces":[{"name":"official","path":"/plugins","plugins":[` + summary + `]}]}`
	case "installed":
		method = "plugin/installed"
		result = `{"marketplaces":[{"name":"official","path":"/plugins","plugins":[` + summary + `]}]}`
	case "share-list":
		method = "plugin/share/list"
		result = `{"data":[{"plugin":` + summary + `}]}`
	default:
		t.Fatalf("unknown service %q", service)
	}
	transport := NewMockTransport()
	transport.SetResponse(method, codex.Response{JSONRPC: "2.0", Result: json.RawMessage(result)})
	client := codex.NewClient(transport)
	switch service {
	case "read":
		response, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue132StringPointer("official")})
		if err != nil {
			return codex.PluginSource{}, err
		}
		return response.Plugin.Summary.Source, nil
	case "list":
		response, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
		if err != nil {
			return codex.PluginSource{}, err
		}
		return response.Marketplaces[0].Plugins[0].Source, nil
	case "installed":
		response, err := client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
		if err != nil {
			return codex.PluginSource{}, err
		}
		return response.Marketplaces[0].Plugins[0].Source, nil
	default:
		response, err := client.Plugin.ShareList(context.Background(), codex.PluginShareListParams{})
		if err != nil {
			return codex.PluginSource{}, err
		}
		return response.Data[0].Plugin.Source, nil
	}
}

func issue132StringPointer(value string) *string { return &value }

func TestPluginNPMSourcesDecodeAndSerializeAcrossPublicCarriers(t *testing.T) {
	services := []string{"read", "list", "installed", "share-list"}
	tests := []struct {
		name        string
		source      string
		wantPresent map[string]string
		wantAbsent  []string
	}{
		{
			name:        "nonempty package optional metadata absent",
			source:      `{"type":"npm","package":"@example/calendar-plugin"}`,
			wantPresent: map[string]string{"type": `"npm"`, "package": `"@example/calendar-plugin"`},
			wantAbsent:  []string{"registry", "version"},
		},
		{
			name:        "empty package nullable metadata null",
			source:      `{"type":"npm","package":"","registry":null,"version":null}`,
			wantPresent: map[string]string{"type": `"npm"`, "package": `""`, "registry": `null`, "version": `null`},
		},
		{
			name:        "empty optional metadata strings",
			source:      `{"type":"npm","package":"calendar-plugin","registry":"","version":""}`,
			wantPresent: map[string]string{"type": `"npm"`, "package": `"calendar-plugin"`, "registry": `""`, "version": `""`},
		},
		{
			name:        "schema-permitted numeric legacy path extra",
			source:      `{"type":"npm","package":"calendar-plugin","path":123}`,
			wantPresent: map[string]string{"type": `"npm"`, "package": `"calendar-plugin"`},
			wantAbsent:  []string{"path"},
		},
		{
			name:        "schema-permitted object legacy URL extra",
			source:      `{"type":"npm","package":"calendar-plugin","url":{}}`,
			wantPresent: map[string]string{"type": `"npm"`, "package": `"calendar-plugin"`},
			wantAbsent:  []string{"url"},
		},
	}

	for _, service := range services {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s/%s", service, tt.name), func(t *testing.T) {
				source, err := issue132PluginSource(t, service, tt.source)
				if err != nil {
					t.Fatalf("public %s rejected schema-valid npm source %s: %v", service, tt.source, err)
				}
				serialized, err := json.Marshal(source)
				if err != nil {
					t.Fatalf("serialize selected source: %v", err)
				}
				var got map[string]json.RawMessage
				if err := json.Unmarshal(serialized, &got); err != nil {
					t.Fatalf("decode selected source JSON %s: %v", serialized, err)
				}
				for field, want := range tt.wantPresent {
					if string(got[field]) != want {
						t.Errorf("serialized %s = %s; want %s (source=%s)", field, got[field], want, serialized)
					}
				}
				for _, field := range tt.wantAbsent {
					if _, ok := got[field]; ok {
						t.Errorf("serialized source unexpectedly includes %s: %s", field, serialized)
					}
				}
				if len(got) != len(tt.wantPresent) {
					t.Errorf("serialized source fields = %v; want only %v", got, tt.wantPresent)
				}
			})
		}
	}
}

func TestPluginNPMSourcesKeepExistingNonNPMControl(t *testing.T) {
	for _, service := range []string{"read", "list", "installed", "share-list"} {
		t.Run(service, func(t *testing.T) {
			source, err := issue132PluginSource(t, service, `{"type":"remote"}`)
			if err != nil {
				t.Fatalf("public %s rejected existing remote source control: %v", service, err)
			}
			serialized, err := json.Marshal(source)
			if err != nil {
				t.Fatalf("serialize remote source control: %v", err)
			}
			if string(serialized) != `{"type":"remote"}` {
				t.Fatalf("remote source JSON = %s; want {\"type\":\"remote\"}", serialized)
			}
		})
	}
}
