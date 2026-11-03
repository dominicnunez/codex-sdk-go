package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func pluginNullCarrier(t *testing.T, owner, record string) json.RawMessage {
	t.Helper()
	var detail map[string]json.RawMessage
	for _, r := range requiredNullRecords() {
		if r.name == "PluginDetail" {
			if err := json.Unmarshal([]byte(r.body), &detail); err != nil {
				t.Fatal(err)
			}
		}
	}
	switch owner {
	case "PluginDetail":
		return json.RawMessage(`{"plugin":` + record + `}`)
	case "SkillSummary":
		detail["skills"] = json.RawMessage("[" + record + "]")
	case "AppSummary":
		detail["apps"] = json.RawMessage("[" + record + "]")
	case "PluginSummary":
		detail["summary"] = json.RawMessage(record)
	default:
		var summary map[string]json.RawMessage
		if err := json.Unmarshal(detail["summary"], &summary); err != nil {
			t.Fatal(err)
		}
		share := record
		if owner == "PluginSharePrincipal" {
			share = `{"remotePluginId":"remote","sharePrincipals":[` + record + `]}`
		}
		summary["shareContext"] = json.RawMessage(share)
		raw, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		detail["summary"] = raw
	}
	raw, err := json.Marshal(map[string]any{"plugin": detail})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRequiredNullPublicRecovery(t *testing.T) {
	records := append(requiredNullRecords(), requiredNullRecord{"SkillSummary", `{"description":"description","enabled":true,"name":"skill"}`, []string{"description", "enabled", "name"}, func() any { return new(p.SkillSummary) }})
	for _, record := range records {
		for _, field := range record.fields {
			t.Run(record.name+"/"+field, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				defer client.Close()
				method := "plugin/read"
				call := func() (any, error) {
					return client.Plugin.Read(context.Background(), p.PluginReadParams{PluginName: "plugin"})
				}
				updates := 0
				if record.name == "Thread" {
					method = "thread/read"
					client.CacheThreadState(p.Thread{ID: "thread", Name: p.Ptr("cached"), Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}})
					remove := client.AddThreadStateListener("thread", func(p.Thread) { updates++ }, nil)
					defer remove()
					updates = 0
					call = func() (any, error) {
						return client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "thread"})
					}
				}
				for _, invalid := range []bool{true, false} {
					body := record.body
					if invalid {
						body = hiddenNullBody(body, field, true)
					}
					var raw json.RawMessage
					if record.name == "Thread" {
						raw = json.RawMessage(`{"thread":` + body + `}`)
					} else {
						raw = pluginNullCarrier(t, record.name, body)
					}
					mock.SetResponse(method, p.Response{Result: raw})
					result, err := call()
					if invalid {
						if !errors.Is(err, p.ErrNullResultField) || !reflect.ValueOf(result).IsZero() {
							t.Fatalf("invalid response published: %v", err)
						}
						if record.name == "Thread" {
							snapshot, ok := client.ThreadStateSnapshot("thread")
							if !ok || snapshot.Name == nil || *snapshot.Name != "cached" || updates != 0 {
								t.Fatal("invalid response changed cache/listener state")
							}
						}
					} else if err != nil {
						t.Fatalf("same-client recovery: %v", err)
					}
				}
				if len(mock.SentRequests) != 2 {
					t.Fatal("recovery did not issue a second request")
				}
				if record.name == "Thread" && updates != 1 {
					t.Fatal("valid recovery did not publish exactly once")
				}
			})
		}
	}
}

func TestRequiredNullThreadNotificationRecovery(t *testing.T) {
	record := requiredNullRecords()[0]
	for _, field := range record.fields {
		t.Run(field, func(t *testing.T) {
			mock := NewMockTransport()
			reports, on, add := 0, 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
				if method != "thread/started" {
					t.Errorf("diagnostic origin=%s", method)
				}
				reports++
			}))
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "thread", Name: p.Ptr("cached"), Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}})
			client.OnThreadStarted(func(p.ThreadStartedNotification) { on++ })
			remove := client.AddThreadStartedListener(func(p.ThreadStartedNotification) { add++ })
			defer remove()
			for _, invalid := range []bool{true, false} {
				body := record.body
				if invalid {
					body = hiddenNullBody(body, field, true)
				}
				before := reports
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/started", Params: json.RawMessage(`{"thread":` + body + `}`)})
				snapshot, present := client.ThreadStateSnapshot("thread")
				if !present || snapshot.Name == nil {
					t.Fatal("cached thread lost")
				}
				if invalid {
					if on != 0 || add != 0 || reports <= before || *snapshot.Name != "cached" {
						t.Fatal("invalid notification published or changed cache")
					}
				} else if on != 1 || add != 1 || reports != before || *snapshot.Name != "prior" {
					t.Fatal("valid notification recovery failed")
				}
			}
		})
	}
}

func TestRequiredNullSettingsNotificationRecovery(t *testing.T) {
	mock := NewMockTransport()
	reports, on, add := 0, 0, 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
		if method != "thread/settings/updated" {
			t.Errorf("diagnostic origin=%s", method)
		}
		reports++
	}))
	defer client.Close()
	client.CacheThreadState(p.Thread{ID: "thread", DisabledPluginIDs: []string{"prior"}})
	client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { on++ })
	remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { add++ })
	defer remove()
	for _, invalid := range []bool{true, false} {
		policy := `{"granular":{"mcp_elicitations":false,"rules":true,"sandbox_approval":false}}`
		if invalid {
			policy = `{"granular":{"rules":null},"granular":{"mcp_elicitations":false,"rules":true,"sandbox_approval":false}}`
		}
		params := `{"threadId":"thread","threadSettings":{"approvalPolicy":` + policy + `,"approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandboxPolicy":{"type":"readOnly"},"collaborationMode":{"mode":"default","settings":{"model":"model"}},"disabledPluginIds":["recovered"]}}`
		before := reports
		mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: json.RawMessage(params)})
		snapshot, ok := client.ThreadStateSnapshot("thread")
		if !ok {
			t.Fatal("cached thread lost")
		}
		if invalid {
			if on != 0 || add != 0 || reports <= before || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"prior"}) {
				t.Fatal("invalid settings published or changed cache")
			}
		} else if on != 1 || add != 1 || reports != before || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"recovered"}) {
			t.Fatal("valid settings recovery failed")
		}
	}
}
