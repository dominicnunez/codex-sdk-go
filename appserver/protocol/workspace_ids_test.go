package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestConfigWorkspaceRestrictionWire(t *testing.T) {
	for _, raw := range []string{`"workspace-a"`, `["workspace-a","workspace-b"]`, `[]`, `null`, ``} {
		t.Run(raw, func(t *testing.T) {
			mock := NewMockTransport()
			fields := `"model":"retained"`
			if raw != "" {
				fields += `,"forced_chatgpt_workspace_id":` + raw
			}
			if err := mock.SetResponseData("config/read", json.RawMessage(`{"config":{`+fields+`},"origins":{}}`)); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			response, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
			if err != nil {
				t.Fatal(err)
			}
			if response.Config == nil || response.Config.Model == nil || *response.Config.Model != "retained" {
				t.Fatalf("containing response lost: %+v", response)
			}
			// Preserve the existing pointer convention for null/absence. Arrays,
			// including empty arrays, must retain their actual wire branch.
			present := raw != "" && raw != "null"
			requireJSONMember(t, response.Config, "forced_chatgpt_workspace_id", present, raw)
			if raw == `"workspace-a"` && (response.Config.ForcedChatgptWorkspaceID == nil || *response.Config.ForcedChatgptWorkspaceID != "workspace-a") {
				t.Fatal("legacy single-ID field lost")
			}
		})
	}
}

func TestConfigWorkspaceSiblingErrorCompatibility(t *testing.T) {
	type Config struct {
		Model                 *string                      `json:"model"`
		Instructions          *string                      `json:"instructions"`
		Profiles              map[string]codex.ProfileV2   `json:"profiles"`
		SandboxWorkspaceWrite *codex.SandboxWorkspaceWrite `json:"sandbox_workspace_write"`
	}
	for _, body := range []string{
		`{"model":1,"instructions":"later"}`,
		`{"model":1,"forced_chatgpt_workspace_id":["a"],"instructions":"later"}`,
		`{"forced_chatgpt_workspace_id":["a"],"model":1,"instructions":"later"}`,
		`{"profiles":{"a":{"model":1}},"instructions":"later"}`,
		`{"sandbox_workspace_write":{"writable_roots":[1]},"instructions":"later"}`,
		`{"profiles":{"a":1},"instructions":"later"}`,
	} {
		var reference Config
		var actual codex.Config
		wantErr := json.Unmarshal([]byte(body), &reference)
		gotErr := json.Unmarshal([]byte(body), &actual)
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) || !reflect.DeepEqual(want, got) {
			t.Fatalf("body=%s got=%+v want=%+v", body, got, want)
		}
		if !reflect.DeepEqual(actual.Instructions, reference.Instructions) || !reflect.DeepEqual(actual.Profiles, reference.Profiles) || !reflect.DeepEqual(actual.SandboxWorkspaceWrite, reference.SandboxWorkspaceWrite) {
			t.Fatal("sibling receiver mutation differs")
		}
	}
}

func TestConfigWorkspaceContainingResponse(t *testing.T) {
	var response codex.ConfigReadResponse
	const body = `{"config":{"forced_chatgpt_workspace_id":"old","model":"retained","desktop":{"a":true}},"config":{"forced_chatgpt_workspace_id":["b","b"],"desktop":{"b":false}},"origins":{}}`
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatal(err)
	}
	if response.Config == nil || response.Config.Model == nil || *response.Config.Model != "retained" || response.Config.ForcedChatgptWorkspaceID != nil {
		t.Fatalf("merged configuration lost: %+v", response.Config)
	}
	requireJSONMember(t, response.Config, "forced_chatgpt_workspace_id", true, `["b","b"]`)
	requireJSONMember(t, response.Config, "desktop", true, `{"a":true,"b":false}`)
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var again codex.ConfigReadResponse
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response, again) {
		t.Fatal("containing response round trip changed data")
	}
	for _, raw := range []string{`true`, `1`, `{}`, `[null]`, `["a",null]`} {
		mock := NewMockTransport()
		mock.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{"config":{"forced_chatgpt_workspace_id":` + raw + `},"origins":{}}`)})
		client := codex.NewClient(mock)
		got, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
		client.Close()
		if err == nil || got.Config != nil {
			t.Fatalf("malformed restriction published a typed response: %s, %+v, %v", raw, got, err)
		}
	}
}

func TestConfigWorkspaceJSONEnvelope(t *testing.T) {
	var envelope struct {
		Config codex.Config `json:"config"`
		Label  string       `json:"label"`
	}
	if err := json.Unmarshal([]byte(`{"config":{"forced_chatgpt_workspace_id":[]},"label":"kept"}`), &envelope); err != nil {
		t.Fatal(err)
	}
	requireJSONMember(t, envelope, "label", true, `"kept"`)
	requireJSONMember(t, envelope.Config, "forced_chatgpt_workspace_id", true, `[]`)
	if err := json.Unmarshal([]byte(`{"config":{"model":1},"label":"later"}`), &envelope); err == nil {
		t.Fatal("invalid inner configuration accepted")
	}
	if envelope.Label != "kept" {
		t.Fatal("custom decoder failure did not stop outer traversal")
	}
}

func TestConfigWorkspaceNonObjectErrors(t *testing.T) {
	type Config struct{}
	for _, body := range []string{`[]`, `1`, `true`, `"text"`} {
		var reference Config
		var actual codex.Config
		wantErr := json.Unmarshal([]byte(body), &reference)
		gotErr := json.Unmarshal([]byte(body), &actual)
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) {
			t.Fatalf("body=%s got=%v want=%v", body, gotErr, wantErr)
		}
		want.Type = reflect.TypeFor[codex.Config]()
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("body=%s got=%+v want=%+v", body, got, want)
		}
	}
}

func TestConfigWorkspaceMalformedAdmission(t *testing.T) {
	for _, body := range []string{
		`{"forced_chatgpt_workspace_id":true,"instructions":"later"}`,
		`{"model":1,"forced_chatgpt_workspace_id":true,"instructions":"later"}`,
	} {
		var config codex.Config
		err := json.Unmarshal([]byte(body), &config)
		var failure *json.UnmarshalTypeError
		if !errors.As(err, &failure) || failure.Value != "bool" {
			t.Fatalf("restriction admission error=%v", err)
		}
		if config.Instructions != nil {
			t.Fatal("later configuration decoded after malformed restriction")
		}
		if err := json.Unmarshal([]byte(`{"forced_chatgpt_workspace_id":["recovered"],"instructions":"valid"}`), &config); err != nil {
			t.Fatal(err)
		}
		requireJSONMember(t, config, "forced_chatgpt_workspace_id", true, `["recovered"]`)
		if config.ForcedChatgptWorkspaceID != nil || config.Instructions == nil || *config.Instructions != "valid" {
			t.Fatal("subsequent valid configuration did not recover")
		}
	}
}

func TestConfigWorkspaceConstructedBranches(t *testing.T) {
	emptyString := ""
	single := "a"
	var nilList []string
	emptyList := []string{}
	one := []string{"a"}
	many := []string{"a", "b"}
	for _, test := range []struct {
		config codex.Config
		wire   string
	}{
		{codex.Config{}, ""},
		{codex.Config{ForcedChatgptWorkspaceID: &emptyString}, `""`},
		{codex.Config{ForcedChatgptWorkspaceID: &single}, `"a"`},
		{codex.Config{ForcedChatgptWorkspaceIDs: &nilList}, `[]`},
		{codex.Config{ForcedChatgptWorkspaceIDs: &emptyList}, `[]`},
		{codex.Config{ForcedChatgptWorkspaceIDs: &one}, `["a"]`},
		{codex.Config{ForcedChatgptWorkspaceIDs: &many}, `["a","b"]`},
	} {
		requireJSONMember(t, test.config, "forced_chatgpt_workspace_id", test.wire != "", test.wire)
		requireJSONMember(t, &test.config, "forced_chatgpt_workspace_id", test.wire != "", test.wire)
		data, err := json.Marshal(test.config)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip codex.Config
		if err := json.Unmarshal(data, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if (test.config.ForcedChatgptWorkspaceIDs != nil) != (roundTrip.ForcedChatgptWorkspaceIDs != nil) ||
			(test.config.ForcedChatgptWorkspaceID != nil) != (roundTrip.ForcedChatgptWorkspaceID != nil) {
			t.Fatal("constructed branch lost during round trip")
		}
	}
	if nilList != nil {
		t.Fatal("marshaling mutated caller nil slice")
	}
}

func FuzzConfigWorkspaceRestrictionRoundTrip(f *testing.F) {
	f.Add("workspace-a", "workspace-b", true)
	f.Add("", "", false)
	f.Add("duplicate", "duplicate", true)
	f.Fuzz(func(t *testing.T, first, second string, array bool) {
		if len(first)+len(second) > 4096 {
			t.Skip()
		}
		var input any = first
		if array {
			input = []string{first, second}
		}
		member, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		// JSON replaces invalid UTF-8. Compare semantic JSON rather than the
		// spelling of replacement characters or escaped Unicode sequences.
		var reference any
		if err := json.Unmarshal(member, &reference); err != nil {
			t.Fatal(err)
		}
		expected, err := json.Marshal(reference)
		if err != nil {
			t.Fatal(err)
		}
		var config codex.Config
		if err := json.Unmarshal(append(append([]byte(`{"forced_chatgpt_workspace_id":`), member...), '}'), &config); err != nil {
			t.Fatal(err)
		}
		// Oracle is the generated wire branch, not the production decoder.
		requireJSONMember(t, config, "forced_chatgpt_workspace_id", true, string(expected))
		requireJSONMember(t, &config, "forced_chatgpt_workspace_id", true, string(expected))
		if array != (config.ForcedChatgptWorkspaceIDs != nil) || array == (config.ForcedChatgptWorkspaceID != nil) {
			t.Fatal("wire branch narrowed or changed")
		}
	})
}

func TestConfigWorkspaceRestrictionHistory(t *testing.T) {
	single := "original"
	config := codex.Config{ForcedChatgptWorkspaceID: &single}
	if err := json.Unmarshal([]byte(`{"FORCED_CHATGPT_WORKSPACE_ID":"updated"}`), &config); err != nil {
		t.Fatal(err)
	}
	if single != "updated" || config.ForcedChatgptWorkspaceID != &single {
		t.Fatal("legacy pointer identity or update changed")
	}
	for _, step := range []struct{ body, wire string }{
		{`{"forced_chatgpt_workspace_id":["a"]}`, `["a"]`},
		{`{}`, `["a"]`},
		{`null`, `["a"]`},
		{`{"forced_chatgpt_workspace_id":[],"FORCED_CHATGPT_WORKSPACE_ID":""}`, `""`},
		{`{"forced_chatgpt_workspace_id":"a","forced_chatgpt_workspace_\u0069d":[]}`, `[]`},
		{`{"forced_chatgpt_workspace_id":null}`, ``},
	} {
		if err := json.Unmarshal([]byte(step.body), &config); err != nil {
			t.Fatal(err)
		}
		requireJSONMember(t, config, "forced_chatgpt_workspace_id", step.wire != "", step.wire)
		if config.ForcedChatgptWorkspaceID != nil && config.ForcedChatgptWorkspaceIDs != nil {
			t.Fatal("stale opposite branch")
		}
	}
	for _, bad := range []string{`true`, `1`, `{}`, `[null]`, `[1]`, `["a",null]`} {
		var value codex.Config
		if err := json.Unmarshal([]byte(`{"forced_chatgpt_workspace_id":`+bad+`,"FORCED_CHATGPT_WORKSPACE_ID":["valid"]}`), &value); err == nil {
			t.Fatalf("malformed earlier restriction hidden: %s", bad)
		}
	}
	list := []string{"a", "b"}
	config = codex.Config{ForcedChatgptWorkspaceID: &single, ForcedChatgptWorkspaceIDs: &list}
	if _, err := json.Marshal(config); err == nil {
		t.Fatal("conflicting constructed restrictions silently narrowed")
	}
}
