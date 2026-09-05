package appserver

import (
	"reflect"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestConversationMetadataSnapshotIsolation(t *testing.T) {
	makeThread := func() Thread {
		return Thread{
			ForkedFromID: Ptr("fork"), ParentThreadID: Ptr("parent"), ProjectID: Ptr("project"),
			RecencyAt: Ptr(int64(1)), SectionEnteredAt: Ptr(int64(2)), Model: Ptr("model"),
			ReasoningEffort: Ptr(ReasoningEffortHigh),
			Section:         &protocol.ThreadSection{ID: "section", Name: "name", Appearance: &protocol.ThreadSectionAppearance{Color: Ptr("blue"), Icon: Ptr("star")}},
			Turns: []Turn{{StartedAt: Ptr(int64(3)), CompletedAt: Ptr(int64(4)), DurationMs: Ptr(int64(5)),
				Error: &TurnError{Misalignment: &protocol.MisalignmentErrorDetails{DetailedExplanation: Ptr("explanation")}},
				Items: []ThreadItemWrapper{{Value: &AgentMessageThreadItem{Questions: Ptr([]protocol.AsyncUserInputQuestion{{Title: "question", Options: Ptr([]string{"option"})}})}}},
			}},
		}
	}
	mutate := func(thread Thread) {
		*thread.ForkedFromID = "changed"
		*thread.ParentThreadID = "changed"
		*thread.ProjectID = "changed"
		*thread.RecencyAt = 99
		*thread.SectionEnteredAt = 99
		*thread.Model = "changed"
		*thread.ReasoningEffort = ReasoningEffort("low")
		thread.Section.ID = "changed"
		thread.Section.Name = "changed"
		*thread.Section.Appearance.Color = "changed"
		*thread.Section.Appearance.Icon = "changed"
		*thread.Turns[0].StartedAt = 99
		*thread.Turns[0].CompletedAt = 99
		*thread.Turns[0].DurationMs = 99
		*thread.Turns[0].Error.Misalignment.DetailedExplanation = "changed"
		questions := thread.Turns[0].Items[0].Value.(*AgentMessageThreadItem).Questions
		(*questions)[0].Title = "changed"
		(*(*questions)[0].Options)[0] = "changed"
	}
	for _, storage := range []string{"initial", "store", "completed"} {
		t.Run(storage, func(t *testing.T) {
			input := makeThread()
			state := newConversationState(input)
			if storage == "store" {
				state.storeSnapshot(input)
			}
			if storage == "completed" {
				state.applyCompletedThread(input)
			}
			mutate(input)
			conv := &Conversation{state: state}
			if got := conv.Thread(); !reflect.DeepEqual(got, makeThread()) {
				t.Fatal("stored metadata aliases input")
			}
			mutate(conv.Thread())
			if got := conv.Thread(); !reflect.DeepEqual(got, makeThread()) {
				t.Fatal("returned metadata aliases conversation")
			}
		})
	}
}
