package appserver_test

import (
	"encoding/json"
	"reflect"
	"testing"

	a "github.com/dominicnunez/codex-sdk-go/appserver"
)

type definedCollectorPointer *string

func TestDefinedPointerCollector(t *testing.T) {
	for _, phase := range []string{"started", "completed"} {
		for _, form := range []string{"value", "pointer"} {
			for _, kind := range []string{"ordinary", "defined", "nil-defined"} {
				t.Run(phase+"/"+form+"/"+kind, func(t *testing.T) {
					source := "original"
					var argument any = &source
					if kind == "defined" {
						argument = definedCollectorPointer(&source)
					}
					if kind == "nil-defined" {
						argument = definedCollectorPointer(nil)
					}
					expected, err := json.Marshal(argument)
					if err != nil {
						t.Fatal(err)
					}
					item := a.McpToolCallThreadItem{ID: "i", Server: "server", Tool: "tool", Status: a.McpToolCallStatusCompleted, Arguments: argument}
					if phase == "started" {
						item.Status = a.McpToolCallStatusInProgress
					}
					var value a.ThreadItem = item
					if form == "pointer" {
						value = &item
					}
					collector := a.NewStreamCollector()
					wrapper := a.ThreadItemWrapper{Value: value}
					if phase == "started" {
						collector.Process(&a.ItemStarted{ThreadID: "t", TurnID: "u", Item: wrapper}, nil)
					} else {
						collector.Process(&a.ItemCompleted{ThreadID: "t", TurnID: "u", Item: wrapper}, nil)
					}
					snapshotArgument := func() any {
						lifecycle := collector.Summary().McpToolCalls["i"]
						if phase == "started" {
							return lifecycle.StartedItem.Arguments
						}
						return lifecycle.CompletedItem.Arguments
					}
					copied := snapshotArgument()
					actual, err := json.Marshal(copied)
					if err != nil || string(actual) != string(expected) {
						t.Fatal("JSON value changed")
					}
					if reflect.TypeOf(copied) != reflect.TypeOf(argument) {
						t.Fatalf("collector changed concrete metadata type: %T / %T", copied, argument)
					}
					if kind == "nil-defined" {
						if !reflect.ValueOf(copied).IsNil() {
							t.Fatal("typed nil became nonnil")
						}
						return
					}
					reflect.ValueOf(copied).Elem().SetString("changed copy")
					if source != "original" {
						t.Fatal("summary aliases source")
					}
					if reflect.ValueOf(snapshotArgument()).Elem().String() != "original" {
						t.Fatal("summaries share storage")
					}
					source = "changed source"
					if reflect.ValueOf(snapshotArgument()).Elem().String() != "original" {
						t.Fatal("collector did not own ingested metadata")
					}
				})
			}
		}
	}
}
