package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const validHookPrompt = `{"type":"hookPrompt","id":"i","fragments":[{"hookRunId":"r","text":"prompt"}],"startedAtMs":0,"completedAtMs":1}`
const invalidHookPrompt = `{"type":"hookPrompt","id":"i","fragments":null}`

func hookPromptTurn(raw string) map[string]any {
	return map[string]any{"id": "turn", "status": "completed", "items": []any{json.RawMessage(raw)}}
}

func TestHookPromptThreadResponseRecovery(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "thread/read", "thread/list", "thread/list-json", "thread/rollback", "thread/unarchive", "thread/metadata/update", "thread/revert"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "closed-thread", Name: p.Ptr("cached")})
			updates := 0
			remove := client.AddThreadStateListener("closed-thread", func(p.Thread) { updates++ }, nil)
			defer remove()
			updates = 0
			for _, invalid := range []bool{true, false} {
				raw := validHookPrompt
				if invalid {
					raw = invalidHookPrompt
				}
				thread := validProcessThreadPayload("closed-thread")
				thread["turns"] = []any{hookPromptTurn(raw)}
				body := map[string]any{"thread": thread}
				wireMethod := method
				switch method {
				case "thread/start", "thread/resume", "thread/fork":
					body = validThreadLifecycleResponse(thread)
				case "thread/list", "thread/list-json":
					wireMethod = "thread/list"
					body = map[string]any{"data": []any{validProcessThreadPayload("other-good"), thread}}
				}
				if err := mock.SetResponseData(wireMethod, body); err != nil {
					t.Fatal(err)
				}
				result, err := callClosedSourceCarrier(client, method)
				if invalid {
					if err == nil || !reflect.ValueOf(result).IsZero() || updates != 0 {
						t.Fatalf("invalid item published: %v", err)
					}
					snapshot, ok := client.ThreadStateSnapshot("closed-thread")
					if !ok || snapshot.Name == nil || *snapshot.Name != "cached" {
						t.Fatal("failed response changed cache")
					}
					if _, ok := client.ThreadStateSnapshot("other-good"); ok {
						t.Fatal("list partially published preceding entry")
					}
				} else if err != nil || updates != 1 {
					t.Fatalf("valid recovery updates=%d err=%v", updates, err)
				}
			}
		})
	}
}

func TestHookPromptHistoryAndTurnCarriers(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		method string
		call   func(*p.Client) (any, error)
	}{
		{"thread/items/list", func(c *p.Client) (any, error) {
			return c.Thread.ItemsList(ctx, p.ThreadItemsListParams{ThreadID: "hook-thread"})
		}},
		{"thread/turns/list", func(c *p.Client) (any, error) {
			return c.Thread.TurnsList(ctx, p.ThreadTurnsListParams{ThreadID: "hook-thread"})
		}},
		{"turn/start", func(c *p.Client) (any, error) {
			return c.Turn.Start(ctx, p.TurnStartParams{ThreadID: "hook-thread", Input: []p.UserInput{}})
		}},
		{"review/start", func(c *p.Client) (any, error) {
			return c.Review.Start(ctx, p.ReviewStartParams{ThreadID: "hook-thread", Target: p.ReviewTargetWrapper{Value: &p.UncommittedChangesReviewTarget{}}})
		}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			for _, invalid := range []bool{true, false} {
				raw := validHookPrompt
				if invalid {
					raw = invalidHookPrompt
				}
				body := map[string]any{"turn": hookPromptTurn(raw)}
				switch tc.method {
				case "thread/items/list":
					body = map[string]any{"data": []any{map[string]any{"turnId": "turn", "item": json.RawMessage(raw)}}}
				case "thread/turns/list":
					body = map[string]any{"data": []any{hookPromptTurn(raw)}}
				case "review/start":
					body["reviewThreadId"] = "hook-thread"
				}
				if err := mock.SetResponseData(tc.method, body); err != nil {
					t.Fatal(err)
				}
				result, err := tc.call(client)
				if invalid && (err == nil || !reflect.ValueOf(result).IsZero()) {
					t.Fatalf("invalid history/turn published: %v", err)
				}
				if !invalid && err != nil {
					t.Fatalf("valid recovery: %v", err)
				}
			}
			if len(mock.SentRequests) != 2 {
				t.Fatal("recovery did not send second request")
			}
		})
	}
}

func TestHookPromptNestedNotificationRecovery(t *testing.T) {
	for _, method := range []string{"thread/started", "turn/started", "turn/completed"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			reports, on, add := 0, 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, _ error) {
				if origin != method {
					t.Errorf("diagnostic origin=%s", origin)
				}
				reports++
			}))
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "hook-thread", Name: p.Ptr("cached")})
			var remove func()
			switch method {
			case "thread/started":
				client.OnThreadStarted(func(p.ThreadStartedNotification) { on++ })
				remove = client.AddThreadStartedListener(func(p.ThreadStartedNotification) { add++ })
			case "turn/started":
				client.OnTurnStarted(func(p.TurnStartedNotification) { on++ })
				remove = client.AddTurnStartedListener(func(p.TurnStartedNotification) { add++ })
			case "turn/completed":
				client.OnTurnCompleted(func(p.TurnCompletedNotification) { on++ })
				remove = client.AddTurnCompletedListener(func(p.TurnCompletedNotification) { add++ })
			}
			defer remove()
			for _, invalid := range []bool{true, false} {
				raw := validHookPrompt
				if invalid {
					raw = invalidHookPrompt
				}
				body := map[string]any{"threadId": "hook-thread", "turn": hookPromptTurn(raw)}
				if method == "thread/started" {
					thread := validProcessThreadPayload("hook-thread")
					thread["name"] = "recovered"
					thread["turns"] = []any{hookPromptTurn(raw)}
					body = map[string]any{"thread": thread}
				}
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				before := reports
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: encoded})
				snapshot, ok := client.ThreadStateSnapshot("hook-thread")
				if !ok || snapshot.Name == nil {
					t.Fatal("cache snapshot lost")
				}
				if invalid {
					if on != 0 || add != 0 || reports <= before || *snapshot.Name != "cached" || len(snapshot.Turns) != 0 {
						t.Fatal("invalid nested item published callbacks/cache")
					}
				} else if on != 1 || add != 1 || reports != before {
					t.Fatal("valid notification recovery failed")
				}
				if !invalid && method == "thread/started" {
					item := snapshot.Turns[0].Items[0].Value.(*p.HookPromptThreadItem)
					if item.Fragments[0].Text != "prompt" || *snapshot.Name != "recovered" {
						t.Fatal("valid recovered cache lost typed metadata")
					}
				}
			}
		})
	}
}
