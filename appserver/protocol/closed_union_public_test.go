package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func callClosedSourceCarrier(client *p.Client, method string) (any, error) {
	ctx := context.Background()
	switch method {
	case "thread/start":
		return client.Thread.Start(ctx, p.ThreadStartParams{})
	case "thread/resume":
		return client.Thread.Resume(ctx, p.ThreadResumeParams{ThreadID: "closed-thread"})
	case "thread/fork":
		return client.Thread.Fork(ctx, p.ThreadForkParams{ThreadID: "closed-thread"})
	case "thread/list":
		return client.Thread.List(ctx, p.ThreadListParams{})
	case "thread/list-json":
		return client.Thread.ListJSON(ctx, p.ThreadListParamsJSON{})
	case "thread/rollback":
		return client.Thread.Rollback(ctx, p.ThreadRollbackParams{ThreadID: "closed-thread", NumTurns: 1})
	case "thread/unarchive":
		return client.Thread.Unarchive(ctx, p.ThreadUnarchiveParams{ThreadID: "closed-thread"})
	case "thread/metadata/update":
		return client.Thread.MetadataUpdate(ctx, p.ThreadMetadataUpdateParams{ThreadID: "closed-thread"})
	case "thread/revert":
		return client.Thread.Revert(ctx, p.ThreadRevertParams{ThreadID: "closed-thread", BeforeTurnID: "turn"})
	default:
		return client.Thread.Read(ctx, p.ThreadReadParams{ThreadID: "closed-thread"})
	}
}

func setClosedCarrierResponse(t *testing.T, mock *MockTransport, method, kind string, raw json.RawMessage) {
	t.Helper()
	thread := validProcessThreadPayload("closed-thread")
	if kind == "source" {
		thread["source"] = raw
	}
	var body map[string]any
	switch method {
	case "thread/start", "thread/resume", "thread/fork":
		body = validThreadLifecycleResponse(thread)
		if kind == "policy" {
			body["approvalPolicy"] = raw
		}
	case "thread/list", "thread/list-json":
		body = map[string]any{"data": []any{validProcessThreadPayload("other-good"), thread}}
		method = "thread/list"
	default:
		body = map[string]any{"thread": thread}
	}
	if err := mock.SetResponseData(method, body); err != nil {
		t.Fatal(err)
	}
}

func TestClosedUnionResponseAdmissionAndRecovery(t *testing.T) {
	for _, kind := range []string{"source", "policy"} {
		methods := []string{"thread/start", "thread/resume", "thread/fork"}
		if kind == "source" {
			methods = append(methods, "thread/read", "thread/list", "thread/list-json", "thread/rollback", "thread/unarchive", "thread/metadata/update", "thread/revert")
		}
		for _, method := range methods {
			t.Run(kind+"/"+method, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				defer client.Close()
				client.CacheThreadState(p.Thread{ID: "closed-thread", Name: p.Ptr("prior"), Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}})
				updates := 0
				remove := client.AddThreadStateListener("closed-thread", func(p.Thread) { updates++ }, nil)
				defer remove()
				updates = 0
				bad := json.RawMessage(`{"custom":"bad","extra":true}`)
				good := json.RawMessage(`{"custom":"recovered"}`)
				if kind == "policy" {
					bad = json.RawMessage(`{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"extra":true}`)
					good = json.RawMessage(`{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false,"extra":{"allowed":true}}}`)
				}
				setClosedCarrierResponse(t, mock, method, kind, bad)
				result, err := callClosedSourceCarrier(client, method)
				if err == nil || !reflect.ValueOf(result).IsZero() || updates != 0 {
					t.Fatalf("closed object published: method=%s updates=%d err=%v", method, updates, err)
				}
				snapshot, present := client.ThreadStateSnapshot("closed-thread")
				if !present || snapshot.Name == nil || *snapshot.Name != "prior" || snapshot.Source.Value != p.SessionSourceCLI {
					t.Fatal("failed response changed existing cache state")
				}
				if _, present := client.ThreadStateSnapshot("other-good"); present {
					t.Fatal("list failure partially published preceding valid entry")
				}
				setClosedCarrierResponse(t, mock, method, kind, good)
				if _, err := callClosedSourceCarrier(client, method); err != nil || updates != 1 {
					t.Fatalf("same-client valid recovery failed: updates=%d err=%v", updates, err)
				}
			})
		}
	}
}

func TestClosedApprovalConfigCarriers(t *testing.T) {
	for _, method := range []string{"config/read", "configRequirements/read"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			for _, invalid := range []bool{true, false} {
				policy := `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false,"extra":42}`
				if invalid {
					policy += `,"extra":true`
				}
				policy += "}"
				body := `{"config":{"approval_policy":` + policy + `},"origins":{}}`
				if method == "configRequirements/read" {
					body = `{"requirements":{"allowedApprovalPolicies":["never",` + policy + `]}}`
				}
				mock.SetResponse(method, p.Response{Result: json.RawMessage(body)})
				var result any
				var err error
				if method == "config/read" {
					result, err = client.Config.Read(context.Background(), p.ConfigReadParams{})
				} else {
					result, err = client.Config.ReadRequirements(context.Background())
				}
				if invalid && (err == nil || !reflect.ValueOf(result).IsZero()) {
					t.Fatal("invalid closed policy entered config response")
				}
				if !invalid && err != nil {
					t.Fatalf("open nested config policy metadata rejected: %v", err)
				}
			}
		})
	}
}

func TestClosedUnionNotificationAdmissionAndRecovery(t *testing.T) {
	for _, kind := range []string{"source", "policy"} {
		t.Run(kind, func(t *testing.T) {
			method := "thread/started"
			if kind == "policy" {
				method = "thread/settings/updated"
			}
			mock := NewMockTransport()
			reports, on, add := 0, 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(got string, _ error) {
				if got != method {
					t.Errorf("diagnostic origin=%s", got)
				}
				reports++
			}))
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "notification-thread", Name: p.Ptr("prior"), Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}, DisabledPluginIDs: []string{"prior"}})
			var remove func()
			if kind == "source" {
				client.OnThreadStarted(func(p.ThreadStartedNotification) { on++ })
				remove = client.AddThreadStartedListener(func(p.ThreadStartedNotification) { add++ })
			} else {
				client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { on++ })
				remove = client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { add++ })
			}
			defer remove()
			for _, invalid := range []bool{true, false} {
				var body map[string]any
				if kind == "source" {
					thread := validProcessThreadPayload("notification-thread")
					thread["source"] = json.RawMessage(`{"custom":"recovered"}`)
					if invalid {
						thread["source"] = json.RawMessage(`{"subAgent":{"other":"bad","extra":true}}`)
					}
					body = map[string]any{"thread": thread}
				} else {
					policy := `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false,"extra":42}`
					if invalid {
						policy += `,"extra":true`
					}
					policy += "}"
					body = map[string]any{"threadId": "notification-thread", "threadSettings": map[string]any{"approvalPolicy": json.RawMessage(policy), "approvalsReviewer": "user", "cwd": "/tmp", "model": "model", "modelProvider": "openai", "sandboxPolicy": map[string]any{"type": "readOnly"}, "collaborationMode": map[string]any{"mode": "default", "settings": map[string]any{"model": "model"}}, "disabledPluginIds": []string{"recovered"}}}
				}
				raw, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				before := reports
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: raw})
				snapshot, present := client.ThreadStateSnapshot("notification-thread")
				if !present {
					t.Fatal("notification lost existing cache entry")
				}
				if invalid {
					if on != 0 || add != 0 || reports <= before || snapshot.Name == nil || *snapshot.Name != "prior" || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"prior"}) || snapshot.Source.Value != p.SessionSourceCLI {
						t.Fatal("invalid notification published callbacks or changed cache")
					}
				} else if on != 1 || add != 1 || reports != before {
					t.Fatal("valid notification recovery failed")
				} else if kind == "source" {
					if source, ok := snapshot.Source.Value.(p.SessionSourceCustom); !ok || source.Custom != "recovered" {
						t.Fatal("valid source notification did not update cache")
					}
				} else if !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"recovered"}) {
					t.Fatal("valid settings notification did not update cache")
				}
			}
		})
	}
}
