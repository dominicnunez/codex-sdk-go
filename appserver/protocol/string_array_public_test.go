package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func stringArrayItem(tc stringArrayContractCase, value string) string {
	body := strings.Replace(tc.body, "@VALUE@", value, 1)
	switch tc.name {
	case "question options":
		return `{"type":"agentMessage","id":"i","text":"","questions":[` + body + `]}`
	case "web queries":
		return `{"type":"webSearch","id":"i","query":"","action":` + body + `}`
	default:
		return body
	}
}

func stringArrayItemCase(tc stringArrayContractCase) bool {
	return tc.name == "citation threads" || tc.name == "collab receivers" || tc.name == "question options" || tc.name == "web queries"
}

func stringArrayExpected(t *testing.T, tc stringArrayContractCase) []string {
	t.Helper()
	var want []string
	if err := json.Unmarshal([]byte("["+tc.validItem+"]"), &want); err != nil {
		t.Fatalf("invalid expected item %q: %v", tc.validItem, err)
	}
	return want
}

// assertStringArrayAt follows a schema-owned carrier path in the public value.
// The path is supplied by each entry point below, so a same-named field on an
// unrelated sibling cannot satisfy the assertion.
func assertStringArrayAt(t *testing.T, value any, want []string, path ...any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal public value: %v", err)
	}
	var current any
	if err := json.Unmarshal(raw, &current); err != nil {
		t.Fatalf("decode public value: %v", err)
	}
	for _, part := range path {
		switch key := part.(type) {
		case string:
			object, ok := current.(map[string]any)
			if !ok {
				t.Fatalf("path %v: expected object before %q, got %T", path, key, current)
			}
			var exists bool
			current, exists = object[key]
			if !exists {
				t.Fatalf("path %v: missing field %q in %s", path, key, raw)
			}
		case int:
			array, ok := current.([]any)
			if !ok || key < 0 || key >= len(array) {
				t.Fatalf("path %v: index %d is not present in %T", path, key, current)
			}
			current = array[key]
		default:
			t.Fatalf("unsupported path component %T", part)
		}
	}
	array, ok := current.([]any)
	if !ok {
		t.Fatalf("path %v: expected string array, got %T", path, current)
	}
	got := make([]string, len(array))
	for i, item := range array {
		var ok bool
		got[i], ok = item.(string)
		if !ok {
			t.Fatalf("path %v: item %d is %T, want string", path, i, item)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("path %v: got %#v, want %#v", path, got, want)
	}
}

func stringArrayServicePath(tc stringArrayContractCase, method string) []any {
	switch tc.name {
	case "loaded":
		return []any{"data"}
	case "hooks":
		return []any{"data", 0, "warnings"}
	case "marketplace selection", "marketplace roots":
		return []any{tc.field}
	case "reconcile materialization", "reconcile remote":
		return []any{tc.field}
	case "capabilities", "screenshot URLs", "screenshots", "default prompt":
		if method == "plugin/read" {
			return []any{"plugin", "summary", "interface", tc.field}
		}
		return []any{"marketplaces", 0, "plugins", 0, "interface", tc.field}
	case "MCP server names":
		return []any{"plugin", tc.field}
	case "app categories", "app subcategories":
		return []any{"data", 0, "appMetadata", tc.field}
	default:
		return nil
	}
}

func stringArrayItemPath(tc stringArrayContractCase) []any {
	switch tc.name {
	case "citation threads":
		return []any{"memoryCitation", "threadIds"}
	case "collab receivers":
		return []any{"receiverThreadIds"}
	case "question options":
		return []any{"questions", 0, "options"}
	case "web queries":
		return []any{"action", "queries"}
	default:
		return nil
	}
}

func stringArrayCallbackPath(tc stringArrayContractCase) []any {
	if stringArrayItemCase(tc) {
		return append([]any{"item"}, stringArrayItemPath(tc)...)
	}
	if strings.HasPrefix(tc.name, "app ") {
		return []any{"data", 0, "appMetadata", tc.field}
	}
	return []any{tc.field}
}

func TestStringArrayServiceAdmission(t *testing.T) {
	for _, tc := range stringArrayContractCases() {
		methods := []string{}
		switch tc.name {
		case "loaded":
			methods = []string{"thread/loaded/list"}
		case "hooks":
			methods = []string{"hooks/list"}
		case "marketplace selection", "marketplace roots":
			methods = []string{"marketplace/upgrade"}
		case "reconcile materialization", "reconcile remote":
			methods = []string{"plugin/reconcile"}
		case "capabilities", "screenshot URLs", "screenshots", "default prompt":
			methods = []string{"plugin/list", "plugin/installed", "plugin/read"}
		case "MCP server names":
			methods = []string{"plugin/read"}
		case "app categories", "app subcategories":
			methods = []string{"app/list"}
		}
		for _, method := range methods {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				mock := NewMockTransport()
				client := protocol.NewClient(mock)
				t.Cleanup(func() { _ = client.Close() })
				for index, value := range []string{`[null],"` + tc.field + `":[]`, "[" + tc.validItem + "]"} {
					body := strings.Replace(tc.body, "@VALUE@", value, 1)
					switch method {
					case "hooks/list", "app/list":
						body = `{"data":[` + body + `]}`
					case "plugin/list", "plugin/installed", "plugin/read":
						if tc.name != "MCP server names" {
							summary := strings.TrimSuffix(issue74PluginSummary, "}") + `,"interface":` + body + `}`
							body = `{"apps":[],"appTemplates":[],"hooks":[],"marketplaceName":"M","mcpServers":[],"skills":[],"summary":` + summary + `}`
							if method != "plugin/read" {
								body = `{"marketplaces":[{"name":"M","plugins":[` + summary + `]}]}`
							}
						}
						if method == "plugin/read" {
							body = `{"plugin":` + body + `}`
						}
					}
					mock.SetResponse(method, protocol.Response{Result: json.RawMessage(body)})
					var result any
					var err error
					switch method {
					case "thread/loaded/list":
						result, err = client.Thread.LoadedList(context.Background(), protocol.ThreadLoadedListParams{})
					case "hooks/list":
						result, err = client.Hooks.List(context.Background(), protocol.HooksListParams{})
					case "marketplace/upgrade":
						result, err = client.Marketplace.Upgrade(context.Background(), protocol.MarketplaceUpgradeParams{})
					case "plugin/reconcile":
						result, err = client.Plugin.Reconcile(context.Background(), protocol.PluginReconcileParams{})
					case "plugin/list":
						result, err = client.Plugin.List(context.Background(), protocol.PluginListParams{})
					case "plugin/installed":
						result, err = client.Plugin.Installed(context.Background(), protocol.PluginInstalledParams{})
					case "plugin/read":
						result, err = client.Plugin.Read(context.Background(), protocol.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue74String("M")})
					case "app/list":
						result, err = client.Apps.List(context.Background(), protocol.AppsListParams{})
					}
					if (err == nil) != (index == 1) {
						t.Fatalf("step %d result=%+v error=%v", index, result, err)
					}
					if index == 0 && !reflect.ValueOf(result).IsZero() {
						t.Fatalf("rejected response published partial result: %+v", result)
					}
					if index == 1 {
						assertStringArrayAt(t, result, stringArrayExpected(t, tc), stringArrayServicePath(tc, method)...)
					}
				}
			})
		}
	}
}

func TestStringArrayCallbacks(t *testing.T) {
	for _, tc := range stringArrayContractCases() {
		methods := []string{}
		switch {
		case stringArrayItemCase(tc):
			methods = []string{"item/started", "item/completed"}
		case strings.HasPrefix(tc.name, "app "):
			methods = []string{"app/list/updated"}
		case tc.name == "Windows paths":
			methods = []string{"windows/worldWritableWarning"}
		case tc.name == "verification":
			methods = []string{"model/verification"}
		case tc.name == "filesystem paths":
			methods = []string{"fs/changed"}
		}
		for _, method := range methods {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				mock := NewMockTransport()
				replaced, appended, reports := 0, 0, 0
				var replacedValue, appendedValue any
				client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(origin string, err error) {
					if origin != method || err == nil {
						t.Errorf("invalid error origin %q: %v", origin, err)
					}
					reports++
				}))
				t.Cleanup(func() { _ = client.Close() })
				var remove func()
				switch method {
				case "item/started":
					client.OnItemStarted(func(value protocol.ItemStartedNotification) { replaced++; replacedValue = value })
					remove = client.AddItemStartedListener(func(value protocol.ItemStartedNotification) { appended++; appendedValue = value })
				case "item/completed":
					client.OnItemCompleted(func(value protocol.ItemCompletedNotification) { replaced++; replacedValue = value })
					remove = client.AddItemCompletedListener(func(value protocol.ItemCompletedNotification) { appended++; appendedValue = value })
				case "app/list/updated":
					client.OnAppListUpdated(func(value protocol.AppListUpdatedNotification) { replaced++; replacedValue = value })
					remove = client.AddAppListUpdatedListener(func(value protocol.AppListUpdatedNotification) { appended++; appendedValue = value })
				case "windows/worldWritableWarning":
					client.OnWindowsWorldWritableWarning(func(value protocol.WindowsWorldWritableWarningNotification) { replaced++; replacedValue = value })
					remove = client.AddWindowsWorldWritableWarningListener(func(value protocol.WindowsWorldWritableWarningNotification) { appended++; appendedValue = value })
				case "model/verification":
					client.OnModelVerification(func(value protocol.ModelVerificationNotification) { replaced++; replacedValue = value })
					remove = client.AddModelVerificationListener(func(value protocol.ModelVerificationNotification) { appended++; appendedValue = value })
				case "fs/changed":
					client.OnFsChanged(func(value protocol.FsChangedNotification) { replaced++; replacedValue = value })
					remove = client.AddFsChangedListener(func(value protocol.FsChangedNotification) { appended++; appendedValue = value })
				}
				defer remove()
				for index, value := range []string{`[null],"` + tc.field + `":[]`, "[" + tc.validItem + "]"} {
					body := strings.Replace(tc.body, "@VALUE@", value, 1)
					if stringArrayItemCase(tc) {
						body = `{"threadId":"array-thread","turnId":"turn","startedAtMs":0,"completedAtMs":0,"item":` + stringArrayItem(tc, value) + `}`
					} else if strings.HasPrefix(tc.name, "app ") {
						body = `{"data":[` + body + `]}`
					}
					mock.InjectServerNotification(context.Background(), protocol.Notification{Method: method, Params: json.RawMessage(body)})
					if replaced != index || appended != index || reports != 2 {
						t.Fatalf("step %d: replacement=%d appended=%d errors=%d", index, replaced, appended, reports)
					}
					if index == 1 {
						want := stringArrayExpected(t, tc)
						path := stringArrayCallbackPath(tc)
						assertStringArrayAt(t, replacedValue, want, path...)
						assertStringArrayAt(t, appendedValue, want, path...)
					}
				}
			})
		}
	}
}

func TestStringArrayPersistedItemAdmission(t *testing.T) {
	for _, tc := range stringArrayContractCases() {
		if !stringArrayItemCase(tc) {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := protocol.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			prior := protocol.Thread{ID: "array-thread", Preview: "prior"}
			client.CacheThreadState(prior)
			updates := 0
			remove := client.AddThreadStateListener(prior.ID, func(protocol.Thread) { updates++ }, nil)
			defer remove()
			initial := updates
			for index, value := range []string{`[null],"` + tc.field + `":[]`, "[" + tc.validItem + "]"} {
				thread := validProcessThreadPayload(prior.ID)
				thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": []json.RawMessage{json.RawMessage(stringArrayItem(tc, value))}}}
				if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
					t.Fatal(err)
				}
				result, err := client.Thread.Read(context.Background(), protocol.ThreadReadParams{ThreadID: prior.ID})
				state, ok := client.ThreadStateSnapshot(prior.ID)
				if index == 0 {
					if err == nil || result.Thread.ID != "" || !ok || !reflect.DeepEqual(prior, state) || updates != initial {
						t.Fatalf("invalid item published: result=%+v state=%+v updates=%d error=%v", result, state, updates, err)
					}
				} else if err != nil || !ok || len(state.Turns) != 1 || updates != initial+1 || result.Thread.Turns[0].Items[0].Value == nil {
					t.Fatalf("valid recovery failed: result=%+v state=%+v updates=%d error=%v", result, state, updates, err)
				} else {
					want := stringArrayExpected(t, tc)
					resultPath := append([]any{"thread", "turns", 0, "items", 0}, stringArrayItemPath(tc)...)
					statePath := append([]any{"turns", 0, "items", 0}, stringArrayItemPath(tc)...)
					assertStringArrayAt(t, result, want, resultPath...)
					assertStringArrayAt(t, state, want, statePath...)
				}
			}
		})
	}
}
