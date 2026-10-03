package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestIssue74MigrationDetectImportPreservesDetails(t *testing.T) {
	details := `{"commands":[{"name":"build"}],"hooks":[{"name":"check"}],"mcpServers":[{"name":"docs"}],"memory":["preferences"],"plugins":[{"marketplaceName":"team","pluginNames":["tools"]}],"sessions":[{"cwd":"relative/project","path":"relative/session.json","title":"Session"}],"skills":[{"name":"review"}],"subagents":[{"name":"helper"}]}`
	issue74MigrationRoundTrip(t, details)
}

func TestIssue74MigrationEmptyListsPreserved(t *testing.T) {
	issue74MigrationRoundTrip(t, `{"commands":[],"hooks":[],"mcpServers":[],"memory":[],"plugins":[],"sessions":[],"skills":[],"subagents":[]}`)
	issue74MigrationRoundTrip(t, `{}`)
}

func issue74MigrationRoundTrip(t *testing.T, details string) {
	t.Helper()
	mock := NewMockTransport()
	mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(`{"items":[{"description":"Migration","itemType":"CONFIG","details":` + details + `}]}`)})
	mock.SetResponse("externalAgentConfig/import", codex.Response{Result: json.RawMessage(`{"importId":"migration"}`)})
	client := codex.NewClient(mock)
	detected, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{MigrationItems: detected.Items}); err != nil {
		t.Fatal(err)
	}
	var request struct {
		MigrationItems []struct {
			Details json.RawMessage `json:"details"`
		} `json:"migrationItems"`
	}
	if err := json.Unmarshal(mock.SentRequests[1].Params, &request); err != nil {
		t.Fatal(err)
	}
	issue74EqualJSON(t, request.MigrationItems[0].Details, []byte(details))
}

func TestIssue74MigrationDetailsValidation(t *testing.T) {
	for _, details := range []string{
		`{"commands":null}`, `{"hooks":null}`, `{"mcpServers":null}`, `{"memory":null}`,
		`{"plugins":null}`, `{"sessions":null}`, `{"skills":null}`, `{"subagents":null}`,
		`{"commands":[{}]}`, `{"hooks":[{"name":null}]}`, `{"mcpServers":[{}]}`,
		`{"plugins":[{"marketplaceName":"team"}]}`,
		`{"plugins":[{"marketplaceName":"team","pluginNames":null}]}`,
		`{"sessions":[{"cwd":"relative"}]}`, `{"sessions":[{"cwd":null,"path":"relative"}]}`,
		`{"skills":[{}]}`, `{"subagents":[{}]}`, `{"commands":[null]}`, `{"memory":[null]}`,
		`{"plugins":[{"marketplaceName":"team","pluginNames":[null]}]}`, `[]`, `"not an object"`,
	} {
		t.Run(details, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(`{"items":[{"description":"Migration","itemType":"CONFIG","details":` + details + `}]}`)})
			client := codex.NewClient(mock)
			if _, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{}); err == nil {
				t.Fatal("invalid migration details accepted")
			}
		})
	}
	for _, details := range []string{`null`, `{}`, `{"commands":[],"hooks":[],"mcpServers":[],"memory":[],"plugins":[],"sessions":[],"skills":[],"subagents":[]}`, `{"sessions":[{"cwd":"relative","path":"relative","title":null}]}`} {
		t.Run("valid/"+details, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(`{"items":[{"description":"Migration","itemType":"CONFIG","details":` + details + `}]}`)})
			if _, err := codex.NewClient(mock).ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIssue74MigrationImportRejectsNilPluginNames(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	_, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{
		MigrationItems: []codex.ExternalAgentConfigMigrationItem{{
			Description: "Plugins", ItemType: codex.MigrationItemTypePlugins,
			Details: &codex.MigrationDetails{Plugins: []codex.PluginsMigration{{MarketplaceName: "team"}}},
		}},
	})
	if err == nil || len(mock.SentRequests) != 0 {
		t.Fatalf("invalid plugin names: error = %v, sent requests = %d", err, len(mock.SentRequests))
	}
}

func TestIssue74FuzzyMatchTypePublicBoundaries(t *testing.T) {
	for _, matchType := range []string{"file", "directory"} {
		t.Run(matchType, func(t *testing.T) {
			file := `{"path":"name","file_name":"name","root":"/project","score":0,"match_type":"` + matchType + `"}`
			mock := NewMockTransport()
			mock.SetResponse("fuzzyFileSearch", codex.Response{Result: json.RawMessage(`{"files":[` + file + `]}`)})
			client := codex.NewClient(mock)
			response, err := client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{Query: "name", Roots: []string{"/project"}})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(response.Files[0])
			if err != nil {
				t.Fatal(err)
			}
			issue74EqualJSON(t, encoded, []byte(file))
			var received *codex.FuzzyFileSearchSessionUpdatedNotification
			client.OnFuzzyFileSearchSessionUpdated(func(notification codex.FuzzyFileSearchSessionUpdatedNotification) { received = &notification })
			mock.InjectServerNotification(context.Background(), codex.Notification{Method: "fuzzyFileSearch/sessionUpdated", Params: json.RawMessage(`{"sessionId":"search","query":"name","files":[` + file + `]}`)})
			if received == nil {
				t.Fatal("session update not delivered")
			}
			encoded, err = json.Marshal(received.Files[0])
			if err != nil {
				t.Fatal(err)
			}
			issue74EqualJSON(t, encoded, []byte(file))
		})
	}
}

func TestIssue74FuzzyMatchTypeRejectsInvalid(t *testing.T) {
	for _, property := range []string{``, `,"match_type":null`, `,"match_type":"unknown"`, `,"match_type":7`} {
		t.Run(property, func(t *testing.T) {
			file := `{"path":"name","file_name":"name","root":"/project","score":0` + property + `}`
			mock := NewMockTransport()
			mock.SetResponse("fuzzyFileSearch", codex.Response{Result: json.RawMessage(`{"files":[` + file + `]}`)})
			var handlerErr error
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, err error) { handlerErr = err }))
			if _, err := client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{Query: "name", Roots: []string{"/project"}}); err == nil {
				t.Error("invalid result accepted")
			}
			called := false
			client.OnFuzzyFileSearchSessionUpdated(func(codex.FuzzyFileSearchSessionUpdatedNotification) { called = true })
			mock.InjectServerNotification(context.Background(), codex.Notification{Method: "fuzzyFileSearch/sessionUpdated", Params: json.RawMessage(`{"sessionId":"search","query":"name","files":[` + file + `]}`)})
			if called || handlerErr == nil {
				t.Fatalf("invalid notification: called = %v, handler error = %v", called, handlerErr)
			}
		})
	}
}

func TestIssue74HookMetadataPublicBranches(t *testing.T) {
	for _, properties := range []string{
		`,"handlerType":"command","command":"echo ready","async":true,"additionalContextLimit":0`,
		`,"handlerType":"command","command":"echo ready","async":false`,
		`,"handlerType":"mcpTool","server":"docs","tool":"search","additionalContextLimit":8192`,
		`,"handlerType":"prompt"`,
		`,"handlerType":"agent"`,
	} {
		t.Run(properties, func(t *testing.T) {
			payload := issue74HookPayload(properties)
			response, err := issue74ReadHooks(payload)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(response.Data[0].Hooks[0])
			if err != nil {
				t.Fatal(err)
			}
			issue74EqualJSON(t, encoded, []byte(payload))
		})
	}
	for _, properties := range []string{
		`,"handlerType":"command","command":"echo ready"`,
		`,"handlerType":"command","command":"echo ready","additionalContextLimit":null`,
	} {
		t.Run("valid/"+properties, func(t *testing.T) {
			if _, err := issue74ReadHooks(issue74HookPayload(properties)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIssue74HookMetadataRejectsInvalidBranches(t *testing.T) {
	for _, properties := range []string{
		`,"handlerType":"command"`,
		`,"handlerType":"command","command":null`,
		`,"handlerType":"command","command":"echo ready","async":null`,
		`,"handlerType":"command","command":"echo ready","async":"yes"`,
		`,"handlerType":"mcpTool","server":"docs"`,
		`,"handlerType":"mcpTool","tool":"search"`,
		`,"handlerType":"mcpTool","server":null,"tool":"search"`,
		`,"handlerType":"mcpTool","server":"docs","tool":null`,
		`,"handlerType":"prompt","additionalContextLimit":-1`,
		`,"handlerType":"agent","additionalContextLimit":1.5`,
	} {
		t.Run(properties, func(t *testing.T) {
			if _, err := issue74ReadHooks(issue74HookPayload(properties)); err == nil {
				t.Fatal("invalid hook metadata accepted")
			}
		})
	}
}

func issue74ReadHooks(payload string) (codex.HooksListResponse, error) {
	mock := NewMockTransport()
	mock.SetResponse("hooks/list", codex.Response{Result: json.RawMessage(`{"data":[{"cwd":"relative/project","errors":[],"warnings":[],"hooks":[` + payload + `]}]}`)})
	return codex.NewClient(mock).Hooks.List(context.Background(), codex.HooksListParams{})
}

func issue74HookPayload(properties string) string {
	return `{"currentHash":"hash","displayOrder":0,"enabled":true,"eventName":"preToolUse","isManaged":false,"key":"hook","source":"user","sourcePath":"/config/hooks.json","timeoutSec":0,"trustStatus":"trusted"` + properties + `}`
}

func issue74EqualJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue interface{}
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode got JSON %s: %v", got, err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}
