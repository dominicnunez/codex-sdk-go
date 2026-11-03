package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestToolArgumentNativeErrors(t *testing.T) {
	for _, kind := range []string{"mcpToolCall", "dynamicToolCall"} {
		for _, field := range []string{"id", "tool", "status", "startedAtMs"} {
			fields := `"arguments":42,"id":"i","tool":"fixture","server":"fixture","status":"completed"`
			if field == "startedAtMs" {
				fields += `,"startedAtMs":{}`
			} else {
				fields = strings.Replace(fields, `"`+field+`":"`+map[string]string{"id": "i", "tool": "fixture", "status": "completed"}[field]+`"`, `"`+field+`":17`, 1)
			}
			raw := []byte(`{"type":"` + kind + `",` + fields + `}`)
			var actual, reference error
			if kind == "mcpToolCall" {
				_, actual = decodeMcpToolCallThreadItem(raw)
				_, reference = decodeThreadItemIntoWithValidation(raw, &McpToolCallThreadItem{}, []string{"arguments", "id", "server", "status", "tool"}, []string{"id", "server", "status", "tool"})
			} else {
				_, actual = decodeDynamicToolCallThreadItem(raw)
				_, reference = decodeThreadItemIntoWithValidation(raw, &DynamicToolCallThreadItem{}, []string{"arguments", "id", "status", "tool"}, []string{"id", "status", "tool"})
			}
			var a, b *json.UnmarshalTypeError
			if !errors.As(actual, &a) || !errors.As(reference, &b) || !reflect.DeepEqual(a, b) {
				t.Fatalf("%s %s native context changed: %#v / %#v: %v / %v", kind, field, a, b, actual, reference)
			}
		}
	}
}
