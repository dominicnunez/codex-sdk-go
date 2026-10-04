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
						if source == "" && gotSource != nil {
							t.Fatalf("omitted source = %v; want nil to preserve omission", *gotSource)
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
		{"folded alias ignored", `"SOURCE":"bogus"`, "/tmp/hook", "", ""},
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
	before := run
	invalid := `{"displayOrder":1,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"invalid","scope":"thread","source":"bogus","sourcePath":"/tmp/hook","startedAt":123,"status":"completed"}`
	if err := json.Unmarshal([]byte(invalid), &run); err == nil {
		t.Fatal("invalid HookRunSummary source unexpectedly decoded")
	}
	if !reflect.DeepEqual(run, before) {
		t.Fatalf("receiver changed after rejected decode: got %+v, want %+v", run, before)
	}
	valid := `{"displayOrder":2,"entries":[],"eventName":"sessionStart","executionMode":"sync","handlerType":"command","id":"recovered","scope":"thread","source":"system","sourcePath":"/tmp/hook","startedAt":123,"status":"completed"}`
	if err := json.Unmarshal([]byte(valid), &run); err != nil {
		t.Fatalf("valid recovery decode failed: %v", err)
	}
	if run.ID != "recovered" || run.Source == nil || *run.Source != codex.HookSourceSystem || run.SourcePath != "/tmp/hook" {
		t.Fatalf("recovered value = %+v", run)
	}
}
