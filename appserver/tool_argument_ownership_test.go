package appserver

import (
	"encoding/json"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestToolArgumentNumberSnapshots(t *testing.T) {
	for _, kind := range []string{"mcpToolCall", "dynamicToolCall"} {
		var source ThreadItemWrapper
		raw := `{"type":"` + kind + `","id":"i","server":"fixture","tool":"fixture","status":"completed","arguments":{"ids":[9007199254740993,1e400]}}`
		if err := json.Unmarshal([]byte(raw), &source); err != nil {
			t.Fatal(err)
		}
		args := func(item ThreadItemWrapper) []any {
			var value any
			switch v := item.Value.(type) {
			case *protocol.McpToolCallThreadItem:
				value = v.Arguments
			case *protocol.DynamicToolCallThreadItem:
				value = v.Arguments
			default:
				t.Fatalf("unexpected item %T", v)
			}
			return value.(map[string]any)["ids"].([]any)
		}
		copied := cloneThreadItemWrapper(source)
		original, cloned := args(source), args(copied)
		if original[0] != json.Number("9007199254740993") || cloned[1] != json.Number("1e400") {
			t.Fatal("snapshot lost exact number tokens")
		}
		cloned[0] = json.Number("2")
		if original[0] != json.Number("9007199254740993") {
			t.Fatal("copy mutation reached source arguments")
		}
		original[1] = json.Number("3")
		if cloned[1] != json.Number("1e400") {
			t.Fatal("source mutation reached copied arguments")
		}
	}
}
