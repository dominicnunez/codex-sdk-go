package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestHookRunSummaryRejectsInvalidSourceAndSourcePath(t *testing.T) {
	for _, method := range []string{"hook/started", "hook/completed"} {
		for _, registration := range []string{"On", "Add"} {
			for _, field := range []struct {
				name, value, want string
			}{
				{"source", `"bogus"`, `invalid hook.source`},
				{"source", `""`, `invalid hook.source`},
				{"source", `null`, `hook.source`},
				{"sourcePath", `"relative/hook"`, `must be an absolute path`},
				{"sourcePath", `""`, `must be an absolute path`},
				{"sourcePath", `"/tmp/a/../hook"`, `must be normalized`},
			} {
				t.Run(method+"/"+registration+"/"+field.name+"/"+strings.Trim(field.value, `"`), func(t *testing.T) {
					transport := NewMockTransport()
					var callbackCalled bool
					var handlerError error
					client := codex.NewClient(transport, codex.WithHandlerErrorCallback(func(_ string, err error) { handlerError = err }))
					t.Cleanup(func() { _ = client.Close() })
					var unsubscribe func()
					t.Cleanup(func() {
						if unsubscribe != nil {
							unsubscribe()
						}
					})
					handler := func(codex.HookStartedNotification) { callbackCalled = true }
					if method == "hook/completed" {
						if registration == "On" {
							client.OnHookCompleted(func(codex.HookCompletedNotification) { callbackCalled = true })
						} else {
							unsubscribe = client.AddHookCompletedListener(func(codex.HookCompletedNotification) { callbackCalled = true })
						}
					} else if registration == "On" {
						client.OnHookStarted(handler)
					} else {
						unsubscribe = client.AddHookStartedListener(handler)
					}

					runField := `"` + field.name + `":` + field.value
					if field.name == "source" {
						runField = `"sourcePath":"/tmp/hook",` + runField
					} else {
						runField = `"source":"user",` + runField
					}
					params := `{"run":{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"run-1","scope":"thread",` + runField + `,"startedAt":123,"status":"completed"},"threadId":"thread-1"}`
					transport.InjectServerNotification(context.Background(), codex.Notification{JSONRPC: "2.0", Method: method, Params: json.RawMessage(params)})
					if callbackCalled {
						t.Fatal("typed hook callback ran for invalid run summary")
					}
					if handlerError == nil || !strings.Contains(handlerError.Error(), field.want) {
						t.Fatalf("handler error = %v; want substring %q", handlerError, field.want)
					}
				})
			}
		}
	}
}

func TestHooksListRejectsInvalidMetadataSource(t *testing.T) {
	for _, source := range []string{`"bogus"`, `""`, `null`} {
		t.Run(source, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"data":[{"cwd":"/tmp","errors":[],"hooks":[{"currentHash":"hash","displayOrder":1,"enabled":true,"eventName":"sessionStart","handlerType":"command","command":"hook","isManaged":false,"key":"hook","source":` + source + `,"sourcePath":"/tmp/hook","timeoutSec":1,"trustStatus":"trusted"}],"warnings":[]}]}`
			if err := transport.SetResponseData("hooks/list", json.RawMessage(payload)); err != nil {
				t.Fatal(err)
			}
			_, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
			if err == nil {
				t.Fatal("Hooks.List accepted invalid HookMetadata source")
			}
		})
	}
}

func TestHooksListRejectsErrorRecordMissingRequiredFields(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	payload := `{"data":[{"cwd":"/tmp","errors":[{}],"hooks":[],"warnings":[]}]}`
	if err := transport.SetResponseData("hooks/list", json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
	_, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
	if err == nil || !strings.Contains(err.Error(), "message") {
		t.Fatalf("Hooks.List error = %v; want missing HookErrorInfo.message rejection", err)
	}
}

func TestHooksListHookErrorInfoAdmission(t *testing.T) {
	tests := []struct {
		name, fields, wantError string
		want                    []codex.HookErrorInfo
		wantNonNilEmpty         bool
	}{
		{
			name:   "empty values and opaque relative path are valid",
			fields: `"errors":[{"message":"","path":"relative/path"}]`,
			want:   []codex.HookErrorInfo{{Message: "", Path: "relative/path"}},
		},
		{
			name:   "both empty required strings are present",
			fields: `"errors":[{"message":"","path":""}]`,
			want:   []codex.HookErrorInfo{{Message: "", Path: ""}},
		},
		{
			name:      "missing message",
			fields:    `"errors":[{"path":"relative"}]`,
			wantError: "hook.errors[0].message is required",
		},
		{
			name:      "missing path",
			fields:    `"errors":[{"message":"present"}]`,
			wantError: "hook.errors[0].path is required",
		},
		{
			name:      "null item",
			fields:    `"errors":[null]`,
			wantError: "hook.errors[0] must not be null",
		},
		{
			name:      "null message",
			fields:    `"errors":[{"message":null,"path":"opaque"}]`,
			wantError: "hook.errors[0].message must not be null",
		},
		{
			name:      "escaped field null remains forbidden",
			fields:    `"errors":[{"me\u0073sage":null,"path":"opaque"}]`,
			wantError: "hook.errors[0].message must not be null",
		},
		{
			name:      "null path",
			fields:    `"errors":[{"message":"present","path":null}]`,
			wantError: "hook.errors[0].path must not be null",
		},
		{
			name:   "duplicate arrays complete the same item",
			fields: `"errors":[{"message":"merged"}],"errors":[{"path":"relative"}]`,
			want:   []codex.HookErrorInfo{{Message: "merged", Path: "relative"}},
		},
		{
			name:      "array item null remains invalid after repair",
			fields:    `"errors":[null],"errors":[{"message":"repaired","path":"opaque"}]`,
			wantError: "hook.errors[0] must not be null",
		},
		{
			name:      "scalar null remains invalid after repair",
			fields:    `"errors":[{"message":null,"path":"opaque"}],"errors":[{"message":"repaired","path":"opaque"}]`,
			wantError: "hook.errors[0].message must not be null",
		},
		{
			name:      "empty array resets old presence masks",
			fields:    `"errors":[{"message":"old","path":"old"}],"errors":[],"errors":[{"message":"new"}]`,
			wantError: "hook.errors[0].path is required",
		},
		{
			name:   "shrink and reextend reuse hidden slot fields",
			fields: `"errors":[{"message":"first","path":"first"},{"message":"old","path":"retained"}],"errors":[{"message":"middle","path":"middle"}],"errors":[{"message":"final","path":"final"},{"message":"reused"}]`,
			want:   []codex.HookErrorInfo{{Message: "final", Path: "final"}, {Message: "reused", Path: "retained"}},
		},
		{
			name:   "truncated partial item is no longer visible",
			fields: `"errors":[{"message":"first","path":"first"},{"message":"hidden"}],"errors":[{"message":"visible","path":"visible"}]`,
			want:   []codex.HookErrorInfo{{Message: "visible", Path: "visible"}},
		},
		{
			name:            "final empty array remains empty",
			fields:          `"errors":[{"message":"old","path":"old"}],"errors":[]`,
			want:            []codex.HookErrorInfo{},
			wantNonNilEmpty: true,
		},
		{
			name:   "folded escaped and Unicode field names match stdlib",
			fields: `"errors":[{"me\u017Fsage":"unicode","pa\u0074h":"opaque"}]`,
			want:   []codex.HookErrorInfo{{Message: "unicode", Path: "opaque"}},
		},
		{
			name:   "folded root alias remains unknown",
			fields: `"ERRORS":[{}],"errors":[{"message":"exact","path":"opaque"}]`,
			want:   []codex.HookErrorInfo{{Message: "exact", Path: "opaque"}},
		},
		{
			name:   "escaped root field matches exactly",
			fields: `"err\u006frs":[{"message":"escaped","path":"opaque"}]`,
			want:   []codex.HookErrorInfo{{Message: "escaped", Path: "opaque"}},
		},
		{
			name:      "root null array remains invalid after repair",
			fields:    `"errors":null,"errors":[{"message":"repaired","path":"opaque"}]`,
			wantError: `required field "errors" must not be null`,
		},
		{
			name:      "wrong-kind root array keeps native type error",
			fields:    `"errors":{},"errors":[{"message":"repaired","path":"opaque"}]`,
			wantError: "cannot unmarshal object",
		},
		{
			name:      "native type error precedes null admission",
			fields:    `"errors":[{"message":null,"path":3}]`,
			wantError: "cannot unmarshal number",
		},
		{
			name:      "existing warnings failure precedes missing error fields",
			fields:    `"errors":[{}],"warnings":[null]`,
			wantError: "warnings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"data":[{"cwd":"/tmp",` + tt.fields + `,"hooks":[],"warnings":[]}]}`
			if err := transport.SetResponseData("hooks/list", json.RawMessage(payload)); err != nil {
				t.Fatal(err)
			}
			resp, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("Hooks.List error = %v; want substring %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Hooks.List error = %v", err)
			}
			if len(resp.Data) != 1 || !reflect.DeepEqual(resp.Data[0].Errors, tt.want) {
				t.Fatalf("HookErrorInfo values = %#v; want %#v", resp.Data, tt.want)
			}
			if tt.wantNonNilEmpty && resp.Data[0].Errors == nil {
				t.Fatal("empty errors array became nil")
			}
		})
	}
}

type anonymousHookErrorEnvelope struct {
	Before string `json:"before"`
	codex.HookErrorInfo
	After string `json:"after"`
}

func TestHookErrorInfoRemainsPlainAndEntryFailureDoesNotMutateReceiver(t *testing.T) {
	if reflect.TypeOf(codex.HookErrorInfo{}).NumMethod() != 0 || reflect.TypeOf((*codex.HookErrorInfo)(nil)).NumMethod() != 0 {
		t.Fatal("HookErrorInfo acquired public methods")
	}
	var plain codex.HookErrorInfo
	if err := json.Unmarshal([]byte(`{}`), &plain); err != nil || plain != (codex.HookErrorInfo{}) {
		t.Fatalf("plain HookErrorInfo behavior changed: value=%+v err=%v", plain, err)
	}
	var envelope anonymousHookErrorEnvelope
	if err := json.Unmarshal([]byte(`{"before":"left","message":"m","path":"p","after":"right"}`), &envelope); err != nil {
		t.Fatalf("anonymous HookErrorInfo envelope decode failed: %v", err)
	}
	if envelope.Before != "left" || envelope.Message != "m" || envelope.Path != "p" || envelope.After != "right" {
		t.Fatalf("anonymous HookErrorInfo envelope = %+v", envelope)
	}

	entry := codex.HooksListEntry{
		Cwd:      "/before",
		Errors:   []codex.HookErrorInfo{{Message: "before", Path: "opaque"}},
		Hooks:    []codex.HookMetadata{},
		Warnings: []string{"before"},
	}
	wantBefore := codex.HooksListEntry{
		Cwd:      "/before",
		Errors:   []codex.HookErrorInfo{{Message: "before", Path: "opaque"}},
		Hooks:    []codex.HookMetadata{},
		Warnings: []string{"before"},
	}
	errorsSlot := &entry.Errors[0]
	if err := json.Unmarshal([]byte(`{"cwd":"/after","errors":[{}],"hooks":[],"warnings":[]}`), &entry); err == nil {
		t.Fatal("HooksListEntry accepted incomplete HookErrorInfo")
	}
	if !reflect.DeepEqual(entry, wantBefore) || &entry.Errors[0] != errorsSlot {
		t.Fatalf("receiver or prior slice storage changed after failure: got %+v want %+v", entry, wantBefore)
	}
	if err := json.Unmarshal([]byte(`{"cwd":"/recovered","errors":[{"message":"ok","path":"relative"}],"hooks":[],"warnings":[]}`), &entry); err != nil {
		t.Fatalf("valid receiver recovery failed: %v", err)
	}
	if entry.Cwd != "/recovered" || !reflect.DeepEqual(entry.Errors, []codex.HookErrorInfo{{Message: "ok", Path: "relative"}}) {
		t.Fatalf("recovered HooksListEntry = %+v", entry)
	}
}

func TestHooksListAcceptsEveryKnownMetadataSource(t *testing.T) {
	sources := []string{
		"system", "user", "project", "mdm", "sessionFlags", "plugin",
		"cloudRequirements", "cloudManagedConfig", "legacyManagedConfigFile",
		"legacyManagedConfigMdm", "unknown",
	}
	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			payload := `{"data":[{"cwd":"/tmp","errors":[],"hooks":[{"currentHash":"hash","displayOrder":1,"enabled":true,"eventName":"sessionStart","handlerType":"command","command":"hook","isManaged":false,"key":"hook","source":"` + source + `","sourcePath":"/tmp/hook","timeoutSec":1,"trustStatus":"trusted"}],"warnings":[]}]}`
			if err := transport.SetResponseData("hooks/list", json.RawMessage(payload)); err != nil {
				t.Fatal(err)
			}
			resp, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
			if err != nil {
				t.Fatalf("Hooks.List error = %v; valid source %q should pass", err, source)
			}
			if got := resp.Data[0].Hooks[0].Source; string(got) != source {
				t.Fatalf("source = %q; want %q", got, source)
			}
		})
	}
}

func TestHookRunSummaryAcceptsSourcesAndNormalizedAbsolutePaths(t *testing.T) {
	sources := []string{
		"system", "user", "project", "mdm", "sessionFlags", "plugin",
		"cloudRequirements", "cloudManagedConfig", "legacyManagedConfigFile",
		"legacyManagedConfigMdm", "unknown",
	}
	for _, method := range []string{"hook/started", "hook/completed"} {
		for _, registration := range []string{"On", "Add"} {
			for _, path := range []string{"/tmp/hook", `C:\hooks\demo`} {
				for _, source := range append(append([]string(nil), sources...), "") {
					t.Run(method+"/"+registration+"/"+path+"/"+source, func(t *testing.T) {
						transport := NewMockTransport()
						var callbackCalled bool
						var gotSource *codex.HookSource
						var gotPath string
						client := codex.NewClient(transport)
						t.Cleanup(func() { _ = client.Close() })
						var unsubscribe func()
						t.Cleanup(func() {
							if unsubscribe != nil {
								unsubscribe()
							}
						})
						startedHandler := func(n codex.HookStartedNotification) {
							callbackCalled, gotSource, gotPath = true, n.Run.Source, n.Run.SourcePath
						}
						completedHandler := func(n codex.HookCompletedNotification) {
							callbackCalled, gotSource, gotPath = true, n.Run.Source, n.Run.SourcePath
						}
						if method == "hook/started" {
							if registration == "On" {
								client.OnHookStarted(startedHandler)
							} else {
								unsubscribe = client.AddHookStartedListener(startedHandler)
							}
						} else if registration == "On" {
							client.OnHookCompleted(completedHandler)
						} else {
							unsubscribe = client.AddHookCompletedListener(completedHandler)
						}
						pathJSON, err := json.Marshal(path)
						if err != nil {
							t.Fatal(err)
						}
						extra := `"sourcePath":` + string(pathJSON)
						if source != "" {
							extra += `,"source":"` + source + `"`
						}
						params := `{"run":{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"run-1","scope":"thread",` + extra + `,"startedAt":123,"status":"completed"},"threadId":"thread-1"}`
						transport.InjectServerNotification(context.Background(), codex.Notification{JSONRPC: "2.0", Method: method, Params: json.RawMessage(params)})
						if !callbackCalled {
							t.Fatal("typed hook callback did not run for valid run summary")
						}
						if gotPath != path {
							t.Fatalf("sourcePath = %q; want %q", gotPath, path)
						}
						if source == "" && (gotSource == nil || *gotSource != codex.HookSourceUnknown) {
							t.Fatalf("omitted source = %v; want %q", gotSource, codex.HookSourceUnknown)
						}
						if source != "" && (gotSource == nil || string(*gotSource) != source) {
							t.Fatalf("source = %v; want %q", gotSource, source)
						}
					})
				}
			}
		}
	}
}

func TestHookRunSummarySourceOccurrencesUseExactJSONFieldSemantics(t *testing.T) {
	tests := []struct {
		name, fields, sourcePath, wantSource, wantError string
	}{
		{"escaped key and value", `"\u0073ource":"syst\u0065m"`, "/tmp/hook", "system", ""},
		{"escaped key unknown value", `"\u0073ource":"bogus"`, "/tmp/hook", "", "invalid hook.source"},
		{"invalid duplicate before valid", `"source":"bogus","source":"user"`, "/tmp/hook", "", "invalid hook.source"},
		{"explicit null remains invalid before valid duplicate", `"source":null,"source":"user"`, "/tmp/hook", "", "hook.source"},
		{"folded alias ignored and omitted source defaults", `"SOURCE":"bogus"`, "/tmp/hook", "unknown", ""},
		{"path error precedes enum restriction", `"source":"bogus"`, "relative/hook", "", "must be an absolute path"},
		{"native type error precedes new restrictions", `"source":7`, "relative/hook", "", "cannot unmarshal number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			var called bool
			var gotSource *codex.HookSource
			var handlerError error
			client := codex.NewClient(transport, codex.WithHandlerErrorCallback(func(_ string, err error) { handlerError = err }))
			t.Cleanup(func() { _ = client.Close() })
			client.OnHookStarted(func(n codex.HookStartedNotification) { called, gotSource = true, n.Run.Source })
			pathJSON, err := json.Marshal(tt.sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			params := `{"run":{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"run-1","scope":"thread","sourcePath":` + string(pathJSON) + `,` + tt.fields + `,"startedAt":123,"status":"completed"},"threadId":"thread-1"}`
			transport.InjectServerNotification(context.Background(), codex.Notification{JSONRPC: "2.0", Method: "hook/started", Params: json.RawMessage(params)})
			if tt.wantError == "" {
				if !called {
					t.Fatalf("valid or ignored source did not reach callback; error = %v", handlerError)
				}
				if tt.wantSource == "" && gotSource != nil {
					t.Fatalf("source = %v; want nil", *gotSource)
				}
				if tt.wantSource != "" && (gotSource == nil || string(*gotSource) != tt.wantSource) {
					t.Fatalf("source = %v; want %q", gotSource, tt.wantSource)
				}
				return
			}
			if called {
				t.Fatal("callback ran despite source/path rejection")
			}
			if handlerError == nil || !strings.Contains(handlerError.Error(), tt.wantError) {
				t.Fatalf("handler error = %v; want substring %q", handlerError, tt.wantError)
			}
		})
	}
}

func TestHookRunSummaryInvalidAdmissionPreservesReceiverAndAllowsRecovery(t *testing.T) {
	previousSource := codex.HookSource("user")
	run := codex.HookRunSummary{
		ID:         "previous",
		Source:     &previousSource,
		SourcePath: "/tmp/previous",
		Entries:    []codex.HookOutputEntry{{Kind: codex.HookOutputEntryKindWarning, Text: "previous"}},
	}
	previousSourceExpected := codex.HookSource("user")
	wantBefore := codex.HookRunSummary{
		ID:         "previous",
		Source:     &previousSourceExpected,
		SourcePath: "/tmp/previous",
		Entries:    []codex.HookOutputEntry{{Kind: codex.HookOutputEntryKindWarning, Text: "previous"}},
	}
	entriesSlot := &run.Entries[0]
	sourceSlot := run.Source
	invalid := `{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"invalid","scope":"thread","source":"bogus","sourcePath":"/tmp/hook","startedAt":123,"status":"completed"}`
	if err := json.Unmarshal([]byte(invalid), &run); err == nil {
		t.Fatal("invalid HookRunSummary source unexpectedly decoded")
	}
	if !reflect.DeepEqual(run, wantBefore) || run.Source != sourceSlot || &run.Entries[0] != entriesSlot {
		t.Fatalf("receiver or prior reference changed after rejected decode: got %+v, want %+v", run, wantBefore)
	}
	valid := `{"displayOrder":2,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"recovered","scope":"thread","source":"system","sourcePath":"/tmp/hook","startedAt":123,"status":"completed"}`
	if err := json.Unmarshal([]byte(valid), &run); err != nil {
		t.Fatalf("valid recovery decode failed: %v", err)
	}
	if run.ID != "recovered" || run.Source == nil || *run.Source != codex.HookSourceSystem || run.SourcePath != "/tmp/hook" {
		t.Fatalf("recovered value = %+v", run)
	}
}

func TestHookMetadataCommandAsyncOmittedDefaultsFalseThroughHooksList(t *testing.T) {
	response, err := issue74ReadHooks(issue74HookPayload(`,"handlerType":"command","command":"echo ready"`))
	if err != nil {
		t.Fatalf("Hooks.List error = %v", err)
	}
	async := response.Data[0].Hooks[0].Async
	if async == nil || *async {
		t.Fatalf("omitted command async = %v; want non-nil false default", async)
	}
	if response.Data[0].Hooks[0].AdditionalContextLimit != nil {
		t.Fatalf("omitted additionalContextLimit = %v; want nil", *response.Data[0].Hooks[0].AdditionalContextLimit)
	}
	encoded, err := json.Marshal(response.Data[0].Hooks[0])
	if err != nil {
		t.Fatalf("marshal defaulted command hook: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatal(err)
	}
	if string(members["async"]) != "false" {
		t.Fatalf("serialized default async = %s; want false", members["async"])
	}
}

func TestHookMetadataAsyncDefaultFollowsFinalHandlerBranch(t *testing.T) {
	tests := []struct {
		name       string
		properties string
		wantAsync  *bool
	}{
		{"command omitted", `,"handlerType":"command","command":"echo ready"`, issue127Bool(false)},
		{"command explicit true", `,"handlerType":"command","command":"echo ready","async":true`, issue127Bool(true)},
		{"command explicit false", `,"handlerType":"command","command":"echo ready","async":false`, issue127Bool(false)},
		{"final command duplicate handler type", `,"handlerType":"mcpTool","server":"docs","tool":"search","handlerType":"command","command":"echo ready"`, issue127Bool(false)},
		{"final mcpTool duplicate handler type", `,"handlerType":"command","command":"echo ready","handlerType":"mcpTool","server":"docs","tool":"search"`, nil},
		{"final mcpTool preserves legacy async", `,"handlerType":"command","command":"echo ready","async":true,"handlerType":"mcpTool","server":"docs","tool":"search"`, issue127Bool(true)},
		{"prompt branch", `,"handlerType":"prompt"`, nil},
		{"agent branch", `,"handlerType":"agent"`, nil},
		{"mcpTool branch", `,"handlerType":"mcpTool","server":"docs","tool":"search"`, nil},
		{"prompt legacy extra async true", `,"handlerType":"prompt","async":true`, issue127Bool(true)},
		{"mcpTool legacy extra async null", `,"handlerType":"mcpTool","server":"docs","tool":"search","async":null`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := issue74ReadHooks(issue74HookPayload(tt.properties))
			if err != nil {
				t.Fatalf("Hooks.List error = %v", err)
			}
			got := response.Data[0].Hooks[0].Async
			if tt.wantAsync == nil {
				if got != nil {
					t.Fatalf("async = %v; want nil for non-command branch", *got)
				}
				return
			}
			if got == nil || *got != *tt.wantAsync {
				t.Fatalf("async = %v; want %v", got, *tt.wantAsync)
			}
		})
	}
}

func TestHookMetadataAsyncExactAndEscapedFieldSemantics(t *testing.T) {
	tests := []struct {
		name       string
		properties string
		want       bool
	}{
		{"exact explicit true", `,"handlerType":"command","command":"echo ready","async":true`, true},
		{"escaped exact key", `,"handlerType":"command","command":"echo ready","\u0061sync":true`, true},
		{"folded alias ignored then defaulted", `,"handlerType":"command","command":"echo ready","ASYNC":true`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := issue74ReadHooks(issue74HookPayload(tt.properties))
			if err != nil {
				t.Fatalf("Hooks.List error = %v", err)
			}
			async := response.Data[0].Hooks[0].Async
			if async == nil || *async != tt.want {
				t.Fatalf("async = %v; want non-nil %v", async, tt.want)
			}
		})
	}
}

func TestHookRunSummaryDuplicateRunUsesFreshSourceDefault(t *testing.T) {
	transport := NewMockTransport()
	var got codex.HookSource
	var gotRunID string
	var called bool
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	client.OnHookStarted(func(notification codex.HookStartedNotification) {
		called = true
		gotRunID = notification.Run.ID
		if notification.Run.Source != nil {
			got = *notification.Run.Source
		}
	})
	previous := `{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"previous","scope":"thread","source":"user","sourcePath":"/tmp/hook","startedAt":123,"status":"running"}`
	final := `{"displayOrder":2,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"final","scope":"thread","sourcePath":"/tmp/hook","startedAt":124,"status":"running"}`
	params := `{"run":` + previous + `,"run":` + final + `,"threadId":"thread-1"}`
	transport.InjectServerNotification(context.Background(), codex.Notification{Method: "hook/started", Params: json.RawMessage(params)})
	if !called || gotRunID != "final" || got != codex.HookSourceUnknown {
		t.Fatalf("duplicate run result: called=%v id=%q source=%q", called, gotRunID, got)
	}
}

func TestHookMetadataAsyncDefaultDirectDecodeOwnershipAndRecovery(t *testing.T) {
	explicitTrue := true
	metadata := codex.HookMetadata{Async: &explicitTrue, Key: "old", SourcePath: "/tmp/old"}
	previousAsync := metadata.Async
	invalid := issue74HookPayload(`,"handlerType":"command","command":"echo ready","async":null`)
	if err := json.Unmarshal([]byte(invalid), &metadata); err == nil {
		t.Fatal("command async null unexpectedly decoded")
	}
	if metadata.Async != previousAsync || metadata.Async == nil || !*metadata.Async || metadata.Key != "old" || metadata.SourcePath != "/tmp/old" {
		t.Fatalf("receiver changed after rejected decode: %+v", metadata)
	}

	command := issue74HookPayload(`,"handlerType":"command","command":"echo ready"`)
	if err := json.Unmarshal([]byte(command), &metadata); err != nil {
		t.Fatalf("command recovery decode failed: %v", err)
	}
	if metadata.Async == nil || *metadata.Async {
		t.Fatalf("command default async = %v; want fresh non-nil false", metadata.Async)
	}
	commandAsync := metadata.Async
	var anotherCommand codex.HookMetadata
	if err := json.Unmarshal([]byte(command), &anotherCommand); err != nil {
		t.Fatalf("second command decode failed: %v", err)
	}
	if anotherCommand.Async == nil || anotherCommand.Async == commandAsync {
		t.Fatal("separate decodes reused the default false pointer")
	}
	*metadata.Async = true
	if *anotherCommand.Async {
		t.Fatal("mutating one default async pointer changed another decode")
	}
	*metadata.Async = false

	prompt := issue74HookPayload(`,"handlerType":"prompt"`)
	if err := json.Unmarshal([]byte(prompt), &metadata); err != nil {
		t.Fatalf("prompt recovery decode failed: %v", err)
	}
	if metadata.Async != nil {
		t.Fatalf("prompt async = %v; want nil", *metadata.Async)
	}
	if *commandAsync {
		t.Fatal("later decode changed previously returned default pointer")
	}

	commandText := "echo ready"
	constructed := codex.HookMetadata{
		CurrentHash: "hash",
		EventName:   codex.HookEventNameSessionStart,
		HandlerType: codex.HookHandlerTypeCommand,
		Command:     &commandText,
		Source:      codex.HookSourceUser,
		TrustStatus: codex.HookTrustStatusTrusted,
	}
	encoded, err := json.Marshal(constructed)
	if err != nil {
		t.Fatalf("marshal constructed command hook: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatal(err)
	}
	if _, present := members["async"]; present {
		t.Fatalf("constructed nil async was materialized during serialization: %s", encoded)
	}
}

func TestHookMetadataAsyncDefaultOnDuplicateArrayReuse(t *testing.T) {
	first := issue74HookPayload(`,"handlerType":"command","command":"echo first","async":true`)
	second := issue74HookPayload(`,"handlerType":"command","command":"echo second"`)
	first = strings.TrimSuffix(strings.TrimPrefix(first, "{"), "}")
	second = strings.TrimSuffix(strings.TrimPrefix(second, "{"), "}")
	payload := `{"data":[{"cwd":"/tmp","errors":[],"hooks":[{` + first + `}],"hooks":[{` + second + `}],"warnings":[]}]}`
	transport := NewMockTransport()
	if err := transport.SetResponseData("hooks/list", json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	response, err := client.Hooks.List(context.Background(), codex.HooksListParams{})
	if err != nil {
		t.Fatalf("Hooks.List duplicate arrays failed: %v", err)
	}
	hooks := response.Data[0].Hooks
	if len(hooks) != 1 || hooks[0].Command == nil || *hooks[0].Command != "echo second" || hooks[0].Async == nil || *hooks[0].Async {
		t.Fatalf("final duplicate-array hook = %+v; want second command and fresh false async default", hooks)
	}
}

func issue127Bool(value bool) *bool { return &value }
