package protocol_test

import (
	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"reflect"
	"testing"
)

func TestAuditThreadCachePreservesSharedSlices(t *testing.T) {
	data := []int{1, 2, 3}
	args := struct{ Short, Long []int }{Short: data[:1], Long: data}
	c := codex.NewClient(NewMockTransport())
	c.CacheThreadState(codex.Thread{ID: "thread", Turns: []codex.Turn{{ID: "turn", Items: []codex.ThreadItemWrapper{{Value: &codex.McpToolCallThreadItem{Arguments: args}}}}}})
	got, _ := c.ThreadStateSnapshot("thread")
	item := got.Turns[0].Items[0].Value.(*codex.McpToolCallThreadItem)
	if !reflect.DeepEqual(item.Arguments, args) {
		t.Fatalf("cached arguments changed: got %+v, want %+v", item.Arguments, args)
	}
}
