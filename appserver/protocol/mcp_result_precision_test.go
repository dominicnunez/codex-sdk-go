package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpResultPrecision(t *testing.T) {
	for _, tc := range []struct{ name, field, raw string }{{"content-extreme", "content", "[1e400,-0,0.10]"}, {"content-huge", "content", "[" + strings.Repeat("9", 400) + "]"}, {"structured-scalar", "structuredContent", "1e-400"}, {"structured-array", "structuredContent", "[null,true,9007199254740993]"}, {"meta-null", "_meta", "null"}, {"content-small", "content", "[42]"}, {"content-large", "content", "[9007199254740993]"}, {"content-null-element", "content", "[null]"}, {"structured-small", "structuredContent", `{"n":42}`}, {"structured-large", "structuredContent", `{"ids":[9007199254740993,18446744073709551615]}`}, {"meta-large", "_meta", `{"n":9007199254740993}`}} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			diagnostics := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { diagnostics++ }))
			defer client.Close()
			result := map[string]json.RawMessage{"content": json.RawMessage(`[]`)}
			result[tc.field] = json.RawMessage(tc.raw)
			encoded, e := json.Marshal(result)
			if e != nil {
				t.Fatal(e)
			}
			checkValue := func(value any, route string) {
				t.Helper()
				raw, e := json.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				if string(raw) != tc.raw {
					t.Errorf("%s changed %s: got=%s want=%s", route, tc.field, raw, tc.raw)
				}
			}
			checkItem := func(item p.ThreadItemWrapper, route string) {
				t.Helper()
				call, ok := item.Value.(*p.McpToolCallThreadItem)
				if !ok || call.Result == nil {
					t.Fatal("missing MCP result")
				}
				switch tc.field {
				case "content":
					checkValue(call.Result.Content, route)
				case "structuredContent":
					checkValue(call.Result.StructuredContent, route)
				default:
					checkValue(call.Result.Meta, route)
				}
			}
			item := json.RawMessage(`{"type":"mcpToolCall","id":"i","server":"s","tool":"fixture","status":"completed","arguments":null,"result":` + string(encoded) + `}`)
			thread := validProcessThreadPayload("t")
			thread["turns"] = []any{map[string]any{"id": "turn", "status": "completed", "items": []any{item}}}
			if e := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); e != nil {
				t.Fatal(e)
			}
			r, e := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
			if e != nil {
				t.Fatal(e)
			}
			checkItem(r.Thread.Turns[0].Items[0], "Thread.Read")
			calls := 0
			client.OnItemStarted(func(n p.ItemStartedNotification) { calls++; checkItem(n.Item, "OnItemStarted") })
			client.AddItemStartedListener(func(n p.ItemStartedNotification) { calls++; checkItem(n.Item, "AddItemStarted") })
			client.OnItemCompleted(func(n p.ItemCompletedNotification) { calls++; checkItem(n.Item, "OnItemCompleted") })
			client.AddItemCompletedListener(func(n p.ItemCompletedNotification) { calls++; checkItem(n.Item, "AddItemCompleted") })
			for _, method := range []string{"item/started", "item/completed"} {
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"completedAtMs":1,"startedAtMs":1,"threadId":"t","turnId":"turn","item":` + string(item) + `}`)})
			}
			if calls != 4 || diagnostics != 0 {
				t.Fatalf("callbacks=%d diagnostics=%d", calls, diagnostics)
			}
			// Identical result through the dedicated service is already number-preserving.
			mock.SetResponse("mcpServer/tool/call", p.Response{Result: encoded})
			direct, e := client.Mcp.ToolCall(context.Background(), p.McpServerToolCallParams{Server: "s", ThreadID: "t", Tool: "fixture"})
			if e != nil {
				t.Fatal(e)
			}
			switch tc.field {
			case "content":
				checkValue(direct.Content, "Mcp.ToolCall control")
			case "structuredContent":
				checkValue(direct.StructuredContent, "Mcp.ToolCall control")
			default:
				checkValue(direct.Meta, "Mcp.ToolCall control")
			}
		})
	}
}
