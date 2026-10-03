package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const publicAdmissionGoal = `{"createdAt":0,"objective":"","status":"active","threadId":"","timeUsedSeconds":0,"tokensUsed":0,"updatedAt":0}`

func TestSyncedServiceAdmission(t *testing.T) {
	for _, tc := range []struct {
		method, valid, invalid string
		call                   func(context.Context, *codex.Client) error
	}{
		{"app/read", `{"apps":[],"missingAppIds":[]}`, `{"apps":[{}],"missingAppIds":[]}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Apps.Read(ctx, codex.AppsReadParams{})
			return err
		}},
		{"app/installed", `{"apps":[]}`, `{"apps":[null]}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Apps.Installed(ctx, codex.AppsInstalledParams{})
			return err
		}},
		{"account/workspaceMessages/read", `{"featureEnabled":false,"messages":[]}`, `{"featureEnabled":false,"messages":[{}]}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Account.GetWorkspaceMessages(ctx)
			return err
		}},
		{"thread/goal/set", `{"goal":` + publicAdmissionGoal + `}`, `{"goal":{}}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Thread.GoalSet(ctx, codex.ThreadGoalSetParams{})
			return err
		}},
		{"thread/goal/clear", `{"cleared":false}`, `{"cleared":null}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Thread.GoalClear(ctx, codex.ThreadGoalClearParams{})
			return err
		}},
		{"threadSection/create", `{"section":{"id":"","name":""}}`, `{"section":{}}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Thread.SectionCreate(ctx, codex.ThreadSectionCreateParams{})
			return err
		}},
		{"threadSection/list", `{"data":[]}`, `{"data":[null]}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Thread.SectionList(ctx, codex.ThreadSectionListParams{})
			return err
		}},
		{"threadSection/update", `{"section":{"id":"","name":""}}`, `{"section":{"id":""}}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.Thread.SectionUpdate(ctx, codex.ThreadSectionUpdateParams{})
			return err
		}},
		{"externalAgentConfig/import/recordHistory", `{"importId":""}`, `{"importId":null}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.ExternalAgent.RecordImportHistory(ctx, codex.ExternalAgentConfigImportHistoryRecordParams{})
			return err
		}},
		{"externalAgentConfig/import/readHistories", `{"connectors":[],"data":[]}`, `{"connectors":[{}],"data":[]}`, func(ctx context.Context, c *codex.Client) error {
			_, err := c.ExternalAgent.ImportHistories(ctx)
			return err
		}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			for _, payload := range []string{"{}", tc.invalid, "[]"} {
				mock.SetResponse(tc.method, codex.Response{Result: json.RawMessage(payload)})
				err := tc.call(context.Background(), client)
				if err == nil || !strings.Contains(err.Error(), tc.method) {
					t.Fatalf("response %s: error = %v", payload, err)
				}
				if payload == "{}" && !errors.Is(err, codex.ErrMissingResultField) {
					t.Fatalf("missing-field classification lost: %v", err)
				}
				if payload == "[]" && !errors.Is(err, codex.ErrResultNotObject) {
					t.Fatalf("object classification lost: %v", err)
				}
			}
			mock.SetResponse(tc.method, codex.Response{Result: json.RawMessage(tc.valid)})
			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf("valid recovery: %v", err)
			}
		})
	}
}

func TestSyncedOptionalNestedServiceAdmission(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, payload := range []string{`{"summary":{},"dailyUsageBuckets":[{}]}`, `{"summary":{},"threadUsage":{}}`, `{"summary":{},"threadUsage":{"estimatedUsageCreditsMicros":0,"groups":[{}],"threadId":""}}`} {
		mock.SetResponse("account/usage/read", codex.Response{Result: json.RawMessage(payload)})
		if _, err := client.Account.GetTokenUsage(context.Background(), nil); err == nil {
			t.Fatalf("invalid usage accepted: %s", payload)
		}
	}
	for _, payload := range []string{`{"summary":{}}`, `{"summary":{},"dailyUsageBuckets":null,"threadUsage":null}`, `{"summary":{},"dailyUsageBuckets":[{"startDate":"","tokens":-1}],"threadUsage":{"estimatedUsageCreditsMicros":-1,"groups":[{"estimatedUsageCreditsMicros":-1}],"threadId":""}}`} {
		mock.SetResponse("account/usage/read", codex.Response{Result: json.RawMessage(payload)})
		if _, err := client.Account.GetTokenUsage(context.Background(), nil); err != nil {
			t.Fatalf("valid usage rejected: %s: %v", payload, err)
		}
	}
	mock.SetResponse("thread/goal/get", codex.Response{Result: json.RawMessage(`{"goal":{}}`)})
	if _, err := client.Thread.GoalGet(context.Background(), codex.ThreadGoalGetParams{}); err == nil {
		t.Fatal("empty goal accepted")
	}
	for _, payload := range []string{"{}", `{"goal":null}`, `{"goal":` + publicAdmissionGoal + `}`} {
		mock.SetResponse("thread/goal/get", codex.Response{Result: json.RawMessage(payload)})
		if _, err := client.Thread.GoalGet(context.Background(), codex.ThreadGoalGetParams{}); err != nil {
			t.Fatalf("valid optional goal: %v", err)
		}
	}
}

func TestSyncedAppToolEnabledDefault(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, tc := range []struct {
		extra   string
		enabled bool
	}{
		{"", true}, {`,"isEnabled":false`, false}, {`,"isEnabled":true`, true},
	} {
		payload := `{"apps":[{"id":"","name":"","toolSummaries":[{"name":"","description":""` + tc.extra + `}]}],"missingAppIds":[]}`
		mock.SetResponse("app/read", codex.Response{Result: json.RawMessage(payload)})
		result, err := client.Apps.Read(context.Background(), codex.AppsReadParams{})
		if err != nil {
			t.Fatal(err)
		}
		if got := (*result.Apps[0].ToolSummaries)[0].IsEnabled; got != tc.enabled {
			t.Fatalf("enabled=%v want%v", got, tc.enabled)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip codex.AppsReadResponse
		if err := json.Unmarshal(encoded, &roundtrip); err != nil {
			t.Fatal(err)
		}
		if got := (*roundtrip.Apps[0].ToolSummaries)[0].IsEnabled; got != tc.enabled {
			t.Fatalf("enabled changed on public response round trip: %s", encoded)
		}
	}
	var tool codex.AppToolSummary
	for _, payload := range []string{`{"name":"","description":"","isEnabled":false}`, `{"name":"","description":""}`} {
		if err := json.Unmarshal([]byte(payload), &tool); err != nil {
			t.Fatal(err)
		}
	}
	if !tool.IsEnabled {
		t.Fatal("omitted field retained earlier false")
	}
	for _, enabled := range []bool{false, true} {
		constructed := codex.AppToolSummary{Name: "", Description: "", IsEnabled: enabled}
		encoded, err := json.Marshal(constructed)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &members); err != nil {
			t.Fatal(err)
		}
		if raw, ok := members["isEnabled"]; !ok || string(raw) != map[bool]string{false: "false", true: "true"}[enabled] {
			t.Fatalf("explicit bool omitted or changed: %s", encoded)
		}
		var decoded codex.AppToolSummary
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded.IsEnabled != enabled {
			t.Fatalf("constructed roundtrip=%+v %v", decoded, err)
		}
	}
}

func TestSyncedDetectedConnectorContract(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, source := range []string{"remoteMcpServersConfig", "sessionToolUse"} {
		payload := `{"items":[],"connectors":[{"name":"","sessionCount":0,"source":"` + source + `"}]}`
		mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(payload)})
		got, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
		if err != nil || len(got.Connectors) != 1 || string(got.Connectors[0].Source) != source {
			t.Fatalf("detect source %s: %+v %v", source, got, err)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip codex.ExternalAgentConfigDetectResponse
		if err := json.Unmarshal(encoded, &roundtrip); err != nil || string(roundtrip.Connectors[0].Source) != source {
			t.Fatalf("detect roundtrip: %s %v", encoded, err)
		}
	}
	for _, payload := range []string{`{"items":[],"connectors":null}`, `{"items":[],"connectors":[null]}`, `{"items":[],"connectors":[{}]}`, `{"items":[],"connectors":[{"name":"","sessionCount":0,"source":"bad"}]}`, `{"items":[],"connectors":[{"name":"","sessionCount":0,"source":"bad","source":"sessionToolUse"}]}`, `{"items":[],"connectors":[{"name":"","sessionCount":0,"source":"sessionToolUse","source":"bad"}]}`} {
		mock.SetResponse("externalAgentConfig/detect", codex.Response{Result: json.RawMessage(payload)})
		if _, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{}); err == nil {
			t.Fatalf("invalid detect admitted: %s", payload)
		}
	}
	mock.SetResponse("externalAgentConfig/import/readHistories", codex.Response{Result: json.RawMessage(`{"connectors":[{"name":"","sessionCount":0,"source":"sessionToolUse"}],"data":[]}`)})
	if _, err := client.ExternalAgent.ImportHistories(context.Background()); err == nil {
		t.Fatal("detected-only source admitted to imported history")
	}
}

func TestSyncedNotificationAdmission(t *testing.T) {
	for _, tc := range []struct {
		method, invalid, valid string
		on                     func(*codex.Client, func())
		add                    func(*codex.Client, func()) func()
	}{
		{"thread/goal/updated", `{"threadId":"","goal":{}}`, `{"threadId":"","goal":` + publicAdmissionGoal + `}`,
			func(c *codex.Client, h func()) {
				c.OnThreadGoalUpdated(func(codex.ThreadGoalUpdatedNotification) { h() })
			},
			func(c *codex.Client, h func()) func() {
				return c.AddThreadGoalUpdatedListener(func(codex.ThreadGoalUpdatedNotification) { h() })
			}},
		{"externalAgentConfig/import/progress", `{"importId":"","itemTypeResults":[{}]}`, `{"importId":"","itemTypeResults":[{"itemType":"CONFIG","failures":[],"successes":[]}]}`,
			func(c *codex.Client, h func()) {
				c.OnExternalAgentConfigImportProgress(func(codex.ExternalAgentConfigImportProgressNotification) { h() })
			},
			func(c *codex.Client, h func()) func() {
				return c.AddExternalAgentConfigImportProgressListener(func(codex.ExternalAgentConfigImportProgressNotification) { h() })
			}},
		{"externalAgentConfig/import/completed", `{"importId":"","itemTypeResults":[{"itemType":"CONFIG","failures":[{}],"successes":[]}]}`, `{"importId":"","itemTypeResults":[{"itemType":"CONFIG","failures":[],"successes":[{"itemType":"CONFIG"}]}]}`,
			func(c *codex.Client, h func()) {
				c.OnExternalAgentConfigImportCompleted(func(codex.ExternalAgentConfigImportCompletedNotification) { h() })
			},
			func(c *codex.Client, h func()) func() {
				return c.AddExternalAgentConfigImportCompletedListener(func(codex.ExternalAgentConfigImportCompletedNotification) { h() })
			}},
		{"project/changed", `{"projectId":"","changeType":"bad"}`, `{"projectId":"","changeType":"created"}`,
			func(c *codex.Client, h func()) { c.OnProjectChanged(func(codex.ProjectChangedNotification) { h() }) },
			func(c *codex.Client, h func()) func() {
				return c.AddProjectChangedListener(func(codex.ProjectChangedNotification) { h() })
			}},
		{"model/safetyBuffering/updated", `{"threadId":"","turnId":"","model":"","reasons":[null],"useCases":[],"showBufferingUi":false}`, `{"threadId":"","turnId":"","model":"","reasons":[""],"useCases":[],"showBufferingUi":false}`,
			func(c *codex.Client, h func()) {
				c.OnModelSafetyBufferingUpdated(func(codex.ModelSafetyBufferingUpdatedNotification) { h() })
			},
			func(c *codex.Client, h func()) func() {
				return c.AddModelSafetyBufferingUpdatedListener(func(codex.ModelSafetyBufferingUpdatedNotification) { h() })
			}},
		{"item/started", `{"threadId":"","turnId":"","startedAtMs":0,"item":{"type":"mcpToolCall","id":"","server":"","tool":"","status":"inProgress","arguments":null,"appContext":{}}}`, `{"threadId":"","turnId":"","startedAtMs":0,"item":{"type":"mcpToolCall","id":"","server":"","tool":"","status":"inProgress","arguments":null,"appContext":{"connectorId":""}}}`,
			func(c *codex.Client, h func()) { c.OnItemStarted(func(codex.ItemStartedNotification) { h() }) },
			func(c *codex.Client, h func()) func() {
				return c.AddItemStartedListener(func(codex.ItemStartedNotification) { h() })
			}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			for _, appendListener := range []bool{false, true} {
				mock := NewMockTransport()
				callbacks, failures := 0, 0
				client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
					if method != tc.method || err == nil {
						t.Errorf("wrong notification diagnostic: %s %v", method, err)
					}
					failures++
				}))
				t.Cleanup(func() { _ = client.Close() })
				if appendListener {
					t.Cleanup(tc.add(client, func() { callbacks++ }))
				} else {
					tc.on(client, func() { callbacks++ })
				}
				mock.InjectServerNotification(context.Background(), codex.Notification{Method: tc.method, Params: json.RawMessage(tc.invalid)})
				if callbacks != 0 || failures != 1 {
					t.Fatalf("malformed notification callbacks=%d failures=%d", callbacks, failures)
				}
				mock.InjectServerNotification(context.Background(), codex.Notification{Method: tc.method, Params: json.RawMessage(tc.valid)})
				if callbacks != 1 || failures != 1 {
					t.Fatalf("valid recovery callbacks=%d failures=%d", callbacks, failures)
				}
			}
		})
	}
}
