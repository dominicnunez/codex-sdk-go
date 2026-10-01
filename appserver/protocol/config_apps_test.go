package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestConfigReadPreservesApps(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{
		"config":{"apps":{
			"_default":{"enabled":false,"destructive_enabled":false,"open_world_enabled":false,"default_tools_approval_mode":"prompt","approvals_reviewer":"user"},
			"connector-a":{"enabled":false,"omit_tools_from":["code_mode","deferred","direct"],"default_tools_enabled":false,"destructive_enabled":false,"open_world_enabled":false,"default_tools_approval_mode":"writes","approvals_reviewer":"auto_review","tools":{"write":{"enabled":false,"approval_mode":"approve"}},"links":{"account-1":{"default_tools_approval_mode":"auto","approvals_reviewer":"guardian_subagent"}}},
			"connector-b":{"enabled":true,"omit_tools_from":[]}
		}},"origins":{}
	}`)})
	response, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	// Observe retention through the public read result before depending on new types.
	encoded, err := json.Marshal(response.Config)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &config); err != nil {
		t.Fatal(err)
	}
	if _, ok := config["apps"]; !ok {
		t.Fatalf("Config.Read discarded apps: %s", encoded)
	}
	apps := response.Config.Apps
	if apps == nil || apps.Default == nil || len(apps.Apps) != 2 {
		t.Fatalf("apps/defaults missing: %+v", apps)
	}
	app := apps.Apps["connector-a"]
	wantSurfaces := []codex.ToolExposureSurface{codex.ToolExposureSurfaceCodeMode, codex.ToolExposureSurfaceDeferred, codex.ToolExposureSurfaceDirect}
	if app.Enabled || app.OmitToolsFrom == nil || !reflect.DeepEqual(*app.OmitToolsFrom, wantSurfaces) {
		t.Fatalf("app enabled/exposure = %+v", app)
	}
	if app.Tools == nil || (*app.Tools)["write"].ApprovalMode == nil || *(*app.Tools)["write"].ApprovalMode != codex.AppToolApprovalApprove {
		t.Fatalf("tool settings lost: %+v", app.Tools)
	}
	if app.Links == nil || (*app.Links)["account-1"].ApprovalsReviewer == nil || *(*app.Links)["account-1"].ApprovalsReviewer != codex.ApprovalsReviewerGuardianSubagent {
		t.Fatalf("account settings lost: %+v", app.Links)
	}
	// Every supplied property must survive the custom flattened maps and scalars.
	var want, got interface{}
	wantJSON := `{"_default":{"enabled":false,"destructive_enabled":false,"open_world_enabled":false,"default_tools_approval_mode":"prompt","approvals_reviewer":"user"},"connector-a":{"enabled":false,"omit_tools_from":["code_mode","deferred","direct"],"default_tools_enabled":false,"destructive_enabled":false,"open_world_enabled":false,"default_tools_approval_mode":"writes","approvals_reviewer":"auto_review","tools":{"write":{"enabled":false,"approval_mode":"approve"}},"links":{"account-1":{"default_tools_approval_mode":"auto","approvals_reviewer":"guardian_subagent"}}},"connector-b":{"enabled":true,"omit_tools_from":[]}}`
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(config["apps"], &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apps wire shape changed: %s", config["apps"])
	}
}

func readAppsConfig(t *testing.T, raw string) (*codex.Config, error) {
	t.Helper()
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	transport.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{"config":` + raw + `,"origins":{}}`)})
	response, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
	return response.Config, err
}

func TestConfigAppsDefaultsAndOptionalValues(t *testing.T) {
	for _, raw := range []string{`{}`, `{"apps":null}`} {
		config, err := readAppsConfig(t, raw)
		if err != nil || config.Apps != nil {
			t.Fatalf("optional apps: config=%+v, err=%v", config, err)
		}
	}
	config, err := readAppsConfig(t, `{"apps":{"_default":{},"app":{},"empty":{"tools":{},"links":{},"omit_tools_from":[]},"nullable":{"approvals_reviewer":null,"destructive_enabled":null,"open_world_enabled":null,"default_tools_enabled":null,"default_tools_approval_mode":null,"tools":null,"links":null,"omit_tools_from":null}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defaults := config.Apps.Default
	if defaults == nil || !defaults.Enabled || !defaults.DestructiveEnabled || !defaults.OpenWorldEnabled {
		t.Fatalf("omitted defaults must be enabled: %+v", defaults)
	}
	app := config.Apps.Apps["app"]
	if !app.Enabled || app.DestructiveEnabled != nil || app.OpenWorldEnabled != nil || app.DefaultToolsEnabled != nil {
		t.Fatalf("app defaults changed: %+v", app)
	}
	empty := config.Apps.Apps["empty"]
	if empty.Tools == nil || len(*empty.Tools) != 0 || empty.Links == nil || len(*empty.Links) != 0 || empty.OmitToolsFrom == nil || len(*empty.OmitToolsFrom) != 0 {
		t.Fatalf("explicit empty values lost: %+v", empty)
	}
	nullable := config.Apps.Apps["nullable"]
	if !nullable.Enabled || nullable.OmitToolsFrom != nil || nullable.Tools != nil || nullable.Links != nil || nullable.ApprovalsReviewer != nil {
		t.Fatalf("nullable values changed: %+v", nullable)
	}
	config, err = readAppsConfig(t, `{"apps":{"_default":null,"app":{"tools":{"empty":{}},"links":{"empty":{}}}}}`)
	if err != nil || config.Apps.Default != nil {
		t.Fatalf("null defaults: config=%+v, err=%v", config, err)
	}
	app = config.Apps.Apps["app"]
	if tool := (*app.Tools)["empty"]; tool.Enabled != nil || tool.ApprovalMode != nil {
		t.Fatalf("empty tool settings changed: %+v", tool)
	}
	if link := (*app.Links)["empty"]; link.ApprovalsReviewer != nil || link.DefaultToolsApprovalMode != nil {
		t.Fatalf("empty link settings changed: %+v", link)
	}
}

func TestConfigReadRejectsMalformedApps(t *testing.T) {
	for _, apps := range []string{
		`[]`, `false`, `{"app":null}`, `{"app":[]}`, `{"_default":false}`,
		`{"_default":{"enabled":null}}`, `{"_default":{"destructive_enabled":null}}`, `{"_default":{"open_world_enabled":null}}`,
		`{"app":{"enabled":null}}`, `{"app":{"enabled":"false"}}`,
		`{"app":{"omit_tools_from":["unknown"]}}`, `{"app":{"omit_tools_from":[null]}}`, `{"app":{"omit_tools_from":"direct"}}`,
		`{"_default":{"default_tools_approval_mode":"unknown"}}`, `{"app":{"default_tools_approval_mode":"unknown"}}`,
		`{"app":{"tools":[]}}`, `{"app":{"tools":{"tool":null}}}`, `{"app":{"tools":{"tool":{"approval_mode":"unknown"}}}}`,
		`{"app":{"links":[]}}`, `{"app":{"links":{"account":null}}}`, `{"app":{"links":{"account":{"default_tools_approval_mode":"unknown"}}}}`,
		`{"app":{"approvals_reviewer":"unknown"}}`, `{"app":{"links":{"account":{"approvals_reviewer":"unknown"}}}}`,
	} {
		t.Run(apps, func(t *testing.T) {
			if _, err := readAppsConfig(t, `{"apps":`+apps+`}`); err == nil {
				t.Fatal("malformed apps accepted through Config.Read")
			}
		})
	}
}

func TestAppsConfigRejectsReservedKeyAndInvalidEnums(t *testing.T) {
	if _, err := json.Marshal(codex.AppsConfig{Apps: map[string]codex.AppConfig{"_default": {}}}); err == nil {
		t.Fatal("reserved app ID silently overwrote defaults")
	}
	for _, value := range []interface{}{codex.AppToolApproval("unknown"), codex.ToolExposureSurface("unknown")} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatalf("invalid enum %v serialized", value)
		}
	}
}
