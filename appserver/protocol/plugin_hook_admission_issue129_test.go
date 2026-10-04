package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func issue129PluginRead(t *testing.T, hooks string) (codex.PluginReadResponse, error) {
	return issue129PluginReadFields(t, `"hooks":`+hooks)
}

func TestPluginReadHookDuplicateHistories(t *testing.T) {
	tests := []struct {
		name       string
		hookFields string
		wantErr    bool
		wantCount  int
		wantEvent  codex.HookEventName
		wantKey    string
	}{
		{
			name:       "partial duplicate fills missing key",
			hookFields: `"hooks":[{"eventName":"sessionStart"}],"HOOKS":[{"key":"hook-key"}]`,
			wantCount:  1,
			wantEvent:  codex.HookEventNameSessionStart,
			wantKey:    "hook-key",
		},
		{
			name:       "partial duplicate fills missing event name",
			hookFields: `"hooks":[{"key":"hook-key"}],"HOOKS":[{"eventName":"sessionStart"}]`,
			wantCount:  1,
			wantEvent:  codex.HookEventNameSessionStart,
			wantKey:    "hook-key",
		},
		{
			name:       "shrink and reextend preserves hidden fields",
			hookFields: `"hooks":[{"eventName":"sessionStart","key":"zero"},{"eventName":"preToolUse","key":"one"},{"eventName":"stop","key":"two"}],"hooks":[{"key":"zero-updated"}],"HOOKS":[{"eventName":"sessionEnd"},{"eventName":"postToolUse"},{"eventName":"interrupt"}]`,
			wantCount:  3,
			wantEvent:  codex.HookEventNameSessionEnd,
			wantKey:    "zero-updated",
		},
		{
			name:       "empty array resets prior presence",
			hookFields: `"hooks":[{"eventName":"sessionStart","key":"old"}],"hooks":[],"hooks":[{"key":"new"}]`,
			wantErr:    true,
		},
		{
			name:       "null array then valid duplicate remains invalid",
			hookFields: `"hooks":[{"eventName":"sessionStart","key":"old"}],"hooks":null,"hooks":[{"eventName":"sessionEnd","key":"new"}]`,
			wantErr:    true,
		},
		{
			name:       "null item then valid duplicate remains invalid",
			hookFields: `"hooks":[null],"hooks":[{"eventName":"sessionStart","key":"new"}]`,
			wantErr:    true,
		},
		{
			name:       "null key then repair remains invalid",
			hookFields: `"hooks":[{"eventName":"sessionStart","key":null}],"hooks":[{"key":"repaired"}]`,
			wantErr:    true,
		},
		{
			name:       "escaped and folded names",
			hookFields: `"HOOKS":[{"EVENTNAME":"sessionStart","KEY":""}],"hook\u017F":[{"eventN\u0061me":"stop","\u212aEY":""}]`,
			wantCount:  1,
			wantEvent:  codex.HookEventNameStop,
			wantKey:    "",
		},
		{
			name:       "final null preserves existing missing field error",
			hookFields: `"hooks":[{"eventName":"sessionStart","key":"old"}],"hooks":null`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var direct codex.PluginDetail
			directErr := json.Unmarshal([]byte(issue129PluginObject(tt.hookFields)), &direct)
			public, publicErr := issue129PluginReadFields(t, tt.hookFields)
			if tt.wantErr {
				if directErr == nil || publicErr == nil {
					t.Fatalf("want direct and public rejection, got direct=%v public=%v", directErr, publicErr)
				}
				if tt.name == "final null preserves existing missing field error" && !strings.Contains(directErr.Error(), "missing plugin.hooks") {
					t.Fatalf("direct final-null error = %v; want existing missing plugin.hooks error", directErr)
				}
				return
			}
			if directErr != nil || publicErr != nil {
				t.Fatalf("decode failed: direct=%v public=%v", directErr, publicErr)
			}
			for name, hooks := range map[string][]codex.PluginHookSummary{"direct": direct.Hooks, "public": public.Plugin.Hooks} {
				if len(hooks) != tt.wantCount {
					t.Errorf("%s hooks count=%d; want %d", name, len(hooks), tt.wantCount)
					continue
				}
				if tt.wantCount != 0 && (hooks[0].EventName != tt.wantEvent || hooks[0].Key != tt.wantKey) {
					t.Errorf("%s hook=%+v; want event=%q key=%q", name, hooks[0], tt.wantEvent, tt.wantKey)
				}
			}
		})
	}
}

type issue129NativeHookPresence struct {
	EventName *string `json:"eventName"`
	Key       *string `json:"key"`
}

type issue129NativeHookPresenceEnvelope struct {
	Hooks *[]issue129NativeHookPresence `json:"hooks"`
}

type issue129NativeHookValue struct {
	EventName string `json:"eventName"`
	Key       string `json:"key"`
}

type issue129NativeHookValueEnvelope struct {
	Hooks *[]issue129NativeHookValue `json:"hooks"`
}

func TestPluginReadGeneratedHookPresenceMatchesIndependentModel(t *testing.T) {
	rootNames := []string{`"hooks"`, `"HOOKS"`, `"hook\u017F"`, `"hoo\u212As"`}
	eventNames := []string{"eventName", "EVENTNAME", `eventN\u0061me`}
	keyNames := []string{"key", "KEY", `\u212aEY`}
	quoted := func(value string) string {
		data, _ := json.Marshal(value)
		return string(data)
	}
	arrayField := func(arrayIndex int, items []string) string {
		return rootNames[arrayIndex%len(rootNames)] + `:[` + strings.Join(items, ",") + "]"
	}
	completeItem := func(index, variant int) string {
		event := "sessionStart"
		if index%2 == 1 {
			event = "stop"
		}
		key := fmt.Sprintf("hook-%d", index)
		if index == 0 {
			key = ""
		}
		return `{"` + eventNames[variant%len(eventNames)] + `":` + quoted(event) + `,"` + keyNames[variant%len(keyNames)] + `":` + quoted(key) + `}`
	}
	partialItem := func(index, variant int) string {
		if (index+variant)%2 == 0 {
			event := "sessionStart"
			if index%2 == 1 {
				event = "stop"
			}
			return `{"` + eventNames[variant%len(eventNames)] + `":` + quoted(event) + `}`
		}
		key := fmt.Sprintf("updated-%d", index)
		if index == 0 {
			key = ""
		}
		return `{"` + keyNames[variant%len(keyNames)] + `":` + quoted(key) + `}`
	}

	accepted, rejected, populatedAccepted, emptyKeyAccepted := 0, 0, 0, 0
	incompleteRejected := 0
	for initialLength := 1; initialLength <= 3; initialLength++ {
		for middleLength := -1; middleLength <= 3; middleLength++ {
			for finalLength := 0; finalLength <= 3; finalLength++ {
				for _, reset := range []bool{false, true} {
					for event := 0; event < 6; event++ {
						initial := make([]string, initialLength)
						for i := range initial {
							initial[i] = completeItem(i, i)
						}
						if event == 2 {
							initial[0] = "null"
						}
						if event == 4 {
							initial[0] = `{"key":"hook-0"}`
						}
						if event == 5 {
							initial[0] = `{"eventName":"sessionStart"}`
						}
						fields := []string{arrayField(0, initial)}
						if event == 1 {
							fields = append(fields, `"HOOKS":null`)
						}
						if event == 3 {
							fields = append(fields, arrayField(1, []string{`{"key":null}`}))
						}

						priorSlots := initialLength
						if event == 1 {
							priorSlots = 0
						}
						if middleLength >= 0 {
							middle := make([]string, middleLength)
							for i := range middle {
								if i < priorSlots {
									middle[i] = partialItem(i, i+1)
									if i == 0 && event == 4 {
										middle[i] = `{"key":"hook-0"}`
									}
									if i == 0 && event == 5 {
										middle[i] = `{"eventName":"sessionStart"}`
									}
								} else {
									middle[i] = completeItem(i, i+1)
								}
							}
							fields = append(fields, arrayField(len(fields), middle))
							if middleLength == 0 {
								priorSlots = 0
							} else if middleLength > priorSlots {
								priorSlots = middleLength
							}
						}
						if reset {
							fields = append(fields, arrayField(len(fields), nil))
							priorSlots = 0
						}
						final := make([]string, finalLength)
						for i := range final {
							if i < priorSlots {
								final[i] = partialItem(i, i+len(fields))
								if i == 0 && event == 4 {
									final[i] = `{"key":"hook-0"}`
								}
								if i == 0 && event == 5 {
									final[i] = `{"eventName":"sessionStart"}`
								}
							} else {
								final[i] = completeItem(i, i+len(fields))
							}
						}
						fields = append(fields, arrayField(len(fields), final))
						hookFields := strings.Join(fields, ",")
						payload := []byte(issue129PluginObject(hookFields))

						var presence issue129NativeHookPresenceEnvelope
						if err := json.Unmarshal(payload, &presence); err != nil {
							t.Fatalf("native presence reference decode: %v", err)
						}
						var values issue129NativeHookValueEnvelope
						if err := json.Unmarshal(payload, &values); err != nil {
							t.Fatalf("native value reference decode: %v", err)
						}
						stickyEvent := event >= 1 && event <= 3
						wantAccept := !stickyEvent && presence.Hooks != nil
						if wantAccept {
							for _, hook := range *presence.Hooks {
								if hook.EventName == nil || hook.Key == nil {
									wantAccept = false
									break
								}
							}
						}
						if wantAccept {
							accepted++
							if len(*presence.Hooks) > 0 {
								populatedAccepted++
								if (*presence.Hooks)[0].Key != nil && *(*presence.Hooks)[0].Key == "" {
									emptyKeyAccepted++
								}
							}
						} else {
							rejected++
							if event == 4 || event == 5 {
								incompleteRejected++
							}
						}

						name := fmt.Sprintf("initial-%d-middle-%d-final-%d-reset-%t-event-%d", initialLength, middleLength, finalLength, reset, event)
						t.Run(name, func(t *testing.T) {
							var direct codex.PluginDetail
							directErr := json.Unmarshal(payload, &direct)
							public, publicErr := issue129PluginReadFields(t, hookFields)
							if (directErr == nil) != wantAccept || (publicErr == nil) != wantAccept {
								t.Fatalf("acceptance direct=%v public=%v want=%t fields=%s", directErr, publicErr, wantAccept, hookFields)
							}
							if !wantAccept {
								if len(public.Plugin.Hooks) != 0 {
									t.Fatalf("rejected public response published %d hooks", len(public.Plugin.Hooks))
								}
								return
							}
							if values.Hooks == nil || len(direct.Hooks) != len(*values.Hooks) || len(public.Plugin.Hooks) != len(*values.Hooks) {
								t.Fatalf("accepted hook lengths direct=%d public=%d values=%v", len(direct.Hooks), len(public.Plugin.Hooks), values.Hooks)
							}
							for i, want := range *values.Hooks {
								if direct.Hooks[i].EventName != codex.HookEventName(want.EventName) || direct.Hooks[i].Key != want.Key {
									t.Errorf("direct hook %d=%+v; want native value %+v", i, direct.Hooks[i], want)
								}
								if public.Plugin.Hooks[i].EventName != codex.HookEventName(want.EventName) || public.Plugin.Hooks[i].Key != want.Key {
									t.Errorf("public hook %d=%+v; want native value %+v", i, public.Plugin.Hooks[i], want)
								}
							}
						})
					}
				}
			}
		}
	}
	if accepted == 0 || rejected == 0 || incompleteRejected == 0 || populatedAccepted == 0 || emptyKeyAccepted == 0 {
		t.Fatalf("generated reference covered accepted=%d rejected=%d incomplete-rejected=%d populated=%d populated-empty-key=%d cases", accepted, rejected, incompleteRejected, populatedAccepted, emptyKeyAccepted)
	}
}

func TestPluginDetailHookAdmissionPreservesSeededReceiverOnNewFailure(t *testing.T) {
	oldDescription := "seeded description"
	oldPath := "/seeded/plugin"
	oldHooks := []codex.PluginHookSummary{{EventName: codex.HookEventNameStop, Key: "seeded-hook"}}
	oldApps := []codex.AppSummary{{ID: "seeded-app"}}
	detail := codex.PluginDetail{
		Apps:            oldApps,
		Description:     &oldDescription,
		Hooks:           oldHooks,
		MarketplaceName: "seeded-marketplace",
		MarketplacePath: &oldPath,
		McpServers:      []string{"seeded-mcp"},
	}
	before := detail
	oldHookSlot := &detail.Hooks[0]
	oldAppSlot := &detail.Apps[0]
	oldDescriptionRef := detail.Description
	oldPathRef := detail.MarketplacePath

	err := json.Unmarshal([]byte(issue129PluginObject(`"hooks":[{"key":"missing-event"}]`)), &detail)
	if err == nil || !strings.Contains(err.Error(), "eventName is required") {
		t.Fatalf("decode error = %v; want new required-eventName error", err)
	}
	if !reflect.DeepEqual(detail, before) {
		t.Fatalf("new admission failure changed receiver:\n got %#v\nwant %#v", detail, before)
	}
	if len(detail.Hooks) != 1 || detail.Hooks[0].EventName != codex.HookEventNameStop || detail.Hooks[0].Key != "seeded-hook" {
		t.Fatalf("seeded hooks changed: %+v", detail.Hooks)
	}
	if len(detail.Apps) != 1 || detail.Apps[0].ID != "seeded-app" {
		t.Fatalf("seeded apps changed: %+v", detail.Apps)
	}
	if detail.Description == nil || *detail.Description != "seeded description" || detail.MarketplacePath == nil || *detail.MarketplacePath != "/seeded/plugin" || detail.MarketplaceName != "seeded-marketplace" || len(detail.McpServers) != 1 || detail.McpServers[0] != "seeded-mcp" {
		t.Fatalf("seeded scalar/reference values changed: %+v", detail)
	}
	if &detail.Hooks[0] != oldHookSlot || &detail.Apps[0] != oldAppSlot || detail.Description != oldDescriptionRef || detail.MarketplacePath != oldPathRef {
		t.Fatal("new admission failure replaced seeded receiver references")
	}
}

func TestPluginReadHookAdmissionPrecedesSummaryIDValidation(t *testing.T) {
	plugin := strings.Replace(issue129PluginObject(`"hooks":[{}]`), `"id":"plugin-129"`, `"id":""`, 1)
	transport := NewMockTransport()
	transport.SetResponse("plugin/read", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"plugin":` + plugin + `}`)})
	client := codex.NewClient(transport)
	_, err := client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue129StringPointer("official")})
	if err == nil || !strings.Contains(err.Error(), "eventName is required") {
		t.Fatalf("Plugin.Read error = %v; want hook admission before the later empty summary ID response check", err)
	}
}

func TestPluginDetailHookAdmissionRetainsExistingErrorPrecedence(t *testing.T) {
	t.Run("missing plugin field precedes hook admission", func(t *testing.T) {
		payload := strings.Replace(issue129PluginObject(`"hooks":[{"eventName":"sessionStart"}]`), `"appTemplates":[],`, "", 1)
		var detail codex.PluginDetail
		err := json.Unmarshal([]byte(payload), &detail)
		if err == nil || !strings.Contains(err.Error(), "missing plugin.appTemplates") {
			t.Fatalf("error = %v; want existing missing plugin.appTemplates error", err)
		}
	})
	t.Run("mcp server element validation precedes hook admission", func(t *testing.T) {
		payload := strings.Replace(issue129PluginObject(`"hooks":[{"eventName":"sessionStart"}]`), `"mcpServers":[]`, `"mcpServers":[null]`, 1)
		var detail codex.PluginDetail
		err := json.Unmarshal([]byte(payload), &detail)
		if err == nil || !strings.Contains(err.Error(), "string at index 0 must not be null") {
			t.Fatalf("error = %v; want existing mcpServers null-element error", err)
		}
	})
	t.Run("path error retains historical staged receiver update", func(t *testing.T) {
		payload := strings.Replace(issue129PluginObject(`"hooks":[{"eventName":"sessionStart","key":"new-hook"}]`), `,"marketplaceName":"official"`, `,"marketplacePath":"relative","marketplaceName":"official"`, 1)
		oldPath := "/old/path"
		detail := codex.PluginDetail{
			Hooks:           []codex.PluginHookSummary{{EventName: codex.HookEventNameStop, Key: "old-hook"}},
			MarketplaceName: "old-marketplace",
			MarketplacePath: &oldPath,
		}
		err := json.Unmarshal([]byte(payload), &detail)
		if err == nil || !strings.Contains(err.Error(), "marketplacePath") {
			t.Fatalf("error = %v; want existing marketplacePath error", err)
		}
		if detail.MarketplaceName != "official" || len(detail.Hooks) != 1 || detail.Hooks[0].Key != "new-hook" || detail.MarketplacePath != &oldPath {
			t.Fatalf("path failure no longer exposes its established stage: %+v", detail)
		}
	})
}

func TestPluginHookSummaryRemainsPlainAndEmbeddable(t *testing.T) {
	typeOf := reflect.TypeOf(codex.PluginHookSummary{})
	if typeOf.NumMethod() != 0 || reflect.TypeOf((*codex.PluginHookSummary)(nil)).NumMethod() != 0 {
		t.Fatal("PluginHookSummary acquired a JSON method")
	}
	type envelope struct {
		codex.PluginHookSummary
		Sibling string `json:"sibling"`
	}
	var wrapped envelope
	if err := json.Unmarshal([]byte(`{"eventName":"sessionStart","key":"embedded","sibling":"kept"}`), &wrapped); err != nil {
		t.Fatalf("anonymous envelope decode: %v", err)
	}
	if wrapped.EventName != codex.HookEventNameSessionStart || wrapped.Key != "embedded" || wrapped.Sibling != "kept" {
		t.Fatalf("anonymous envelope = %+v", wrapped)
	}
	encoded, err := json.Marshal(codex.PluginHookSummary{EventName: codex.HookEventNameSessionStart, Key: ""})
	if err != nil {
		t.Fatalf("constructed valid summary marshal: %v", err)
	}
	if string(encoded) != `{"eventName":"sessionStart","key":""}` {
		t.Fatalf("constructed valid summary JSON = %s", encoded)
	}
}

func issue129PluginReadFields(t *testing.T, hookFields string) (codex.PluginReadResponse, error) {
	t.Helper()
	result := json.RawMessage(`{"plugin":` + issue129PluginObject(hookFields) + `}`)
	transport := NewMockTransport()
	transport.SetResponse("plugin/read", codex.Response{JSONRPC: "2.0", Result: result})
	client := codex.NewClient(transport)
	return client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue129StringPointer("official")})
}

func issue129PluginObject(hookFields string) string {
	return `{"appTemplates":[],"apps":[],` + hookFields + `,"marketplaceName":"official","mcpServers":[],"skills":[],"summary":{"authPolicy":"ON_USE","enabled":true,"id":"plugin-129","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"remote"}}}`
}

func issue129StringPointer(value string) *string { return &value }

func TestPluginReadRequiresValidHookSummaryRecords(t *testing.T) {
	tests := []struct {
		name      string
		hooks     string
		wantErr   bool
		wantCount int
		wantEvent codex.HookEventName
		wantKey   string
	}{
		{name: "missing both required fields", hooks: `[{}]`, wantErr: true},
		{name: "missing event name", hooks: `[{"key":"hook-key"}]`, wantErr: true},
		{name: "missing key", hooks: `[{"eventName":"sessionStart"}]`, wantErr: true},
		{name: "null hook item", hooks: `[null]`, wantErr: true},
		{name: "null key", hooks: `[{"eventName":"sessionStart","key":null}]`, wantErr: true},
		{name: "null hooks followed by valid duplicate remains invalid", hooks: `null,"hooks":[{"eventName":"sessionStart","key":"hook-key"}]`, wantErr: true},
		{name: "final null hooks retains missing field error", hooks: `null`, wantErr: true},
		{name: "empty key and known event are valid", hooks: `[{"eventName":"sessionStart","key":""}]`, wantCount: 1, wantEvent: codex.HookEventNameSessionStart},
		{name: "empty hooks array is valid", hooks: `[]`, wantCount: 0},
		{name: "null event name remains invalid", hooks: `[{"eventName":null,"key":"hook-key"}]`, wantErr: true},
		{name: "unknown event name remains invalid", hooks: `[{"eventName":"notAnEvent","key":"hook-key"}]`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := issue129PluginRead(t, tt.hooks)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Plugin.Read accepted hooks %s; want a schema validation error", tt.hooks)
				}
				return
			}
			if err != nil {
				t.Fatalf("Plugin.Read rejected valid hooks %s: %v", tt.hooks, err)
			}
			if len(response.Plugin.Hooks) != tt.wantCount {
				t.Fatalf("hooks count = %d; want %d", len(response.Plugin.Hooks), tt.wantCount)
			}
			if tt.wantCount > 0 && (response.Plugin.Hooks[0].EventName != tt.wantEvent || response.Plugin.Hooks[0].Key != tt.wantKey) {
				t.Fatalf("hook = %+v; want eventName=%q key=%q", response.Plugin.Hooks[0], tt.wantEvent, tt.wantKey)
			}
		})
	}
}
