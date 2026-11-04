package protocol_test

import (
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type definedMetadataPointer *string

func TestDefinedPointerSnapshot(t *testing.T) {
	for _, name := range []string{"ordinary", "defined", "nil-defined"} {
		t.Run(name, func(t *testing.T) {
			source := "original"
			var argument any = &source
			if name == "defined" {
				argument = definedMetadataPointer(&source)
			}
			if name == "nil-defined" {
				argument = definedMetadataPointer(nil)
			}
			before, e := json.Marshal(argument)
			if e != nil {
				t.Fatal(e)
			}
			m := NewMockTransport()
			c := p.NewClient(m)
			defer c.Close()
			base, err := json.Marshal(validProcessThreadPayload("t"))
			if err != nil {
				t.Fatal(err)
			}
			var state p.Thread
			if err = json.Unmarshal(base, &state); err != nil {
				t.Fatal(err)
			}
			state.Turns = []p.Turn{{ID: "u", Status: p.TurnStatusCompleted, ItemsView: p.TurnItemsViewFull, Items: []p.ThreadItemWrapper{{Value: &p.McpToolCallThreadItem{ID: "i", Server: "server", Tool: "tool", Status: p.McpToolCallStatusCompleted, Arguments: argument}}}}}
			if _, err = json.Marshal(state); err != nil {
				t.Fatal(err)
			}
			c.CacheThreadState(state)
			snapshot, ok := c.ThreadStateSnapshot("t")
			if !ok {
				t.Fatal("snapshot missing")
			}
			copied := snapshot.Turns[0].Items[0].Value.(*p.McpToolCallThreadItem).Arguments
			after, e := json.Marshal(copied)
			if e != nil || string(after) != string(before) {
				t.Fatal("JSON value changed")
			}
			if reflect.TypeOf(copied) != reflect.TypeOf(argument) {
				t.Errorf("public snapshot changed concrete metadata type: got %T want %T", copied, argument)
			}
			if name == "nil-defined" {
				if !reflect.ValueOf(copied).IsNil() {
					t.Fatal("typed nil became nonnil")
				}
				return
			}
			reflect.ValueOf(copied).Elem().SetString("changed")
			if source != "original" {
				t.Error("public snapshot aliases input")
			}
			again, _ := c.ThreadStateSnapshot("t")
			fresh := again.Turns[0].Items[0].Value.(*p.McpToolCallThreadItem).Arguments
			if reflect.ValueOf(fresh).Elem().String() != "original" {
				t.Error("public snapshots share pointer storage")
			}
			source = "changed input"
			last, _ := c.ThreadStateSnapshot("t")
			if reflect.ValueOf(last.Turns[0].Items[0].Value.(*p.McpToolCallThreadItem).Arguments).Elem().String() != "original" {
				t.Fatal("cached input pointer was not owned")
			}
		})
	}
}
