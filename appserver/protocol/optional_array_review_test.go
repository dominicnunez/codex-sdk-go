package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestOptionalArrayAnonymousEnvelopes(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		newEnvelope func() any
	}{
		{"reasoning", `"id":"r","summary":["s"]`, func() any {
			return &struct {
				Before string
				codex.ReasoningThreadItem
				After string
			}{}
		}},
		{"text", `"text":"hello","text_elements":[]`, func() any {
			return &struct {
				Before string
				codex.TextUserInput
				After string
			}{}
		}},
		{"workspace", `"writable_roots":["relative"]`, func() any {
			return &struct {
				Before string
				codex.SandboxWorkspaceWrite
				After string
			}{}
		}},
		{"policy", `"writableRoots":["/tmp/project"]`, func() any {
			return &struct {
				Before string
				codex.SandboxPolicyWorkspaceWrite
				After string
			}{}
		}},
		{"marketplace", `"marketplacePath":"/tmp/plugins","message":"unavailable"`, func() any {
			return &struct {
				Before string
				codex.MarketplaceLoadErrorInfo
				After string
			}{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envelope := tc.newEnvelope()
			if err := json.Unmarshal([]byte(`{"Before":"b",`+tc.field+`,"After":"a"}`), envelope); err != nil {
				t.Fatal(err)
			}
			value := reflect.ValueOf(envelope).Elem()
			if value.FieldByName("Before").String() != "b" || value.FieldByName("After").String() != "a" {
				t.Fatalf("embedded variant consumed application fields: %+v", envelope)
			}
		})
	}
}

func TestOptionalArrayEstablishedErrorPrecedence(t *testing.T) {
	const model = `{"id":42,"model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]}`
	lifecycle, err := json.Marshal(validThreadLifecycleResponse(validProcessThreadPayload("array-thread")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, nullField string
		newReceiver           func() any
	}{
		{"model", model, "inputModalities", func() any { return &codex.Model{} }},
		{"plugin-list", `{"marketplaces":[],"remoteSyncError":42}`, "marketplaceLoadErrors", func() any { return &codex.PluginListResponse{} }},
		{"plugin-list-marketplaces", `{"marketplaces":42}`, "featuredPluginIds", func() any { return &codex.PluginListResponse{} }},
		{"plugin-installed", `{"marketplaces":42}`, "marketplaceLoadErrors", func() any { return &codex.PluginInstalledResponse{} }},
		{"reasoning", `{"type":"reasoning","id":42}`, "summary", func() any { return &codex.ThreadItemWrapper{} }},
		{"text", `{"type":"text","text":42}`, "text_elements", func() any { return &optionalTextInput{} }},
		{"policy", `{"type":"workspaceWrite","networkAccess":42}`, "writableRoots", func() any { return &codex.SandboxPolicyWrapper{} }},
		{"config", `{"sandbox_workspace_write":{"network_access":42}}`, "sandbox_workspace_write", func() any { return &codex.Config{} }},
		{"start", strings.Replace(string(lifecycle), `"model":"`, `"model":42,"ignored":"`, 1), "disabledPluginIds", func() any { return &codex.ThreadStartResponse{} }},
		{"resume", strings.Replace(string(lifecycle), `"model":"`, `"model":42,"ignored":"`, 1), "instructionSources", func() any { return &codex.ThreadResumeResponse{} }},
		{"fork", strings.Replace(string(lifecycle), `"model":"`, `"model":42,"ignored":"`, 1), "disabledPluginIds", func() any { return &codex.ThreadForkResponse{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := tc.body
			body := strings.TrimSuffix(base, "}") + `,"` + tc.nullField + `":null}`
			if tc.name == "config" {
				body = `{"sandbox_workspace_write":{"network_access":42,"writable_roots":null}}`
			}
			wantErr := json.Unmarshal([]byte(base), tc.newReceiver())
			var wantType *json.UnmarshalTypeError
			if !errors.As(wantErr, &wantType) {
				t.Fatalf("reference did not produce type error: %s: %v", base, wantErr)
			}
			gotErr := json.Unmarshal([]byte(body), tc.newReceiver())
			if !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("new null guard replaced established error: got %+v, want %+v", gotErr, wantErr)
			}
		})
	}
}

func TestOptionalArraySemanticErrorPrecedence(t *testing.T) {
	for _, field := range []string{"authPolicy", "installPolicy", "availability"} {
		t.Run(field, func(t *testing.T) {
			var baseFields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(issue74PluginSummary), &baseFields); err != nil {
				t.Fatal(err)
			}
			baseFields[field] = json.RawMessage(`"invalid"`)
			base, err := json.Marshal(baseFields)
			if err != nil {
				t.Fatal(err)
			}
			var reference, actual codex.PluginSummary
			wantErr := json.Unmarshal(base, &reference)
			if wantErr == nil {
				t.Fatal("fixture did not fail established enum validation")
			}
			baseFields["keywords"] = json.RawMessage(`null`)
			body, err := json.Marshal(baseFields)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := json.Unmarshal(body, &actual)
			if !reflect.DeepEqual(gotErr, wantErr) || !reflect.DeepEqual(actual, reference) {
				t.Fatalf("semantic error/receiver changed: got=%v %+v, want=%v %+v", gotErr, actual, wantErr, reference)
			}
		})
	}
	var basePolicy, nullPolicy codex.SandboxPolicyWrapper
	wantErr := json.Unmarshal([]byte(`{"type":"workspaceWrite","readOnlyAccess":{"type":"restricted","readableRoots":["relative"]}}`), &basePolicy)
	gotErr := json.Unmarshal([]byte(`{"type":"workspaceWrite","readOnlyAccess":{"type":"restricted","readableRoots":["relative"]},"writableRoots":null}`), &nullPolicy)
	if wantErr == nil || !reflect.DeepEqual(gotErr, wantErr) {
		t.Fatalf("path validation precedence changed: got=%v want=%v", gotErr, wantErr)
	}
}

func TestOptionalConfigWorkspaceArrayOwner(t *testing.T) {
	for _, field := range []string{"writable_roots", "WRITABLE_ROOTS", `\u0077ritable_roots`} {
		for _, value := range []string{"null", "[null]", "true", "[42]"} {
			for _, suffix := range []string{"", `,"sandbox_workspace_write":{"writable_roots":[]}`} {
				body := `{"sandbox_workspace_write":{"` + field + `":` + value + `}}`
				if suffix != "" {
					body = strings.TrimSuffix(body, "}") + suffix + `}`
				}
				var config codex.Config
				if err := json.Unmarshal([]byte(body), &config); err == nil {
					t.Errorf("accepted %s", body)
				}
			}
		}
	}
	prior := &codex.SandboxWorkspaceWrite{WritableRoots: []string{"relative"}}
	config := codex.Config{SandboxWorkspaceWrite: prior}
	for _, body := range []string{`{}`, `{"sandbox_workspace_write":{}}`, `{"sandbox_workspace_write":{"network_access":true},"sandbox_workspace_write":{"writable_roots":["", "relative"]}}`} {
		if err := json.Unmarshal([]byte(body), &config); err != nil {
			t.Fatal(err)
		}
		if config.SandboxWorkspaceWrite != prior {
			t.Fatal("valid decoding replaced existing workspace pointer")
		}
	}
	if prior.NetworkAccess == nil || !*prior.NetworkAccess || !reflect.DeepEqual(prior.WritableRoots, []string{"", "relative"}) {
		t.Fatalf("duplicate nested objects lost merged fields: %+v", prior)
	}
	if err := json.Unmarshal([]byte(`{"sandbox_workspace_write":null}`), &config); err != nil || config.SandboxWorkspaceWrite != nil {
		t.Fatalf("nullable outer workspace lost reset semantics: %+v %v", config, err)
	}
}

func TestOptionalConfigWorkspacePartialErrorCompatibility(t *testing.T) {
	// Independent method-free model of the baseline fields exercised here.
	// The existing workspace-restriction decoder is absent from this fixture.
	type Config struct {
		SandboxWorkspaceWrite *codex.SandboxWorkspaceWrite `json:"sandbox_workspace_write"`
		ModelProvider         *string                      `json:"model_provider"`
	}
	old := &codex.SandboxWorkspaceWrite{WritableRoots: []string{"prior"}}
	referenceOld := &codex.SandboxWorkspaceWrite{WritableRoots: []string{"prior"}}
	actual := codex.Config{SandboxWorkspaceWrite: old}
	reference := Config{SandboxWorkspaceWrite: referenceOld}
	body := []byte(`{"sandbox_workspace_write":{"network_access":42,"writable_roots":null},"model_provider":"updated"}`)
	gotErr, wantErr := json.Unmarshal(body, &actual), json.Unmarshal(body, &reference)
	if !reflect.DeepEqual(gotErr, wantErr) || !reflect.DeepEqual(old, referenceOld) || !reflect.DeepEqual(actual.ModelProvider, reference.ModelProvider) || actual.SandboxWorkspaceWrite != old {
		t.Fatalf("baseline error/partial update/pointer retention changed: got=%v %+v %+v, want=%v %+v", gotErr, actual, old, wantErr, reference)
	}
}

func TestOptionalArrayLaterServiceValidationBoundary(t *testing.T) {
	// A newly rejected array stops the existing codec before later service
	// validation; only established checks within that codec retain precedence.
	body := validThreadLifecycleResponse(validProcessThreadPayload("array-thread"))
	body["model"] = ""
	body["disabledPluginIds"] = nil
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	mock := NewMockTransport()
	mock.SetResponse("thread/start", codex.Response{Result: payload})
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	_, err = client.Thread.Start(context.Background(), codex.ThreadStartParams{})
	if !errors.Is(err, codex.ErrNullResultField) {
		t.Fatalf("new admission error must precede later missing-model service check: %v", err)
	}
	if _, ok := client.ThreadStateSnapshot("array-thread"); ok {
		t.Fatal("failed admission published thread state")
	}
}

func TestOptionalMarketplaceRecordSelection(t *testing.T) {
	for _, method := range []string{"plugin/list", "plugin/installed"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			call := func(record string) ([]codex.MarketplaceLoadErrorInfo, error) {
				mock.SetResponse(method, codex.Response{Result: json.RawMessage(`{"marketplaces":[],"marketplaceLoadErrors":[` + record + `]}`)})
				if method == "plugin/list" {
					response, err := client.Plugin.List(context.Background(), codex.PluginListParams{})
					return response.MarketplaceLoadErrors, err
				}
				response, err := client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
				return response.MarketplaceLoadErrors, err
			}
			for _, record := range []string{
				`{"marketplacePath":"relative","MARKETPLACEPATH":"/tmp/last","message":"ok"}`,
				`{"MARKETPLACEPATH":"relative","marketplacePath":"/tmp/last","message":"ok"}`,
				`{"marketplacePath":"relative","\u004darketplacePath":"/tmp/last","message":"ok"}`,
				`{"marketplacePath":"/tmp/last","message":"first","me\u017f\u017fage":"last"}`,
			} {
				var expected codex.MarketplaceLoadErrorInfo
				if err := json.Unmarshal([]byte(record), &expected); err != nil {
					t.Fatal(err)
				}
				got, err := call(record)
				if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0], expected) {
					t.Fatalf("selected valid record differs from ordinary wire: %s got=%+v %v want=%+v", record, got, err, expected)
				}
			}
			for _, record := range []string{
				`{"marketplacePath":"/tmp","MARKETPLACEPATH":"relative","message":"ok"}`,
				`{"MARKETPLACEPATH":"/tmp","marketplacePath":"relative","message":"ok"}`,
				`{"marketplacePath":"/tmp","MESSAGE":null,"message":"ok"}`,
				`{"marketplacePath":"/tmp","message":"ok","me\u017f\u017fage":null}`,
				`{"marketplacePath":"/tmp","MARKETPLACEPATH":null,"message":"ok"}`,
				`{"MARKETPLACEPATH":null,"marketplacePath":"/tmp","message":"ok"}`,
				// Schema requiredness remains canonical, as in other strict owners.
				`{"MARKETPLACEPATH":"/tmp","MESSAGE":"ok"}`,
			} {
				if _, err := call(record); err == nil {
					t.Errorf("accepted invalid record %s", record)
				}
			}
		})
	}
	// Independent stdlib record model retains the original nested error context.
	type MarketplaceLoadErrorInfo struct {
		MarketplacePath string `json:"marketplacePath"`
		Message         string `json:"message"`
	}
	type wire struct {
		Marketplaces          []json.RawMessage          `json:"marketplaces"`
		MarketplaceLoadErrors []MarketplaceLoadErrorInfo `json:"marketplaceLoadErrors"`
	}
	for _, body := range []string{`{"marketplaces":[],"marketplaceLoadErrors":[{"marketplacePath":42,"message":null}]}`, `{"marketplaces":[],"marketplaceLoadErrors":[{"marketplacePath":null,"message":42}]}`} {
		var reference wire
		wantErr := json.Unmarshal([]byte(body), &reference)
		for _, actual := range []any{&codex.PluginListResponse{}, &codex.PluginInstalledResponse{}} {
			gotErr := json.Unmarshal([]byte(body), actual)
			if wantErr == nil || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("record saved type error changed: got=%+v want=%+v", gotErr, wantErr)
			}
		}
	}
}
