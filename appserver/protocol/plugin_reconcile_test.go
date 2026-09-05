package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPluginReconcile(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("plugin/reconcile", codex.Response{Result: json.RawMessage(`{"changedPlugins":[{"id":"plugin@marketplace","hasApps":false,"hasHooks":true,"hasMcps":false,"hasSkills":true}],"failedRemotePluginIds":["remote-1"],"failedMaterializationRemotePluginIds":[]}`)})
	response, err := client.Plugin.Reconcile(context.Background(), codex.PluginReconcileParams{Reason: strPtr("refresh")})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ChangedPlugins) != 1 || response.ChangedPlugins[0].ID != "plugin@marketplace" || !response.ChangedPlugins[0].HasHooks || response.ChangedPlugins[0].HasApps || response.ChangedPlugins[0].HasMcps || !response.ChangedPlugins[0].HasSkills {
		t.Fatalf("changedPlugins = %+v", response.ChangedPlugins)
	}
	if len(response.FailedRemotePluginIDs) != 1 || response.FailedRemotePluginIDs[0] != "remote-1" || response.FailedMaterializationRemotePluginIDs == nil {
		t.Fatalf("response = %+v", response)
	}
	req := transport.GetSentRequest(0)
	if req.Method != "plugin/reconcile" || string(req.Params) != `{"reason":"refresh"}` {
		t.Fatalf("request = %+v", req)
	}

	_, err = client.Plugin.Reconcile(context.Background(), codex.PluginReconcileParams{})
	if err != nil {
		t.Fatal(err)
	}
	if string(transport.GetSentRequest(1).Params) != `{}` {
		t.Fatalf("optional reason was not omitted")
	}
}

func TestPluginReconcileRejectsMalformedResponses(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"changedPlugins":null,"failedRemotePluginIds":[],"failedMaterializationRemotePluginIds":[]}`,
		`{"changedPlugins":[],"failedRemotePluginIds":[],"failedMaterializationRemotePluginIds":null}`,
		`{"changedPlugins":[],"failedRemotePluginIds":null,"failedMaterializationRemotePluginIds":[]}`,
		`{"changedPlugins":[{"id":"p","hasApps":false,"hasHooks":false,"hasMcps":false}],"failedRemotePluginIds":[],"failedMaterializationRemotePluginIds":[]}`,
	} {
		var response codex.PluginReconcileResponse
		if err := json.Unmarshal([]byte(payload), &response); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
}
