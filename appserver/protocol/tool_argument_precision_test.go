package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestDynamicToolArgumentPrecision(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{{"negative-zero", "-0"}, {"decimal", "0.10"}, {"huge-exponent", "1e400"}, {"tiny-exponent", "1e-400"}, {"boolean", "true"}, {"empty-array", "[]"}, {"empty-object", "{}"}, {"huge-integer", strings.Repeat("9", 400)}, {"small", "42"}, {"exact-boundary", "9007199254740992"}, {"above-boundary", "9007199254740993"}, {"uint64-max", "18446744073709551615"}, {"negative", "-9007199254740993"}, {"nested", `{"ids":[9007199254740993,18446744073709551615]}`}, {"null", "null"}, {"string", `"9007199254740993"`}} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			calls := 0
			var observed []byte
			client.SetApprovalHandlers(p.ApprovalHandlers{OnDynamicToolCall: func(_ context.Context, params p.DynamicToolCallParams) (p.DynamicToolCallResponse, error) {
				calls++
				var e error
				observed, e = json.Marshal(params.Arguments)
				if e != nil {
					t.Fatal(e)
				}
				return p.DynamicToolCallResponse{Success: true, ContentItems: []p.DynamicToolCallOutputContentItemWrapper{}}, nil
			}})
			response, e := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "call"}, Method: "item/tool/call", Params: json.RawMessage(`{"tool":"fixture","arguments":` + tc.raw + `,"callId":"call","threadId":"t","turnId":"turn"}`)})
			if e != nil || response.Error != nil || len(response.Result) == 0 || calls != 1 {
				t.Fatalf("valid tool invocation failed: %v", e)
			}
			if string(observed) != tc.raw {
				t.Errorf("handler received changed arguments: got=%s want=%s", observed, tc.raw)
			}
		})
	}
}

func TestToolHistoryArgumentPrecision(t *testing.T) {
	for _, kind := range []string{"dynamicToolCall", "mcpToolCall"} {
		for _, raw := range []string{"42", "null", "true", `"opaque"`, "[]", "{}", "-0", "0.10", "1e400", "1e-400", strings.Repeat("9", 400), "9007199254740993", `{"ids":[9007199254740993,18446744073709551615]}`} {
			t.Run(kind+"/"+raw, func(t *testing.T) {
				mock := NewMockTransport()
				diagnostics := 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { diagnostics++ }))
				defer client.Close()
				item := `{"type":"` + kind + `","id":"i","tool":"fixture","server":"fixture","status":"completed","arguments":` + raw + `}`
				check := func(value p.ThreadItemWrapper, route string) {
					t.Helper()
					var args any
					switch v := value.Value.(type) {
					case *p.DynamicToolCallThreadItem:
						args = v.Arguments
					case *p.McpToolCallThreadItem:
						args = v.Arguments
					default:
						t.Fatalf("unexpected item %T", value.Value)
					}
					encoded, e := json.Marshal(args)
					if e != nil {
						t.Fatal(e)
					}
					if string(encoded) != raw {
						t.Errorf("%s changed tool arguments: got=%s want=%s", route, encoded, raw)
					}
				}
				thread := validProcessThreadPayload("t")
				thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": []any{json.RawMessage(item)}}}
				if e := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); e != nil {
					t.Fatal(e)
				}
				r, e := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
				if e != nil {
					t.Fatal(e)
				}
				check(r.Thread.Turns[0].Items[0], "Thread.Read")
				calls := 0
				client.OnItemStarted(func(n p.ItemStartedNotification) { calls++; check(n.Item, "OnItemStarted") })
				client.AddItemStartedListener(func(n p.ItemStartedNotification) { calls++; check(n.Item, "AddItemStarted") })
				client.OnItemCompleted(func(n p.ItemCompletedNotification) { calls++; check(n.Item, "OnItemCompleted") })
				client.AddItemCompletedListener(func(n p.ItemCompletedNotification) { calls++; check(n.Item, "AddItemCompleted") })
				for _, method := range []string{"item/started", "item/completed"} {
					mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"completedAtMs":1,"startedAtMs":1,"threadId":"t","turnId":"turn","item":` + item + `}`)})
				}
				if calls != 4 || diagnostics != 0 {
					t.Fatalf("valid carriers calls=%d diagnostics=%d", calls, diagnostics)
				}
			})
		}
	}
}
