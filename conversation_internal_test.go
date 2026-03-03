package codex

import (
	"encoding/json"
	"testing"
)

func TestThreadCloneAdditionalDetails(t *testing.T) {
	details := "retry after 30s"
	conv := &Conversation{
		thread: Thread{
			Turns: []Turn{{
				ID:     "t1",
				Status: "completed",
				Error: &TurnError{
					Message:           "rate limited",
					AdditionalDetails: &details,
				},
			}},
		},
	}

	snap := conv.Thread()
	if snap.Turns[0].Error.AdditionalDetails == nil {
		t.Fatal("expected AdditionalDetails in snapshot")
	}
	if *snap.Turns[0].Error.AdditionalDetails != "retry after 30s" {
		t.Fatalf("got %q, want %q", *snap.Turns[0].Error.AdditionalDetails, "retry after 30s")
	}

	// Mutate the snapshot.
	*snap.Turns[0].Error.AdditionalDetails = "mutated"

	// Original must be unchanged.
	if *conv.thread.Turns[0].Error.AdditionalDetails != "retry after 30s" {
		t.Errorf("AdditionalDetails = %q, want %q — mutation leaked through shallow copy",
			*conv.thread.Turns[0].Error.AdditionalDetails, "retry after 30s")
	}
}

func TestThreadCloneSourceIsolation(t *testing.T) {
	conv := &Conversation{
		thread: Thread{
			Source: SessionSourceWrapper{
				Value: SessionSourceSubAgent{
					SubAgent: SubAgentSourceThreadSpawn{
						ThreadSpawn: struct {
							AgentNickname  string `json:"agent_nickname"`
							AgentRole      string `json:"agent_role"`
							Depth          uint32 `json:"depth"`
							ParentThreadID string `json:"parent_thread_id"`
						}{
							AgentNickname:  "helper",
							AgentRole:      "assistant",
							Depth:          1,
							ParentThreadID: "parent-1",
						},
					},
				},
			},
		},
	}

	snap := conv.Thread()
	sub, ok := snap.Source.Value.(SessionSourceSubAgent)
	if !ok {
		t.Fatal("expected SessionSourceSubAgent")
	}
	ts, ok := sub.SubAgent.(SubAgentSourceThreadSpawn)
	if !ok {
		t.Fatal("expected SubAgentSourceThreadSpawn")
	}
	if ts.ThreadSpawn.AgentNickname != "helper" {
		t.Errorf("snapshot has wrong value: got %q, want %q", ts.ThreadSpawn.AgentNickname, "helper")
	}

	origSub, ok := conv.thread.Source.Value.(SessionSourceSubAgent)
	if !ok {
		t.Fatal("expected original SessionSourceSubAgent")
	}
	origTS, ok := origSub.SubAgent.(SubAgentSourceThreadSpawn)
	if !ok {
		t.Fatal("expected original SubAgentSourceThreadSpawn")
	}
	if origTS.ThreadSpawn.AgentNickname != "helper" {
		t.Errorf("source mutation leaked: got %q, want %q", origTS.ThreadSpawn.AgentNickname, "helper")
	}
}

func TestThreadCloneStatusIsolation(t *testing.T) {
	conv := &Conversation{
		thread: Thread{
			Status: ThreadStatusWrapper{
				Value: ThreadStatusActive{
					ActiveFlags: []ThreadActiveFlag{"running", "streaming"},
				},
			},
		},
	}

	snap := conv.Thread()
	active, ok := snap.Status.Value.(ThreadStatusActive)
	if !ok {
		t.Fatal("expected ThreadStatusActive")
	}

	// Mutate the snapshot's ActiveFlags slice.
	active.ActiveFlags[0] = "mutated"

	origActive, ok := conv.thread.Status.Value.(ThreadStatusActive)
	if !ok {
		t.Fatal("expected original ThreadStatusActive")
	}
	if origActive.ActiveFlags[0] != "running" {
		t.Errorf("status mutation leaked: got %q, want %q", origActive.ActiveFlags[0], "running")
	}
}

func TestCloneThreadItemWrapperRoundTrip(t *testing.T) {
	variants := []ThreadItemWrapper{
		{Value: &UserMessageThreadItem{ID: "u1"}},
		{Value: &AgentMessageThreadItem{ID: "a1", Text: "hello"}},
		{Value: &PlanThreadItem{ID: "p1", Text: "plan text"}},
		{Value: &ReasoningThreadItem{ID: "r1", Content: []string{"thought"}}},
		{Value: &CommandExecutionThreadItem{ID: "c1", Command: "ls", Cwd: "/tmp", Status: "completed"}},
		{Value: &FileChangeThreadItem{ID: "f1", Status: "applied"}},
		{Value: &McpToolCallThreadItem{ID: "m1", Server: "srv", Tool: "tool", Status: "completed", Arguments: map[string]string{"k": "v"}}},
		{Value: &DynamicToolCallThreadItem{ID: "d1", Tool: "dyn", Status: "completed", Arguments: "arg"}},
		{Value: &CollabAgentToolCallThreadItem{ID: "col1", SenderThreadId: "s1", ReceiverThreadIds: []string{"r1"}}},
		{Value: &WebSearchThreadItem{ID: "w1", Query: "go"}},
		{Value: &ImageViewThreadItem{ID: "i1", Path: "/img.png"}},
		{Value: &EnteredReviewModeThreadItem{ID: "e1", Review: "rev"}},
		{Value: &ExitedReviewModeThreadItem{ID: "x1", Review: "rev"}},
		{Value: &ContextCompactionThreadItem{ID: "cc1"}},
		{Value: nil},
	}

	for _, w := range variants {
		name := "nil"
		if w.Value != nil {
			b, _ := json.Marshal(w)
			if len(b) > 60 {
				name = string(b)[:60]
			} else {
				name = string(b)
			}
		}
		t.Run(name, func(t *testing.T) {
			clone := cloneThreadItemWrapper(w)

			if w.Value == nil {
				if clone.Value != nil {
					t.Fatal("expected nil Value in clone")
				}
				return
			}

			origJSON, err := json.Marshal(w)
			if err != nil {
				t.Fatalf("marshal original: %v", err)
			}
			cloneJSON, err := json.Marshal(clone)
			if err != nil {
				t.Fatalf("marshal clone: %v", err)
			}
			if string(origJSON) != string(cloneJSON) {
				t.Errorf("round-trip mismatch:\n  orig:  %s\n  clone: %s", origJSON, cloneJSON)
			}
		})
	}
}

func TestThreadCloneNestedItemIsolation(t *testing.T) {
	path := "/tmp"
	placeholder := "file-path"
	conv := &Conversation{
		thread: Thread{
			Turns: []Turn{{
				ID:     "t1",
				Status: TurnStatusCompleted,
				Items: []ThreadItemWrapper{
					{
						Value: &UserMessageThreadItem{
							ID: "u1",
							Content: []UserInput{
								&TextUserInput{
									Text: "hello",
									TextElements: []TextElement{{
										ByteRange:   ByteRange{Start: 0, End: 5},
										Placeholder: &placeholder,
									}},
								},
							},
						},
					},
					{
						Value: &CommandExecutionThreadItem{
							ID:      "cmd-1",
							Command: "rg",
							Cwd:     "/tmp",
							Status:  CommandExecutionStatusCompleted,
							CommandActions: []CommandActionWrapper{
								{Value: &SearchCommandAction{Command: "rg", Path: &path}},
							},
						},
					},
				},
			}},
		},
	}

	snap := conv.Thread()
	user := snap.Turns[0].Items[0].Value.(*UserMessageThreadItem)
	text := user.Content[0].(*TextUserInput)
	*text.TextElements[0].Placeholder = "changed"

	cmd := snap.Turns[0].Items[1].Value.(*CommandExecutionThreadItem)
	search := cmd.CommandActions[0].Value.(*SearchCommandAction)
	*search.Path = "/changed"

	origUser := conv.thread.Turns[0].Items[0].Value.(*UserMessageThreadItem)
	origText := origUser.Content[0].(*TextUserInput)
	if *origText.TextElements[0].Placeholder != "file-path" {
		t.Fatalf("placeholder mutation leaked: got %q, want %q", *origText.TextElements[0].Placeholder, "file-path")
	}

	origCmd := conv.thread.Turns[0].Items[1].Value.(*CommandExecutionThreadItem)
	origSearch := origCmd.CommandActions[0].Value.(*SearchCommandAction)
	if *origSearch.Path != "/tmp" {
		t.Fatalf("command action path mutation leaked: got %q, want %q", *origSearch.Path, "/tmp")
	}
}

func TestThreadCloneDoesNotPanicOnUnmarshalableDynamicArguments(t *testing.T) {
	conv := &Conversation{
		thread: Thread{
			Turns: []Turn{{
				ID:     "t1",
				Status: TurnStatusCompleted,
				Items: []ThreadItemWrapper{
					{
						Value: &DynamicToolCallThreadItem{
							ID:        "dyn-1",
							Tool:      "tool",
							Status:    DynamicToolCallStatusCompleted,
							Arguments: func() {},
						},
					},
				},
			}},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Thread() panicked: %v", r)
		}
	}()

	snap := conv.Thread()
	if snap.Turns[0].Items[0].Value == nil {
		t.Fatal("expected cloned item")
	}
}
