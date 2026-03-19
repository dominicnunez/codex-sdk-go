package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

func newConversationForTest(thread Thread) *Conversation {
	return &Conversation{state: newConversationState(thread)}
}

func storedConversationThread(t *testing.T, conv *Conversation) Thread {
	t.Helper()
	if conv == nil || conv.state == nil {
		t.Fatal("conversation state is nil")
	}
	return conv.state.snapshot()
}

type uncloneableThreadItem struct {
	Fn func()
}

func (*uncloneableThreadItem) threadItem() {}

type uncloneableSessionSource struct {
	Fn func()
}

func (uncloneableSessionSource) isSessionSource() {}

type uncloneableThreadStatus struct {
	Fn func()
}

func (uncloneableThreadStatus) isThreadStatus() {}

type uncloneableSubAgentSource struct {
	Fn func()
}

func (uncloneableSubAgentSource) isSubAgentSource() {}

type uncloneableUserInput struct {
	Fn func()
}

func (*uncloneableUserInput) userInput() {}

type uncloneableCommandAction struct {
	Fn func()
}

func (*uncloneableCommandAction) commandAction() {}

type uncloneablePatchChangeKind struct {
	Fn func()
}

func (uncloneablePatchChangeKind) patchChangeKind() {}

type uncloneableDynamicToolCallOutputContentItem struct {
	Fn func()
}

func (*uncloneableDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

type uncloneableWebSearchAction struct {
	Fn func()
}

func (uncloneableWebSearchAction) webSearchAction() {}

func TestThreadCloneAdditionalDetails(t *testing.T) {
	details := "retry after 30s"
	conv := newConversationForTest(Thread{
		Turns: []Turn{{
			ID:     "t1",
			Status: "completed",
			Error: &TurnError{
				Message:           "rate limited",
				AdditionalDetails: &details,
			},
		}},
	})

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
	original := storedConversationThread(t, conv)
	if *original.Turns[0].Error.AdditionalDetails != "retry after 30s" {
		t.Errorf("AdditionalDetails = %q, want %q — mutation leaked through shallow copy",
			*original.Turns[0].Error.AdditionalDetails, "retry after 30s")
	}
}

func TestConversationThreadRetainsLatestSnapshotAfterCacheEvictionPressure(t *testing.T) {
	client := &Client{
		threadStates:         make(map[string]threadStateEntry),
		threadStateListeners: make(map[string][]threadStateListener),
	}
	process := &Process{Client: client}
	conv := &Conversation{
		process:  process,
		threadID: "thread-1",
		state: newConversationState(Thread{
			ID:     "thread-1",
			Status: ThreadStatusWrapper{Value: ThreadStatusIdle{}},
		}),
	}
	unsubscribe := client.addThreadStateListener(conv.threadID, conv.state.storeSnapshot, conv.state.close)
	defer unsubscribe()

	client.cacheThreadState(storedConversationThread(t, conv))

	name := "renamed"
	client.mutateThreadState(conv.threadID, func(thread *Thread) {
		thread.Name = &name
		thread.Status = ThreadStatusWrapper{
			Value: ThreadStatusActive{ActiveFlags: []ThreadActiveFlag{ThreadActiveFlagWaitingOnApproval}},
		}
	})

	for i := range maxCachedThreadStates + 10 {
		client.cacheThreadState(Thread{ID: fmt.Sprintf("other-%02d", i)})
	}

	snapshot := conv.Thread()
	if snapshot.Name == nil || *snapshot.Name != name {
		t.Fatalf("Thread().Name = %v, want %q", snapshot.Name, name)
	}
	active, ok := snapshot.Status.Value.(ThreadStatusActive)
	if !ok {
		t.Fatalf("Thread().Status = %T, want ThreadStatusActive", snapshot.Status.Value)
	}
	if len(active.ActiveFlags) != 1 || active.ActiveFlags[0] != ThreadActiveFlagWaitingOnApproval {
		t.Fatalf("Thread().Status.ActiveFlags = %v, want waitingOnApproval", active.ActiveFlags)
	}
}

func TestConversationCloseReleasesResourcesAndRejectsFutureTurns(t *testing.T) {
	var released atomic.Int32
	conv := &Conversation{
		state: newConversationState(Thread{ID: "thread-1"}),
		release: func() {
			released.Add(1)
		},
	}

	if err := conv.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if err := conv.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
	if got := released.Load(); got != 1 {
		t.Fatalf("release count = %d, want 1", got)
	}
	if err := conv.state.ensureOpen(); err != errConversationClosed {
		t.Fatalf("ensureOpen() error = %v, want %v", err, errConversationClosed)
	}
	if _, _, err := conv.state.startTurn(); err != errConversationClosed {
		t.Fatalf("startTurn() error = %v, want %v", err, errConversationClosed)
	}
}

func TestThreadCloneSourceIsolation(t *testing.T) {
	conv := newConversationForTest(Thread{
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
	})

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

	origSub, ok := storedConversationThread(t, conv).Source.Value.(SessionSourceSubAgent)
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
	conv := newConversationForTest(Thread{
		Status: ThreadStatusWrapper{
			Value: ThreadStatusActive{
				ActiveFlags: []ThreadActiveFlag{"running", "streaming"},
			},
		},
	})

	snap := conv.Thread()
	active, ok := snap.Status.Value.(ThreadStatusActive)
	if !ok {
		t.Fatal("expected ThreadStatusActive")
	}

	// Mutate the snapshot's ActiveFlags slice.
	active.ActiveFlags[0] = "mutated"

	origActive, ok := storedConversationThread(t, conv).Status.Value.(ThreadStatusActive)
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
	conv := newConversationForTest(Thread{
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
	})

	snap := conv.Thread()
	user := snap.Turns[0].Items[0].Value.(*UserMessageThreadItem)
	text := user.Content[0].(*TextUserInput)
	*text.TextElements[0].Placeholder = "changed"

	cmd := snap.Turns[0].Items[1].Value.(*CommandExecutionThreadItem)
	search := cmd.CommandActions[0].Value.(*SearchCommandAction)
	*search.Path = "/changed"

	original := storedConversationThread(t, conv)
	origUser := original.Turns[0].Items[0].Value.(*UserMessageThreadItem)
	origText := origUser.Content[0].(*TextUserInput)
	if *origText.TextElements[0].Placeholder != "file-path" {
		t.Fatalf("placeholder mutation leaked: got %q, want %q", *origText.TextElements[0].Placeholder, "file-path")
	}

	origCmd := original.Turns[0].Items[1].Value.(*CommandExecutionThreadItem)
	origSearch := origCmd.CommandActions[0].Value.(*SearchCommandAction)
	if *origSearch.Path != "/tmp" {
		t.Fatalf("command action path mutation leaked: got %q, want %q", *origSearch.Path, "/tmp")
	}
}

func TestThreadCloneCollabToolCallPointerIsolation(t *testing.T) {
	model := "gpt-5"
	effort := ReasoningEffortHigh
	prompt := "delegate this"
	conv := newConversationForTest(Thread{
		Turns: []Turn{{
			ID:     "t1",
			Status: TurnStatusCompleted,
			Items: []ThreadItemWrapper{{
				Value: &CollabAgentToolCallThreadItem{
					ID:                "collab-1",
					Tool:              CollabAgentToolSpawnAgent,
					Status:            CollabAgentToolCallStatusCompleted,
					AgentsStates:      map[string]CollabAgentState{"worker": {Status: CollabAgentStatusCompleted}},
					Model:             &model,
					ReceiverThreadIds: []string{"thread-2"},
					ReasoningEffort:   &effort,
					SenderThreadId:    "thread-1",
					Prompt:            &prompt,
				},
			}},
		}},
	})

	snap := conv.Thread()
	item, ok := snap.Turns[0].Items[0].Value.(*CollabAgentToolCallThreadItem)
	if !ok {
		t.Fatal("expected CollabAgentToolCallThreadItem")
	}

	*item.Model = "gpt-5-mini"
	*item.ReasoningEffort = ReasoningEffortLow
	*item.Prompt = "mutated"

	orig, ok := storedConversationThread(t, conv).Turns[0].Items[0].Value.(*CollabAgentToolCallThreadItem)
	if !ok {
		t.Fatal("expected original CollabAgentToolCallThreadItem")
	}
	if got := *orig.Model; got != "gpt-5" {
		t.Fatalf("model mutation leaked: got %q, want %q", got, "gpt-5")
	}
	if got := *orig.ReasoningEffort; got != ReasoningEffortHigh {
		t.Fatalf("reasoning effort mutation leaked: got %q, want %q", got, ReasoningEffortHigh)
	}
	if got := *orig.Prompt; got != "delegate this" {
		t.Fatalf("prompt mutation leaked: got %q, want %q", got, "delegate this")
	}
}

func TestThreadCloneDoesNotPanicOnUnmarshalableDynamicArguments(t *testing.T) {
	argFn := func() {}
	conv := newConversationForTest(Thread{
		Turns: []Turn{{
			ID:     "t1",
			Status: TurnStatusCompleted,
			Items: []ThreadItemWrapper{
				{
					Value: &DynamicToolCallThreadItem{
						ID:        "dyn-1",
						Tool:      "tool",
						Status:    DynamicToolCallStatusCompleted,
						Arguments: argFn,
					},
				},
			},
		}},
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Thread() panicked: %v", r)
		}
	}()

	snap := conv.Thread()
	if snap.Turns[0].Items[0].Value == nil {
		t.Fatal("expected cloned item")
	}
	item := snap.Turns[0].Items[0].Value.(*DynamicToolCallThreadItem)
	if item.Arguments == nil {
		t.Fatal("Arguments = nil, want preserved function value")
	}
	if _, ok := item.Arguments.(func()); !ok {
		t.Fatalf("Arguments type = %T, want func()", item.Arguments)
	}
}

func TestCloneFallbacksPreserveUncloneableValues(t *testing.T) {
	originalThreadItem := &uncloneableThreadItem{Fn: func() {}}
	if got := cloneThreadItemWrapperFallback(ThreadItemWrapper{Value: originalThreadItem}); got.Value == nil {
		t.Fatal("thread item fallback lost Value")
	} else if clone, ok := got.Value.(*uncloneableThreadItem); !ok {
		t.Fatalf("thread item fallback type = %T, want *uncloneableThreadItem", got.Value)
	} else if clone == originalThreadItem {
		t.Fatal("thread item fallback reused original pointer")
	}
	if got := cloneSessionSourceWrapperFallback(SessionSourceWrapper{Value: uncloneableSessionSource{Fn: func() {}}}); got.Value == nil {
		t.Fatal("session source fallback lost Value")
	} else if _, ok := got.Value.(uncloneableSessionSource); !ok {
		t.Fatalf("session source fallback type = %T, want uncloneableSessionSource", got.Value)
	}
	if got := cloneThreadStatusWrapperFallback(ThreadStatusWrapper{Value: uncloneableThreadStatus{Fn: func() {}}}); got.Value == nil {
		t.Fatal("thread status fallback lost Value")
	} else if _, ok := got.Value.(uncloneableThreadStatus); !ok {
		t.Fatalf("thread status fallback type = %T, want uncloneableThreadStatus", got.Value)
	}
	originalSubAgent := uncloneableSubAgentSource{Fn: func() {}}
	if got := cloneSubAgentSourceFallback(originalSubAgent); got == nil {
		t.Fatal("sub-agent source fallback lost value")
	} else if _, ok := got.(uncloneableSubAgentSource); !ok {
		t.Fatalf("sub-agent source fallback type = %T, want uncloneableSubAgentSource", got)
	}
	originalUserInput := &uncloneableUserInput{Fn: func() {}}
	if got := cloneUserInputFallback(originalUserInput); got == nil {
		t.Fatal("user input fallback lost value")
	} else if clone, ok := got.(*uncloneableUserInput); !ok {
		t.Fatalf("user input fallback type = %T, want *uncloneableUserInput", got)
	} else if clone == originalUserInput {
		t.Fatal("user input fallback reused original pointer")
	}
	originalCommandAction := &uncloneableCommandAction{Fn: func() {}}
	if got := cloneCommandActionWrapperFallback(CommandActionWrapper{Value: originalCommandAction}); got.Value == nil {
		t.Fatal("command action fallback lost Value")
	} else if clone, ok := got.Value.(*uncloneableCommandAction); !ok {
		t.Fatalf("command action fallback type = %T, want *uncloneableCommandAction", got.Value)
	} else if clone == originalCommandAction {
		t.Fatal("command action fallback reused original pointer")
	}
	if got := clonePatchChangeKindWrapperFallback(PatchChangeKindWrapper{Value: uncloneablePatchChangeKind{Fn: func() {}}}); got.Value == nil {
		t.Fatal("patch change fallback lost Value")
	} else if _, ok := got.Value.(uncloneablePatchChangeKind); !ok {
		t.Fatalf("patch change fallback type = %T, want uncloneablePatchChangeKind", got.Value)
	}
	originalOutput := &uncloneableDynamicToolCallOutputContentItem{Fn: func() {}}
	if got := cloneDynamicToolCallOutputContentItemWrapperFallback(DynamicToolCallOutputContentItemWrapper{Value: originalOutput}); got.Value == nil {
		t.Fatal("dynamic output fallback lost Value")
	} else if clone, ok := got.Value.(*uncloneableDynamicToolCallOutputContentItem); !ok {
		t.Fatalf("dynamic output fallback type = %T, want *uncloneableDynamicToolCallOutputContentItem", got.Value)
	} else if clone == originalOutput {
		t.Fatal("dynamic output fallback reused original pointer")
	}
	if got := cloneWebSearchActionWrapperFallback(WebSearchActionWrapper{Value: uncloneableWebSearchAction{Fn: func() {}}}); got.Value == nil {
		t.Fatal("web search action fallback lost Value")
	} else if _, ok := got.Value.(uncloneableWebSearchAction); !ok {
		t.Fatalf("web search action fallback type = %T, want uncloneableWebSearchAction", got.Value)
	}
	sourceMap := map[string]interface{}{"bad": func() {}, "nested": map[string]string{"k": "v"}}
	gotJSON := cloneJSONValue(sourceMap)
	clonedMap, ok := gotJSON.(map[string]interface{})
	if !ok {
		t.Fatalf("cloneJSONValue type = %T, want map[string]interface{}", gotJSON)
	}
	if clonedMap["bad"] == nil {
		t.Fatal("cloneJSONValue lost function value")
	}
	nested, ok := clonedMap["nested"].(map[string]string)
	if !ok {
		t.Fatalf("cloneJSONValue nested type = %T, want map[string]string", clonedMap["nested"])
	}
	nested["k"] = "changed"
	if sourceMap["nested"].(map[string]string)["k"] != "v" {
		t.Fatal("cloneJSONValue reused nested map")
	}

	conv := newConversationForTest(Thread{
		Source: SessionSourceWrapper{Value: uncloneableSessionSource{Fn: func() {}}},
		Status: ThreadStatusWrapper{Value: uncloneableThreadStatus{Fn: func() {}}},
		Turns: []Turn{{
			ID:     "turn-1",
			Status: TurnStatusCompleted,
			Items: []ThreadItemWrapper{
				{Value: &uncloneableThreadItem{Fn: func() {}}},
			},
		}},
	})

	snap := conv.Thread()
	if _, ok := snap.Source.Value.(uncloneableSessionSource); !ok {
		t.Fatalf("snapshot source type = %T, want uncloneableSessionSource", snap.Source.Value)
	}
	if _, ok := snap.Status.Value.(uncloneableThreadStatus); !ok {
		t.Fatalf("snapshot status type = %T, want uncloneableThreadStatus", snap.Status.Value)
	}
	if _, ok := snap.Turns[0].Items[0].Value.(*uncloneableThreadItem); !ok {
		t.Fatalf("snapshot item type = %T, want *uncloneableThreadItem", snap.Turns[0].Items[0].Value)
	}
}

func TestConversationTurnsRequireInitializedConversation(t *testing.T) {
	tests := []struct {
		name string
		conv *Conversation
	}{
		{name: "nil conversation", conv: nil},
		{name: "zero value", conv: &Conversation{}},
		{name: "missing state", conv: &Conversation{process: &Process{Client: &Client{}}}},
		{name: "missing process", conv: &Conversation{state: newConversationState(Thread{ID: "thread-1"})}},
		{name: "missing client", conv: &Conversation{process: &Process{}, state: newConversationState(Thread{ID: "thread-1"})}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.conv.Turn(context.Background(), TurnOptions{Prompt: "hello"}); err != errConversationUninitialized {
				t.Fatalf("Turn() error = %v, want %v", err, errConversationUninitialized)
			}

			stream := tt.conv.TurnStreamed(context.Background(), TurnOptions{Prompt: "hello"})
			var gotErr error
			for _, err := range stream.Events() {
				gotErr = err
				break
			}
			if !errors.Is(gotErr, errConversationUninitialized) {
				t.Fatalf("TurnStreamed() error = %v, want %v", gotErr, errConversationUninitialized)
			}
		})
	}
}
