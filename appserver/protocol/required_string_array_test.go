package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRequiredStringArrayLoadedList(t *testing.T) {
	for _, body := range []string{
		`{"data":[null]}`,
		`{"data":["valid",null]}`,
		`{"data":[null],"data":["later"]}`,
		`{"data":["valid"],"DATA":[null]}`,
		`{"data":[null],"data":[]}`,
	} {
		t.Run(body, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponse("thread/loaded/list", protocol.Response{Result: json.RawMessage(body)})
			client := protocol.NewClient(mock)
			defer client.Close()
			response, err := client.Thread.LoadedList(context.Background(), protocol.ThreadLoadedListParams{})
			if err == nil || response.Data != nil {
				t.Fatal("schema-invalid null thread ID was published by LoadedList")
			}
			original := protocol.ThreadLoadedListResponse{Data: []string{"prior"}}
			receiver := original
			if err := json.Unmarshal([]byte(body), &receiver); err == nil || !reflect.DeepEqual(receiver, original) {
				t.Fatal("invalid required string array replaced the prior receiver")
			}
			mock.SetResponse("thread/loaded/list", protocol.Response{Result: json.RawMessage(`{"data":["","recovered"]}`)})
			response, err = client.Thread.LoadedList(context.Background(), protocol.ThreadLoadedListParams{})
			if err != nil || !reflect.DeepEqual(response.Data, []string{"", "recovered"}) {
				t.Fatal("valid empty string or recovery failed")
			}
		})
	}
	var empty protocol.ThreadLoadedListResponse
	if err := json.Unmarshal([]byte(`{"data":[]}`), &empty); err != nil || empty.Data == nil || len(empty.Data) != 0 {
		t.Fatal("valid present empty array changed")
	}
}

func TestRequiredStringArraySiblingAdmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		newValue func() any
	}{
		{"hook warnings", `{"cwd":"/tmp","errors":[],"hooks":[],"warnings":[null]}`, func() any { return new(protocol.HooksListEntry) }},
		{"marketplace selection", `{"errors":[],"selectedMarketplaces":[null],"upgradedRoots":[]}`, func() any { return new(protocol.MarketplaceUpgradeResponse) }},
		{"reconcile materialization IDs", `{"changedPlugins":[],"failedMaterializationRemotePluginIds":[null],"failedRemotePluginIds":[]}`, func() any { return new(protocol.PluginReconcileResponse) }},
		{"reconcile remote IDs", `{"changedPlugins":[],"failedMaterializationRemotePluginIds":[],"failedRemotePluginIds":[null]}`, func() any { return new(protocol.PluginReconcileResponse) }},
		{"plugin capabilities", `{"capabilities":[null],"screenshotUrls":[],"screenshots":[]}`, func() any { return new(protocol.PluginInterface) }},
		{"plugin screenshot URLs", `{"capabilities":[],"screenshotUrls":[null],"screenshots":[]}`, func() any { return new(protocol.PluginInterface) }},
		{"citation thread IDs", `{"type":"agentMessage","id":"i","text":"","memoryCitation":{"entries":[],"threadIds":[null]}}`, func() any { return new(protocol.ThreadItemWrapper) }},
		{"collab receiver IDs", `{"type":"collabAgentToolCall","id":"i","agentsStates":{},"receiverThreadIds":[null],"senderThreadId":"t","status":"completed","tool":"spawnAgent"}`, func() any { return new(protocol.ThreadItemWrapper) }},
		{"warning sample paths", `{"extraCount":0,"failedScan":false,"samplePaths":[null]}`, func() any { return new(protocol.WindowsWorldWritableWarningNotification) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid := strings.ReplaceAll(tc.body, "[null]", `[""]`)
			if err := json.Unmarshal([]byte(valid), tc.newValue()); err != nil {
				t.Fatalf("valid empty string fixture rejected: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.body), tc.newValue()); err == nil {
				t.Fatal("schema-invalid null item admitted by sibling SDK owner")
			}
		})
	}
	t.Run("plugin MCP server names through Read", func(t *testing.T) {
		detail := issue74PluginDetail()
		detail["mcpServers"] = []any{nil}
		response, err := issue74Read(t, detail)
		if err == nil || response.Plugin.McpServers != nil {
			t.Fatal("null MCP server name published by Plugin.Read")
		}
	})
}

func TestRequiredStringArrayIgnoredAliases(t *testing.T) {
	// These owners intentionally match exact schema keys. Uppercase properties
	// are ignored by their wire decoder, including malformed values.
	var hooks protocol.HooksListEntry
	if err := json.Unmarshal([]byte(`{"cwd":"/tmp","errors":[],"hooks":[],"warnings":[],"WARNINGS":[null]}`), &hooks); err != nil {
		t.Fatalf("ignored hook property changed admission: %v", err)
	}
	var warning protocol.WindowsWorldWritableWarningNotification
	if err := json.Unmarshal([]byte(`{"extraCount":0,"failedScan":false,"samplePaths":[],"SAMPLEPATHS":[null]}`), &warning); err != nil {
		t.Fatalf("ignored warning property changed admission: %v", err)
	}
}
