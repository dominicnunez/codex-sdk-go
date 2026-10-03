package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const issue74PluginSummary = `{"authPolicy":"ON_USE","enabled":true,"id":"plugin-74","installPolicy":"AVAILABLE","installed":true,"name":"calendar","source":{"type":"remote"},"disabledReason":"plan_not_eligible","eligiblePlanTypes":["team","enterprise"],"installPolicySource":"IMPLICIT_CANONICAL_APP","installedAt":1728000000,"mustShowInstallationInterstitial":false,"version":"2.0","interface":{"capabilities":[],"screenshots":[],"screenshotUrls":[],"logoDark":"/plugins/dark.png","logoUrlDark":"https://example.test/dark.png"},"shareContext":{"remotePluginId":"remote-74","canPublishToWorkspace":false}}`

func issue74PluginDetail() map[string]any {
	var summary map[string]any
	_ = json.Unmarshal([]byte(issue74PluginSummary), &summary)
	return map[string]any{
		"apps":         []any{map[string]any{"id": "app-74", "name": "Calendar", "needsAuth": true, "category": "productivity"}},
		"appTemplates": []any{map[string]any{"name": "Template", "templateId": "template-74", "materializedAppIds": []any{"app-74"}, "canonicalConnectorId": "calendar", "category": "productivity", "description": "Calendar template", "logoUrl": "https://example.test/light.png", "logoUrlDark": "https://example.test/dark.png", "reason": "NOT_CONFIGURED_FOR_WORKSPACE"}},
		"hooks":        []any{}, "marketplaceName": "official", "mcpServers": []any{},
		"skills":  []any{map[string]any{"description": "Calendar", "enabled": true, "name": "book", "interface": map[string]any{"iconSmallUrl": "https://example.test/small.png", "iconLargeUrl": "https://example.test/large.png"}}},
		"summary": summary, "shareUrl": "https://example.test/share",
		"scheduledTasks": []any{map[string]any{"key": "task-74", "name": "Morning", "prompt": "Check calendar", "schedule": map[string]any{"type": "weekly", "time": "09:30", "days": []any{"MO", "FR"}}}},
	}
}

func issue74Read(t *testing.T, detail map[string]any) (codex.PluginReadResponse, error) {
	t.Helper()
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	data, err := json.Marshal(map[string]any{"plugin": detail})
	if err != nil {
		t.Fatal(err)
	}
	transport.SetResponse("plugin/read", codex.Response{JSONRPC: "2.0", Result: data})
	return client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", RemoteMarketplaceName: issue74String("official")})
}

func issue74String(s string) *string { return &s }

func issue74JSON(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIssue74PluginResponsePreservesFields(t *testing.T) {
	detail := issue74PluginDetail()
	resp, err := issue74Read(t, detail)
	if err != nil {
		t.Fatal(err)
	}
	got := issue74JSON(t, resp.Plugin)
	for _, field := range []string{"appTemplates", "scheduledTasks", "shareUrl"} {
		if !reflect.DeepEqual(got[field], detail[field]) {
			t.Errorf("%s = %#v; want %#v", field, got[field], detail[field])
		}
	}
	if got["apps"].([]any)[0].(map[string]any)["category"] != "productivity" {
		t.Error("app category lost")
	}
	ui := got["skills"].([]any)[0].(map[string]any)["interface"].(map[string]any)
	for _, field := range []string{"iconSmallUrl", "iconLargeUrl"} {
		if ui[field] == nil {
			t.Errorf("skill %s lost", field)
		}
	}
	for _, method := range []string{"plugin/list", "plugin/installed", "plugin/share/list"} {
		t.Run(method, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			payload := `{"marketplaces":[{"name":"official","plugins":[` + issue74PluginSummary + `]}]}`
			if method == "plugin/share/list" {
				payload = `{"data":[{"plugin":` + issue74PluginSummary + `}]}`
			}
			transport.SetResponse(method, codex.Response{JSONRPC: "2.0", Result: json.RawMessage(payload)})
			var summary codex.PluginSummary
			switch method {
			case "plugin/list":
				r, e := client.Plugin.List(context.Background(), codex.PluginListParams{})
				if e != nil {
					t.Fatal(e)
				}
				summary = r.Marketplaces[0].Plugins[0]
			case "plugin/installed":
				r, e := client.Plugin.Installed(context.Background(), codex.PluginInstalledParams{})
				if e != nil {
					t.Fatal(e)
				}
				summary = r.Marketplaces[0].Plugins[0]
			case "plugin/share/list":
				r, e := client.Plugin.ShareList(context.Background(), codex.PluginShareListParams{})
				if e != nil {
					t.Fatal(e)
				}
				summary = r.Data[0].Plugin
			}
			want := detail["summary"].(map[string]any)
			actual := issue74JSON(t, summary)
			for _, field := range []string{"disabledReason", "eligiblePlanTypes", "installPolicySource", "installedAt", "mustShowInstallationInterstitial", "version", "interface", "shareContext"} {
				if !reflect.DeepEqual(actual[field], want[field]) {
					t.Errorf("%s = %#v; want %#v", field, actual[field], want[field])
				}
			}
		})
	}
}

func TestIssue74PluginSchemaRequiredBranches(t *testing.T) {
	for _, field := range []string{"appTemplates", "materializedAppIds", "templateId", "name", "key", "taskName", "prompt", "schedule", "intervalHours", "time", "days"} {
		for _, null := range []bool{false, true} {
			t.Run(field+map[bool]string{false: "/absent", true: "/null"}[null], func(t *testing.T) {
				detail := issue74PluginDetail()
				target := detail
				switch field {
				case "materializedAppIds", "templateId", "name":
					target = detail["appTemplates"].([]any)[0].(map[string]any)
				case "key", "prompt", "schedule":
					target = detail["scheduledTasks"].([]any)[0].(map[string]any)
				case "taskName":
					target = detail["scheduledTasks"].([]any)[0].(map[string]any)
				case "intervalHours":
					target = map[string]any{"type": "hourly", "intervalHours": float64(2)}
					detail["scheduledTasks"].([]any)[0].(map[string]any)["schedule"] = target
				case "time", "days":
					target = detail["scheduledTasks"].([]any)[0].(map[string]any)["schedule"].(map[string]any)
				}
				wireField := field
				if field == "taskName" {
					wireField = "name"
				}
				if null {
					target[wireField] = nil
				} else {
					delete(target, wireField)
				}
				if _, err := issue74Read(t, detail); err == nil {
					t.Fatal("expected schema required-field error")
				}
			})
		}
	}
}

func TestIssue74PluginAppNeedsAuthOptional(t *testing.T) {
	detail := issue74PluginDetail()
	delete(detail["apps"].([]any)[0].(map[string]any), "needsAuth")
	if _, err := issue74Read(t, detail); err != nil {
		t.Fatal(err)
	}
}

func TestIssue74PluginScheduleVariants(t *testing.T) {
	for _, raw := range []string{`{"type":"hourly","intervalHours":0}`, `{"type":"hourly","intervalHours":4294967295,"days":[]}`, `{"type":"daily","time":"09:00"}`, `{"type":"weekdays","time":"17:00"}`, `{"type":"weekly","time":"10:00","days":[]}`} {
		t.Run(raw, func(t *testing.T) {
			var schedule map[string]any
			if err := json.Unmarshal([]byte(raw), &schedule); err != nil {
				t.Fatal(err)
			}
			detail := issue74PluginDetail()
			detail["scheduledTasks"].([]any)[0].(map[string]any)["schedule"] = schedule
			resp, err := issue74Read(t, detail)
			if err != nil {
				t.Fatal(err)
			}
			got := issue74JSON(t, resp.Plugin)["scheduledTasks"].([]any)[0].(map[string]any)["schedule"].(map[string]any)
			for key, want := range schedule {
				if !reflect.DeepEqual(got[key], want) {
					t.Errorf("schedule.%s = %#v; want %#v", key, got[key], want)
				}
			}
		})
	}
	for _, raw := range []string{`{"type":"daily"}`, `{"type":"weekdays","time":null}`, `{"type":"hourly","intervalHours":-1}`, `{"type":"hourly","intervalHours":4294967296}`, `{"type":"hourly","intervalHours":1.5}`, `{"type":"weekly","time":"10:00","days":["MON"]}`, `{"type":"hourly","intervalHours":2,"days":["MON"]}`, `{"type":"yearly"}`, `{"time":"10:00"}`, `null`} {
		t.Run("invalid/"+raw, func(t *testing.T) {
			var schedule any
			if err := json.Unmarshal([]byte(raw), &schedule); err != nil {
				t.Fatal(err)
			}
			detail := issue74PluginDetail()
			detail["scheduledTasks"].([]any)[0].(map[string]any)["schedule"] = schedule
			if _, err := issue74Read(t, detail); err == nil {
				t.Fatal("expected invalid schedule error")
			}
		})
	}
}

func TestIssue74PluginOptionalAndInvalidMetadata(t *testing.T) {
	for _, value := range []any{nil, []any{}} {
		t.Run("nullable slices", func(t *testing.T) {
			detail := issue74PluginDetail()
			detail["scheduledTasks"] = value
			detail["summary"].(map[string]any)["eligiblePlanTypes"] = value
			resp, err := issue74Read(t, detail)
			if err != nil {
				t.Fatal(err)
			}
			got := issue74JSON(t, resp.Plugin)
			if !reflect.DeepEqual(got["scheduledTasks"], value) {
				t.Errorf("scheduledTasks = %#v; want %#v", got["scheduledTasks"], value)
			}
			if !reflect.DeepEqual(got["summary"].(map[string]any)["eligiblePlanTypes"], value) {
				t.Errorf("eligiblePlanTypes lost empty/null distinction")
			}
		})
	}
	for _, field := range []string{"disabledReason", "installPolicySource", "logoDark", "reason"} {
		t.Run("invalid/"+field, func(t *testing.T) {
			detail := issue74PluginDetail()
			target := detail["summary"].(map[string]any)
			if field == "logoDark" {
				target = target["interface"].(map[string]any)
			}
			if field == "reason" {
				target = detail["appTemplates"].([]any)[0].(map[string]any)
			}
			target[field] = "invalid"
			if _, err := issue74Read(t, detail); err == nil {
				t.Fatal("expected invalid metadata error")
			}
		})
	}
}

func TestIssue74PluginOptionalNonNullAndPathValidation(t *testing.T) {
	for _, field := range []string{"availability", "keywords"} {
		t.Run(field, func(t *testing.T) {
			detail := issue74PluginDetail()
			detail["summary"].(map[string]any)[field] = nil
			if _, err := issue74Read(t, detail); err == nil {
				t.Fatal("explicit null must fail for optional non-null field")
			}
		})
	}
	for _, path := range []string{"dark.png", "/plugins/../dark.png", "/plugins/dark.png"} {
		t.Run(path, func(t *testing.T) {
			detail := issue74PluginDetail()
			detail["summary"].(map[string]any)["interface"].(map[string]any)["logoDark"] = path
			resp, err := issue74Read(t, detail)
			if path == "/plugins/dark.png" {
				if err != nil {
					t.Fatal(err)
				}
				if resp.Plugin.Summary.Interface.LogoDark == nil || *resp.Plugin.Summary.Interface.LogoDark != path {
					t.Fatal("logoDark lost")
				}
			} else if err == nil {
				t.Fatal("expected absolute normalized path rejection")
			}
		})
	}
}

func TestIssue74PluginSharedMetadataBoundaries(t *testing.T) {
	t.Run("install app metadata", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("plugin/install", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"authPolicy":"ON_USE","appsNeedingAuth":[{"id":"app-74","name":"Calendar","category":"productivity"}]}`)})
		resp, err := client.Plugin.Install(context.Background(), codex.PluginInstallParams{PluginName: "calendar", RemoteMarketplaceName: issue74String("official")})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.AppsNeedingAuth) != 1 || resp.AppsNeedingAuth[0].Category == nil || *resp.AppsNeedingAuth[0].Category != "productivity" || resp.AppsNeedingAuth[0].NeedsAuth {
			t.Fatalf("unexpected app metadata: %+v", resp)
		}
	})
	t.Run("skills list URL metadata", func(t *testing.T) {
		transport := NewMockTransport()
		client := codex.NewClient(transport)
		transport.SetResponse("skills/list", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[{"cwd":"/workspace","errors":[],"skills":[{"name":"book","description":"Calendar","enabled":true,"path":"/workspace/SKILL.md","scope":"repo","interface":{"iconLargeUrl":"https://example.test/large.png","iconSmallUrl":"https://example.test/small.png"}}]}]}`)})
		resp, err := client.Skills.List(context.Background(), codex.SkillsListParams{})
		if err != nil {
			t.Fatal(err)
		}
		ui := resp.Data[0].Skills[0].Interface
		if ui == nil || ui.IconLargeURL == nil || ui.IconSmallURL == nil || *ui.IconLargeURL != "https://example.test/large.png" || *ui.IconSmallURL != "https://example.test/small.png" {
			t.Fatalf("URL metadata lost: %+v", ui)
		}
	})
}

func TestIssue74PluginConcreteScheduleDiscriminators(t *testing.T) {
	for _, schedule := range []any{&codex.HourlyScheduledTaskSchedule{}, &codex.DailyScheduledTaskSchedule{}, &codex.WeekdaysScheduledTaskSchedule{}, &codex.WeeklyScheduledTaskSchedule{}} {
		t.Run(reflect.TypeOf(schedule).Elem().Name(), func(t *testing.T) {
			if err := json.Unmarshal([]byte(`{"type":"other","intervalHours":2,"days":[],"time":"09:00"}`), schedule); err == nil {
				t.Fatal("concrete schedule accepted mismatched discriminator")
			}
		})
	}
}

func TestIssue74PluginConstructedRequiredValues(t *testing.T) {
	for _, value := range []any{codex.AppTemplateSummary{}, codex.WeeklyScheduledTaskSchedule{}, codex.ScheduledTaskSummary{}, codex.ScheduledTaskSummary{Schedule: (*codex.DailyScheduledTaskSchedule)(nil)}} {
		t.Run(reflect.TypeOf(value).Name(), func(t *testing.T) {
			if _, err := json.Marshal(value); err == nil {
				t.Fatal("expected required non-null value rejection")
			}
		})
	}
	for _, value := range []any{codex.AppTemplateSummary{MaterializedAppIDs: []string{}}, codex.WeeklyScheduledTaskSchedule{Days: []codex.ScheduledTaskWeekday{}}, codex.ScheduledTaskSummary{Schedule: codex.DailyScheduledTaskSchedule{Time: "09:00"}}} {
		t.Run("valid/"+reflect.TypeOf(value).Name(), func(t *testing.T) {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			decoded := reflect.New(reflect.TypeOf(value)).Interface()
			if err := json.Unmarshal(encoded, decoded); err != nil {
				t.Fatalf("constructed value failed decode: %s: %v", encoded, err)
			}
		})
	}
}

func TestIssue74PluginConstructedEnums(t *testing.T) {
	for _, value := range []any{codex.PluginDisabledReason("invalid"), codex.PluginInstallPolicySource("invalid"), codex.AppTemplateUnavailableReason("invalid"), codex.ScheduledTaskWeekday("invalid")} {
		t.Run(reflect.TypeOf(value).Name(), func(t *testing.T) {
			if _, err := json.Marshal(value); err == nil {
				t.Fatal("expected invalid constructed enum rejection")
			}
		})
	}
}
