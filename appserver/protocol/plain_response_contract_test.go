package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadResponsePathStringsArePreserved(t *testing.T) {
	for _, path := range []string{"", "relative/thread.json", "/tmp/../thread.json"} {
		t.Run(path, func(t *testing.T) {
			transport := NewMockTransport()
			defer transport.Close()
			thread := responseThread(path)
			if err := transport.SetResponseData("thread/list", map[string]any{"data": []any{thread}}); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(transport)
			defer client.Close()
			got, err := client.Thread.List(context.Background(), codex.ThreadListParams{})
			if err != nil {
				t.Fatalf("Thread.List() error = %v", err)
			}
			if len(got.Data) != 1 || got.Data[0].Path == nil || *got.Data[0].Path != path {
				t.Fatalf("Thread.List() path = %#v, want %q", got.Data, path)
			}
			cached, ok := client.ThreadStateSnapshot(got.Data[0].ID)
			if !ok || cached.Path == nil || *cached.Path != path {
				t.Fatalf("cached path = %#v, %v; want %q", cached.Path, ok, path)
			}
			*got.Data[0].Path = "caller mutation"
			cached, ok = client.ThreadStateSnapshot(got.Data[0].ID)
			if !ok || cached.Path == nil || *cached.Path != path {
				t.Fatalf("cached path after response mutation = %#v, %v; want %q", cached.Path, ok, path)
			}
		})
	}
}

func TestThreadStartedNotificationPreservesPlainPath(t *testing.T) {
	for _, path := range []string{"relative/thread.json", "/tmp/../thread.json"} {
		t.Run(path, func(t *testing.T) {
			transport := NewMockTransport()
			defer transport.Close()
			client := codex.NewClient(transport)
			defer client.Close()
			var got string
			client.OnThreadStarted(func(n codex.ThreadStartedNotification) {
				if n.Thread.Path != nil {
					got = *n.Thread.Path
				}
			})
			params, _ := json.Marshal(map[string]any{"thread": responseThread(path)})
			transport.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/started", Params: params})
			if got != path {
				t.Fatalf("thread/started path = %q, want %q", got, path)
			}
		})
	}
}

func TestThreadResponseStillRequiresAbsoluteCwd(t *testing.T) {
	for _, cwd := range []string{"relative/repo", "/repo/../project"} {
		thread := responseThread("relative/thread.json")
		thread["cwd"] = cwd
		var got codex.Thread
		data, _ := json.Marshal(thread)
		if err := json.Unmarshal(data, &got); err == nil || !strings.Contains(err.Error(), "thread.cwd") {
			t.Errorf("Thread cwd %q error = %v, want absolute path validation", cwd, err)
		}
	}
}

func TestThreadPathKeepsNullableStringShape(t *testing.T) {
	for _, wire := range []string{
		`{"id":"thread-1","projectId":null,"sessionId":"session-1","cliVersion":"1.0","createdAt":1,"cwd":"/repo","ephemeral":false,"modelProvider":"openai","preview":"thread","source":"cli","status":{"type":"idle"},"turns":[],"updatedAt":2}`,
		`{"id":"thread-1","projectId":null,"sessionId":"session-1","cliVersion":"1.0","createdAt":1,"cwd":"/repo","ephemeral":false,"modelProvider":"openai","preview":"thread","source":"cli","status":{"type":"idle"},"turns":[],"updatedAt":2,"path":null}`,
	} {
		var got codex.Thread
		if err := json.Unmarshal([]byte(wire), &got); err != nil {
			t.Fatalf("Thread with absent/null path rejected: %v", err)
		}
		if got.Path != nil {
			t.Fatalf("Thread path = %q, want nil", *got.Path)
		}
	}
	var got codex.Thread
	wrongType, _ := json.Marshal(func() map[string]any {
		thread := responseThread("relative")
		thread["path"] = 7
		return thread
	}())
	if err := json.Unmarshal(wrongType, &got); err == nil {
		t.Fatal("numeric Thread.path accepted; want string type error")
	}
}

func TestMigrationItemPlainCwdDetectAndImportRoundTrip(t *testing.T) {
	for _, cwd := range []string{"", "relative/repo", "/repo/../project"} {
		t.Run(cwd, func(t *testing.T) {
			transport := NewMockTransport()
			defer transport.Close()
			if err := transport.SetResponseData("externalAgentConfig/detect", map[string]any{"items": []any{map[string]any{"cwd": cwd, "description": "repo config", "itemType": "CONFIG"}}}); err != nil {
				t.Fatal(err)
			}
			if err := transport.SetResponseData("externalAgentConfig/import", map[string]any{"importId": "import-1"}); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(transport)
			defer client.Close()
			detected, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
			if err != nil {
				t.Fatalf("ConfigDetect() error = %v", err)
			}
			if len(detected.Items) != 1 || detected.Items[0].Cwd == nil || *detected.Items[0].Cwd != cwd {
				t.Fatalf("detected cwd = %#v, want %q", detected.Items, cwd)
			}
			if _, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{MigrationItems: detected.Items}); err != nil {
				t.Fatalf("ConfigImport() with detected item error = %v", err)
			}
			var params struct {
				MigrationItems []struct {
					Cwd *string `json:"cwd"`
				} `json:"migrationItems"`
			}
			if err := json.Unmarshal(transport.GetSentRequest(1).Params, &params); err != nil {
				t.Fatal(err)
			}
			if len(params.MigrationItems) != 1 || params.MigrationItems[0].Cwd == nil || *params.MigrationItems[0].Cwd != cwd {
				t.Fatalf("import cwd = %#v, want %q", params.MigrationItems, cwd)
			}
			var decoded codex.ExternalAgentConfigMigrationItem
			wire, _ := json.Marshal(map[string]any{"cwd": cwd, "description": "repo config", "itemType": "CONFIG"})
			if err := json.Unmarshal(wire, &decoded); err != nil || decoded.Cwd == nil || *decoded.Cwd != cwd {
				t.Fatalf("decoded import item = %#v, error %v; want cwd %q", decoded, err, cwd)
			}
		})
	}
}

func TestMigrationItemKeepsOptionalCwdAndRequiredFields(t *testing.T) {
	for _, wire := range []string{
		`{"description":"config","itemType":"CONFIG"}`,
		`{"cwd":null,"description":"config","itemType":"CONFIG"}`,
	} {
		item := codex.ExternalAgentConfigMigrationItem{Cwd: codex.Ptr("stale"), Description: "old", ItemType: codex.MigrationItemTypeSkills}
		if err := json.Unmarshal([]byte(wire), &item); err != nil {
			t.Fatalf("migration item %s rejected: %v", wire, err)
		}
		if item.Cwd != nil || item.Description != "config" || item.ItemType != codex.MigrationItemTypeConfig {
			t.Fatalf("migration item = %#v, want absent cwd and decoded required fields", item)
		}
	}
	for _, wire := range []string{
		`{"itemType":"CONFIG"}`,
		`{"description":null,"itemType":"CONFIG"}`,
		`{"description":"config"}`,
		`{"description":"config","itemType":null}`,
		`{"cwd":7,"description":"config","itemType":"CONFIG"}`,
		`{"description":"config","itemType":"PROMPTS"}`,
	} {
		item := codex.ExternalAgentConfigMigrationItem{Cwd: codex.Ptr("retained"), Description: "old", ItemType: codex.MigrationItemTypeSkills}
		if err := json.Unmarshal([]byte(wire), &item); err == nil {
			t.Errorf("migration item %s accepted; want required/type/enum failure", wire)
		}
		if item.Cwd == nil || *item.Cwd != "retained" || item.Description != "old" || item.ItemType != codex.MigrationItemTypeSkills {
			t.Errorf("migration item after rejected %s = %#v; receiver changed", wire, item)
		}
	}
}

func TestSkillsListPreservesPlainErrorPath(t *testing.T) {
	for _, path := range []string{"", "relative/broken-skill", "/repo/../broken-skill"} {
		t.Run(path, func(t *testing.T) {
			transport := NewMockTransport()
			defer transport.Close()
			if err := transport.SetResponseData("skills/list", map[string]any{"data": []any{map[string]any{"cwd": "/repo", "errors": []any{map[string]any{"path": path, "message": "invalid skill"}}, "skills": []any{}}}}); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(transport)
			defer client.Close()
			got, err := client.Skills.List(context.Background(), codex.SkillsListParams{})
			if err != nil {
				t.Fatalf("Skills.List() error = %v", err)
			}
			if len(got.Data) != 1 || len(got.Data[0].Errors) != 1 || got.Data[0].Errors[0].Path != path {
				t.Fatalf("Skills.List() errors = %#v, want path %q", got.Data, path)
			}
		})
	}
}

func TestSkillErrorPathRemainsRequiredString(t *testing.T) {
	for _, wire := range []string{
		`{"message":"bad skill"}`,
		`{"path":null,"message":"bad skill"}`,
		`{"path":7,"message":"bad skill"}`,
		`{"path":"relative/path"}`,
		`{"path":"relative/path","message":null}`,
		`{"path":"relative/path","message":7}`,
	} {
		var got codex.SkillErrorInfo
		if err := json.Unmarshal([]byte(wire), &got); err == nil {
			t.Errorf("SkillErrorInfo accepted %s; want required string validation", wire)
		}
	}
}

func responseThread(path string) map[string]any {
	return map[string]any{
		"id": "thread-1", "projectId": nil, "sessionId": "session-1", "cliVersion": "1.0",
		"createdAt": 1, "cwd": "/repo", "ephemeral": false, "modelProvider": "openai",
		"preview": "thread", "source": "cli", "status": map[string]any{"type": "idle"},
		"turns": []any{}, "updatedAt": 2, "path": path,
	}
}
