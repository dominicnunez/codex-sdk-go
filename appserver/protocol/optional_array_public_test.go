package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const optionalArrayModel = `{"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]}`

func TestOptionalArraysPublicServices(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		body         func(string) string
		call         func(*codex.Client) (any, error)
	}{
		{"apps", "app/list", func(v string) string { return `{"data":[{"id":"a","name":"A","pluginDisplayNames":` + v + `}]}` }, func(c *codex.Client) (any, error) { return c.Apps.List(context.Background(), codex.AppsListParams{}) }},
		{"connector", "app/read", func(v string) string {
			return `{"apps":[{"id":"a","name":"A","pluginDisplayNames":` + v + `}],"missingAppIds":[]}`
		}, func(c *codex.Client) (any, error) {
			return c.Apps.Read(context.Background(), codex.AppsReadParams{AppIDs: []string{"a"}})
		}},
		{"model", "model/list", func(v string) string {
			return `{"data":[` + strings.TrimSuffix(optionalArrayModel, "}") + `,"inputModalities":` + v + `}]}`
		}, func(c *codex.Client) (any, error) { return c.Model.List(context.Background(), codex.ModelListParams{}) }},
		{"plugin-list-keywords", "plugin/list", func(v string) string {
			return `{"marketplaces":[{"name":"M","plugins":[` + strings.TrimSuffix(issue74PluginSummary, "}") + `,"keywords":` + v + `}]}]}`
		}, func(c *codex.Client) (any, error) {
			return c.Plugin.List(context.Background(), codex.PluginListParams{})
		}},
		{"plugin-installed-keywords", "plugin/installed", func(v string) string {
			return `{"marketplaces":[{"name":"M","plugins":[` + strings.TrimSuffix(issue74PluginSummary, "}") + `,"keywords":` + v + `}]}]}`
		}, func(c *codex.Client) (any, error) {
			return c.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
		}},
		{"plugin-read-keywords", "plugin/read", func(v string) string {
			return `{"plugin":{"appTemplates":[],"apps":[],"description":"","hooks":[],"marketplaceName":"M","marketplacePath":"/tmp/plugins","mcpServers":[],"skills":[],"summary":` + strings.TrimSuffix(issue74PluginSummary, "}") + `,"keywords":` + v + `}}}`
		}, func(c *codex.Client) (any, error) {
			return c.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", MarketplacePath: "/tmp/plugins"})
		}},
		{"config-workspace", "config/read", func(v string) string {
			return `{"config":{"sandbox_workspace_write":{"writable_roots":` + v + `}},"origins":{}}`
		}, func(c *codex.Client) (any, error) {
			return c.Config.Read(context.Background(), codex.ConfigReadParams{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			for _, value := range []string{`[]`, `["text"]`, `null`, `[null]`, `true`} {
				mock.SetResponse(tc.method, codex.Response{Result: json.RawMessage(tc.body(value))})
				result, err := tc.call(client)
				valid := value == `[]` || value == `["text"]`
				if (err == nil) != valid {
					t.Fatalf("%s: result=%+v error=%v valid=%v", value, result, err, valid)
				}
			}
		})
	}
	// Detection keeps its legacy public field while admitting both detected
	// source variants. This guard was fixed earlier and must remain effective.
	for _, value := range []string{`[]`, `[{"name":"C","sessionCount":0,"source":"sessionToolUse"}]`, `null`, `[null]`, `[{}]`} {
		mock := NewMockTransport()
		mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(`{"items":[],"connectors":` + value + `}`)})
		client := codex.NewClient(mock)
		result, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
		_ = client.Close()
		if (err == nil) != (value == `[]` || strings.Contains(value, "sessionToolUse")) {
			t.Fatalf("detected %s: %+v, %v", value, result, err)
		}
	}
}

func TestOptionalLifecycleArraysFailBeforeCache(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		for _, field := range []string{"disabledPluginIds", "instructionSources", "writableRoots"} {
			t.Run(method+"/"+field, func(t *testing.T) {
				mock := NewMockTransport()
				client := codex.NewClient(mock)
				t.Cleanup(func() { _ = client.Close() })
				var prior codex.Thread
				encoded, err := json.Marshal(validProcessThreadPayload("array-thread"))
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(encoded, &prior); err != nil {
					t.Fatal(err)
				}
				prior.Preview = "prior"
				prior.DisabledPluginIDs = []string{"old"}
				client.CacheThreadState(prior)
				updates := 0
				remove := client.AddThreadStateListener(prior.ID, func(codex.Thread) { updates++ }, nil)
				defer remove()
				before, _ := client.ThreadStateSnapshot(prior.ID)
				initialUpdates := updates
				call := func() (any, error) {
					switch method {
					case "thread/start":
						return client.Thread.Start(context.Background(), codex.ThreadStartParams{})
					case "thread/resume":
						return client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: prior.ID})
					default:
						return client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: prior.ID})
					}
				}
				for _, value := range []string{`null`, `[null]`, `true`, `["new"]`} {
					thread := validProcessThreadPayload(prior.ID)
					thread["preview"] = "new"
					response := validThreadLifecycleResponse(thread)
					if field == "writableRoots" {
						response["sandbox"] = map[string]any{"type": "workspaceWrite", field: json.RawMessage(value)}
						if value == `["new"]` {
							response["sandbox"] = map[string]any{"type": "workspaceWrite", field: []string{"/tmp/new"}}
						}
					} else {
						response[field] = json.RawMessage(value)
					}
					if err := mock.SetResponseData(method, response); err != nil {
						t.Fatal(err)
					}
					result, err := call()
					if value != `["new"]` {
						if err == nil {
							t.Fatalf("accepted %s", value)
						}
						after, ok := client.ThreadStateSnapshot(prior.ID)
						if !ok || !reflect.DeepEqual(before, after) || updates != initialUpdates {
							t.Fatal("rejected response changed cached state or notified listeners")
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						after, ok := client.ThreadStateSnapshot(prior.ID)
						if !ok || after.Preview != "new" || updates != initialUpdates+1 {
							t.Fatal("valid response did not update cache once")
						}
						if field != "writableRoots" && !reflect.DeepEqual(optionalArrayValue(t, result, field).Interface(), []string{"new"}) {
							t.Fatal("valid lifecycle array lost")
						}
						if field == "disabledPluginIds" && !reflect.DeepEqual(after.DisabledPluginIDs, []string{"new"}) {
							t.Fatal("disabled plugins lost at cache boundary")
						}
					}
				}
			})
		}
	}
}

func TestOptionalArraysTypedNotifications(t *testing.T) {
	for _, method := range []string{"app/list/updated", "item/started", "item/completed"} {
		for _, field := range []string{"summary", "content", "text_elements"} {
			if method == "app/list/updated" && field != "summary" {
				continue
			}
			t.Run(method+"/"+field, func(t *testing.T) {
				mock := NewMockTransport()
				callbacks, reports := 0, 0
				client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(gotMethod string, err error) {
					if gotMethod != method || err == nil {
						t.Errorf("diagnostic method=%s error=%v", gotMethod, err)
					}
					reports++
				}))
				t.Cleanup(func() { _ = client.Close() })
				client.OnAppListUpdated(func(n codex.AppListUpdatedNotification) {
					callbacks++
					if len(n.Data) != 1 || n.Data[0].ID != "a" {
						t.Error("lost valid app")
					}
				})
				client.OnItemStarted(func(n codex.ItemStartedNotification) {
					callbacks++
					if n.Item.Value == nil {
						t.Error("lost valid item")
					}
				})
				client.OnItemCompleted(func(n codex.ItemCompletedNotification) {
					callbacks++
					if n.Item.Value == nil {
						t.Error("lost valid item")
					}
				})
				populated := `["text"]`
				if field == "text_elements" {
					populated = `[{"byteRange":{"start":0,"end":1}}]`
				}
				wantCalls, wantReports := 0, 0
				for _, value := range []string{`null`, `[null]`, `true`, `[]`, populated} {
					params := `{"data":[{"id":"a","name":"A","pluginDisplayNames":` + value + `}]}`
					if method != "app/list/updated" {
						params = `{"threadId":"array-thread","turnId":"turn","startedAtMs":0,"completedAtMs":0,"item":` + optionalArrayItem(field, value) + `}`
					}
					mock.InjectServerNotification(context.Background(), codex.Notification{Method: method, Params: json.RawMessage(params)})
					if value == `[]` || value == populated {
						wantCalls++
					} else {
						wantReports++
					}
					if callbacks != wantCalls || reports != wantReports {
						t.Fatalf("%s: callbacks=%d reports=%d want=%d/%d", value, callbacks, reports, wantCalls, wantReports)
					}
				}
			})
		}
	}
}

func optionalArrayItem(field, value string) string {
	if field == "text_elements" {
		return `{"type":"userMessage","id":"i","content":[{"type":"text","text":"hello","text_elements":` + value + `}]}`
	}
	return `{"type":"reasoning","id":"i","` + field + `":` + value + `}`
}

func TestOptionalArraysPersistedItems(t *testing.T) {
	for _, field := range []string{"summary", "content", "text_elements"} {
		t.Run(field, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			prior := codex.Thread{ID: "array-thread", Preview: "prior"}
			client.CacheThreadState(prior)
			populated := `["text"]`
			if field == "text_elements" {
				populated = `[{"byteRange":{"start":0,"end":1}}]`
			}
			for _, value := range []string{`null`, `[null]`, `true`, populated} {
				thread := validProcessThreadPayload(prior.ID)
				thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": []json.RawMessage{json.RawMessage(optionalArrayItem(field, value))}}}
				if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
					t.Fatal(err)
				}
				result, err := client.Thread.Read(context.Background(), codex.ThreadReadParams{ThreadID: prior.ID})
				if value != populated {
					if err == nil {
						t.Fatalf("accepted persisted item %s", value)
					}
					state, ok := client.ThreadStateSnapshot(prior.ID)
					if !ok || !reflect.DeepEqual(prior, state) {
						t.Fatal("invalid persisted item replaced cached thread")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				item := result.Thread.Turns[0].Items[0]
				var array any
				if field == "text_elements" {
					array = item.Value.(*codex.UserMessageThreadItem).Content[0].(*codex.TextUserInput).TextElements
				} else {
					array = optionalArrayValue(t, &item, field).Interface()
				}
				encoded, err := json.Marshal(array)
				if err != nil {
					t.Fatal(err)
				}
				issue74EqualJSON(t, encoded, []byte(populated))
				state, ok := client.ThreadStateSnapshot(prior.ID)
				if !ok || len(state.Turns) != 1 {
					t.Fatal("valid persisted item not cached")
				}
			}
		})
	}
}
