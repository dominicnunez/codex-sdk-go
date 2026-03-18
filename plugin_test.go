package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestPluginList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		forceRemoteSync := true
		transport.SetResponse("plugin/list", codex.Response{
			JSONRPC: "2.0",
			Result: json.RawMessage(`{
				"marketplaces":[
					{
						"name":"official",
						"path":"/plugins",
						"plugins":[
							{
								"authPolicy":"ON_INSTALL",
								"enabled":true,
								"id":"plugin-1",
								"installPolicy":"AVAILABLE",
								"installed":true,
								"name":"calendar",
								"source":{"path":"/plugins/calendar","type":"local"}
							}
						]
					}
				],
				"remoteSyncError":"stale cache"
			}`),
		})

		resp, err := client.Plugin.List(context.Background(), codex.PluginListParams{
			Cwds:            []string{"/workspace"},
			ForceRemoteSync: &forceRemoteSync,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(resp.Marketplaces) != 1 || len(resp.Marketplaces[0].Plugins) != 1 {
			t.Fatalf("response = %+v; want decoded marketplace listing", resp)
		}
		if resp.Marketplaces[0].Plugins[0].ID != "plugin-1" {
			t.Fatalf("plugin id = %q; want plugin-1", resp.Marketplaces[0].Plugins[0].ID)
		}
		if resp.RemoteSyncError == nil || *resp.RemoteSyncError != "stale cache" {
			t.Fatalf("remote sync error = %v; want stale cache", resp.RemoteSyncError)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "plugin/list" {
			t.Fatalf("method = %q; want plugin/list", req.Method)
		}
		var params codex.PluginListParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if len(params.Cwds) != 1 || params.Cwds[0] != "/workspace" || params.ForceRemoteSync == nil || !*params.ForceRemoteSync {
			t.Fatalf("params = %+v; want list payload preserved", params)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/list", codex.Response{JSONRPC: "2.0"})

		_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/list", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"marketplaces":"bad"}`),
		})

		_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/list", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestPluginRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/read", codex.Response{
			JSONRPC: "2.0",
			Result: json.RawMessage(`{
				"plugin":{
					"apps":[{"id":"app-1","name":"Calendar","description":"desc"}],
					"description":"Plugin description",
					"marketplaceName":"official",
					"marketplacePath":"/plugins",
					"mcpServers":["calendar"],
					"skills":[{"description":"skill desc","name":"book","path":"/plugins/book"}],
					"summary":{
						"authPolicy":"ON_USE",
						"enabled":true,
						"id":"plugin-1",
						"installPolicy":"AVAILABLE",
						"installed":true,
						"name":"calendar",
						"source":{"path":"/plugins/calendar","type":"local"}
					}
				}
			}`),
		})

		resp, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{
			MarketplacePath: "/plugins",
			PluginName:      "calendar",
		})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if resp.Plugin.MarketplaceName != "official" {
			t.Fatalf("marketplace name = %q; want official", resp.Plugin.MarketplaceName)
		}
		if len(resp.Plugin.Apps) != 1 || resp.Plugin.Apps[0].ID != "app-1" {
			t.Fatalf("apps = %+v; want decoded app list", resp.Plugin.Apps)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "plugin/read" {
			t.Fatalf("method = %q; want plugin/read", req.Method)
		}
		var params codex.PluginReadParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.MarketplacePath != "/plugins" || params.PluginName != "calendar" {
			t.Fatalf("params = %+v; want read payload preserved", params)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/read", codex.Response{JSONRPC: "2.0"})

		_, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/read", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"plugin":"bad"}`),
		})

		_, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/read", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestPluginInstall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		forceRemoteSync := true
		transport.SetResponse("plugin/install", codex.Response{
			JSONRPC: "2.0",
			Result: json.RawMessage(`{
				"appsNeedingAuth":[{"id":"app-1","name":"Calendar","installUrl":"https://example.com/install"}],
				"authPolicy":"ON_INSTALL"
			}`),
		})

		resp, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{
			MarketplacePath: "/plugins",
			PluginName:      "calendar",
			ForceRemoteSync: &forceRemoteSync,
		})
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		if resp.AuthPolicy != codex.PluginAuthPolicyOnInstall {
			t.Fatalf("auth policy = %q; want %q", resp.AuthPolicy, codex.PluginAuthPolicyOnInstall)
		}
		if len(resp.AppsNeedingAuth) != 1 || resp.AppsNeedingAuth[0].ID != "app-1" {
			t.Fatalf("apps needing auth = %+v; want decoded app", resp.AppsNeedingAuth)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "plugin/install" {
			t.Fatalf("method = %q; want plugin/install", req.Method)
		}
		var params codex.PluginInstallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.MarketplacePath != "/plugins" || params.PluginName != "calendar" || params.ForceRemoteSync == nil || !*params.ForceRemoteSync {
			t.Fatalf("params = %+v; want install payload preserved", params)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/install", codex.Response{JSONRPC: "2.0"})

		_, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{})
		if !errors.Is(err, codex.ErrEmptyResult) {
			t.Fatalf("error = %v; want ErrEmptyResult", err)
		}
	})

	t.Run("malformed result", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/install", codex.Response{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"appsNeedingAuth":"bad","authPolicy":"ON_INSTALL"}`),
		})

		_, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{})
		if err == nil {
			t.Fatal("expected malformed result error")
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/install", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}

func TestPluginUninstall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		forceRemoteSync := true

		_, err := client.Plugin.Uninstall(context.Background(), codex.PluginUninstallParams{
			PluginID:        "plugin-1",
			ForceRemoteSync: &forceRemoteSync,
		})
		if err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}

		req := transport.GetSentRequest(0)
		if req.Method != "plugin/uninstall" {
			t.Fatalf("method = %q; want plugin/uninstall", req.Method)
		}
		var params codex.PluginUninstallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params.PluginID != "plugin-1" || params.ForceRemoteSync == nil || !*params.ForceRemoteSync {
			t.Fatalf("params = %+v; want uninstall payload preserved", params)
		}
	})

	t.Run("rpc error", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/uninstall", codex.Response{
			JSONRPC: "2.0",
			Error: &codex.Error{
				Code:    codex.ErrCodeInternalError,
				Message: "boom",
			},
		})

		_, err := client.Plugin.Uninstall(context.Background(), codex.PluginUninstallParams{PluginID: "plugin-1"})
		assertRPCErrorCode(t, err, codex.ErrCodeInternalError)
	})
}
