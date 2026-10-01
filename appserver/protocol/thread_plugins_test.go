package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestLifecycleDisabledPlugins(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		t.Run(method, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			call := func() (codex.Thread, []string, error) {
				switch method {
				case "thread/start":
					r, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{})
					return r.Thread, r.DisabledPluginIDs, err
				case "thread/resume":
					r, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread"})
					return r.Thread, r.DisabledPluginIDs, err
				default:
					r, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "parent"})
					return r.Thread, r.DisabledPluginIDs, err
				}
			}
			fixture := validThreadLifecycleResponse(validThreadPayload("thread"))
			fixture["disabledPluginIds"] = []string{"plugin-1", "plugin-2"}
			if err := transport.SetResponseData(method, fixture); err != nil {
				t.Fatal(err)
			}
			thread, plugins, err := call()
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"plugin-1", "plugin-2"}
			if !reflect.DeepEqual(thread.DisabledPluginIDs, want) {
				t.Fatalf("returned thread plugins = %v, want %v", thread.DisabledPluginIDs, want)
			}
			encoded, err := json.Marshal(thread)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if _, exists := fields["disabledPluginIds"]; exists {
				t.Fatal("cached plugin settings leaked into the Thread wire object")
			}
			plugins[0] = "response-mutated"
			if !reflect.DeepEqual(thread.DisabledPluginIDs, want) {
				t.Fatal("response settings alias returned thread")
			}
			thread.DisabledPluginIDs[0] = "thread-mutated"
			cached, ok := client.ThreadStateSnapshot("thread")
			if !ok || !reflect.DeepEqual(cached.DisabledPluginIDs, want) {
				t.Fatalf("cached plugins = %v, want %v", cached.DisabledPluginIDs, want)
			}
			// Thread read responses do not carry lifecycle settings.
			if err := transport.SetResponseData("thread/read", map[string]interface{}{"thread": validThreadPayload("thread")}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: "thread"}); err != nil {
				t.Fatal(err)
			}
			cached, _ = client.ThreadStateSnapshot("thread")
			if !reflect.DeepEqual(cached.DisabledPluginIDs, want) {
				t.Fatalf("read discarded cached plugins: %v", cached.DisabledPluginIDs)
			}
			// A later lifecycle response defaults omitted settings to an empty list.
			delete(fixture, "disabledPluginIds")
			if err := transport.SetResponseData(method, fixture); err != nil {
				t.Fatal(err)
			}
			if _, _, err := call(); err != nil {
				t.Fatal(err)
			}
			cached, _ = client.ThreadStateSnapshot("thread")
			if len(cached.DisabledPluginIDs) != 0 {
				t.Fatalf("lifecycle default retained stale plugins: %v", cached.DisabledPluginIDs)
			}
		})
	}
}

func TestSettingsDisabledPlugins(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	client.CacheThreadState(codex.Thread{ID: "thread", DisabledPluginIDs: []string{"old-plugin"}})
	var observed codex.Thread
	unsubscribe := client.AddThreadStateListener("thread", func(thread codex.Thread) { observed = thread }, nil)
	defer unsubscribe()
	settings := map[string]interface{}{
		"approvalPolicy": "untrusted", "approvalsReviewer": "user",
		"collaborationMode": map[string]interface{}{"mode": "default", "settings": map[string]interface{}{"model": "gpt-4"}},
		"cwd":               "/tmp", "model": "gpt-4", "modelProvider": "openai",
		"sandboxPolicy": map[string]interface{}{"type": "readOnly"},
	}
	for _, plugins := range [][]string{{"new-plugin"}, {}} {
		settings["disabledPluginIds"] = plugins
		payload, err := json.Marshal(map[string]interface{}{"threadId": "thread", "threadSettings": settings})
		if err != nil {
			t.Fatal(err)
		}
		transport.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/settings/updated", Params: payload})
		cached, _ := client.ThreadStateSnapshot("thread")
		if !reflect.DeepEqual(cached.DisabledPluginIDs, plugins) || !reflect.DeepEqual(observed.DisabledPluginIDs, plugins) {
			t.Fatalf("settings update lost: cached %v, observed %v, want %v", cached.DisabledPluginIDs, observed.DisabledPluginIDs, plugins)
		}
		if len(plugins) > 0 {
			observed.DisabledPluginIDs[0] = "listener-mutated"
			cached, _ = client.ThreadStateSnapshot("thread")
			if cached.DisabledPluginIDs[0] != "new-plugin" {
				t.Fatal("listener mutation changed cache")
			}
		}
	}
}
