package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestPlainPathRequestsForwardServerOwnedPaths(t *testing.T) {
	const relative = "nested/../repo"
	tests := []struct {
		name   string
		method string
		field  string
		call   func(*codex.Client, string) error
	}{
		{
			name: "config read cwd", method: "config/read", field: "cwd",
			call: func(c *codex.Client, value string) error {
				_, err := c.Config.Read(context.Background(), codex.ConfigReadParams{Cwd: codex.Ptr(value)})
				return err
			},
		},
		{
			name: "config write file path", method: "config/value/write", field: "filePath",
			call: func(c *codex.Client, value string) error {
				_, err := c.Config.Write(context.Background(), codex.ConfigValueWriteParams{KeyPath: "model", MergeStrategy: codex.MergeStrategyReplace, Value: json.RawMessage(`"gpt-5"`), FilePath: codex.Ptr(value)})
				return err
			},
		},
		{
			name: "config batch write file path", method: "config/batchWrite", field: "filePath",
			call: func(c *codex.Client, value string) error {
				_, err := c.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{Edits: []codex.ConfigEdit{{KeyPath: "model", MergeStrategy: codex.MergeStrategyReplace, Value: json.RawMessage(`"gpt-5"`)}}, FilePath: codex.Ptr(value)})
				return err
			},
		},
		{
			name: "permission profile cwd", method: "permissionProfile/list", field: "cwd",
			call: func(c *codex.Client, value string) error {
				_, err := c.PermissionProfile.List(context.Background(), codex.PermissionProfileListParams{Cwd: codex.Ptr(value)})
				return err
			},
		},
		{
			name: "skills list cwds", method: "skills/list", field: "cwds",
			call: func(c *codex.Client, value string) error {
				_, err := c.Skills.List(context.Background(), codex.SkillsListParams{Cwds: []string{value}})
				return err
			},
		},
		{
			name: "hooks list cwds", method: "hooks/list", field: "cwds",
			call: func(c *codex.Client, value string) error {
				_, err := c.Hooks.List(context.Background(), codex.HooksListParams{Cwds: []string{value}})
				return err
			},
		},
		{
			name: "fuzzy search roots", method: "fuzzyFileSearch", field: "roots",
			call: func(c *codex.Client, value string) error {
				_, err := c.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{Query: "needle", Roots: []string{value}})
				return err
			},
		},
		{
			name: "external detect cwds", method: "externalAgentConfig/detect", field: "cwds",
			call: func(c *codex.Client, value string) error {
				_, err := c.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{Cwds: codex.Ptr([]string{value})})
				return err
			},
		},
		{
			name: "external import item cwd", method: "externalAgentConfig/import", field: "migrationItems",
			call: func(c *codex.Client, value string) error {
				_, err := c.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{MigrationItems: []codex.ExternalAgentConfigMigrationItem{{Cwd: codex.Ptr(value), Description: "import", ItemType: codex.MigrationItemTypeConfig}}})
				return err
			},
		},
	}

	for _, tt := range tests {
		values := []struct{ name, value string }{{"relative", relative}, {"absolute spelling", "/tmp/../workspace"}}
		switch tt.name {
		case "skills list cwds", "hooks list cwds", "fuzzy search roots", "external detect cwds":
			values = append(values, struct{ name, value string }{"empty string", ""})
		}
		for _, tc := range values {
			t.Run(tt.name+"/"+tc.name, func(t *testing.T) {
				transport := NewMockTransport()
				defer transport.Close()
				if err := setPlainPathResponse(t, transport, tt.method); err != nil {
					t.Fatal(err)
				}
				client := codex.NewClient(transport)
				defer client.Close()
				if err := tt.call(client, tc.value); err != nil {
					t.Fatalf("request failed: %v", err)
				}
				request := transport.GetSentRequest(0)
				if request == nil || request.Method != tt.method {
					t.Fatalf("request = %v, want method %q", request, tt.method)
				}
				var params map[string]json.RawMessage
				if err := json.Unmarshal(request.Params, &params); err != nil {
					t.Fatalf("decode params %s: %v", request.Params, err)
				}
				want, err := json.Marshal(tc.value)
				switch tt.field {
				case "cwds", "roots":
					want, err = json.Marshal([]string{tc.value})
				case "migrationItems":
					want = json.RawMessage(`[{"cwd":` + string(mustJSONRawMessage(t, tc.value)) + `,"description":"import","itemType":"CONFIG"}]`)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := params[tt.field]; string(got) != string(want) {
					t.Fatalf("%s = %s, want %s", tt.field, got, want)
				}
			})
		}
	}
}

func TestSkillsListAcceptsServerOwnedRelativeResponseCwd(t *testing.T) {
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("skills/list", map[string]any{
		"data": []any{map[string]any{"cwd": "relative/repo", "errors": []any{}, "skills": []any{}}},
	}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	response, err := client.Skills.List(context.Background(), codex.SkillsListParams{Cwds: []string{"/absolute/request"}})
	if err != nil {
		t.Fatalf("Skills.List() error = %v", err)
	}
	if got := response.Data[0].Cwd; got != "relative/repo" {
		t.Fatalf("response cwd = %q, want unchanged server value", got)
	}
}

func TestSkillsListPreservesRelativeCwdAcrossRequestAndResponse(t *testing.T) {
	const cwd = "nested/../repo"
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("skills/list", map[string]any{
		"data": []any{map[string]any{"cwd": cwd, "errors": []any{}, "skills": []any{}}},
	}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	response, err := client.Skills.List(context.Background(), codex.SkillsListParams{Cwds: []string{cwd}})
	if err != nil {
		t.Fatalf("Skills.List() error = %v", err)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if got := string(params["cwds"]); got != `["nested/../repo"]` {
		t.Fatalf("request cwds = %s, want unchanged relative value", got)
	}
	if got := response.Data[0].Cwd; got != cwd {
		t.Fatalf("response cwd = %q, want unchanged server value", got)
	}
}

func TestExternalAgentDetectCwdsPreserveNullableArrayBranches(t *testing.T) {
	tests := []struct {
		name string
		cwds *[]string
		want string
	}{
		{name: "omitted pointer", cwds: nil, want: "<omitted>"},
		{name: "present nil slice", cwds: codex.Ptr([]string(nil)), want: `[]`},
		{name: "present empty slice", cwds: codex.Ptr([]string{}), want: `[]`},
		{name: "populated", cwds: codex.Ptr([]string{"nested/../repo"}), want: `["nested/../repo"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			defer transport.Close()
			if err := transport.SetResponseData("externalAgentConfig/detect", map[string]any{"items": []any{}}); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(transport)
			defer client.Close()
			if _, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{Cwds: tt.cwds}); err != nil {
				t.Fatalf("ConfigDetect() error = %v", err)
			}
			var params map[string]json.RawMessage
			if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			got, ok := params["cwds"]
			if tt.want == "<omitted>" {
				if ok {
					t.Fatalf("cwds = %s, want omitted", got)
				}
				return
			}
			if !ok || string(got) != tt.want {
				t.Fatalf("cwds = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExternalAgentImportCwdPreservesHomeAndRelativeBranches(t *testing.T) {
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("externalAgentConfig/import", map[string]any{"importId": "import-1"}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	items := []codex.ExternalAgentConfigMigrationItem{
		{Description: "omitted home cwd", ItemType: codex.MigrationItemTypeConfig},
		{Cwd: codex.Ptr(""), Description: "empty home cwd", ItemType: codex.MigrationItemTypeSkills},
		{Cwd: codex.Ptr("nested/../repo"), Description: "relative repo cwd", ItemType: codex.MigrationItemTypeHooks},
	}
	if _, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{MigrationItems: items}); err != nil {
		t.Fatalf("ConfigImport() error = %v", err)
	}
	var params struct {
		MigrationItems []map[string]json.RawMessage `json:"migrationItems"`
	}
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if len(params.MigrationItems) != 3 {
		t.Fatalf("migration item count = %d, want 3", len(params.MigrationItems))
	}
	if _, ok := params.MigrationItems[0]["cwd"]; ok {
		t.Fatalf("nil cwd = %s, want omitted", params.MigrationItems[0]["cwd"])
	}
	if got := string(params.MigrationItems[1]["cwd"]); got != `""` {
		t.Fatalf("empty cwd = %s, want empty string", got)
	}
	if got := string(params.MigrationItems[2]["cwd"]); got != `"nested/../repo"` {
		t.Fatalf("relative cwd = %s, want original spelling", got)
	}
}

func setPlainPathResponse(t *testing.T, transport *MockTransport, method string) error {
	t.Helper()
	var data any
	switch method {
	case "config/read":
		data = map[string]any{"config": map[string]any{}, "origins": map[string]any{}}
	case "config/value/write", "config/batchWrite":
		data = map[string]any{"filePath": "/tmp/config.toml", "status": "ok", "version": "v1"}
	case "permissionProfile/list":
		data = map[string]any{"data": []any{}}
	case "skills/list", "hooks/list":
		data = map[string]any{"data": []any{}}
	case "fuzzyFileSearch":
		data = map[string]any{"files": []any{}}
	case "externalAgentConfig/detect":
		data = map[string]any{"items": []any{}}
	case "externalAgentConfig/import":
		data = map[string]any{"importId": "import-1"}
	default:
		t.Fatalf("unrecognized plain path method %q", method)
	}
	return transport.SetResponseData(method, data)
}
