package codex_test

import (
	"context"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestAgentTrackerProcessEvent(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Simulate a spawnAgent event.
	tracker.ProcessEvent(&codex.CollabToolCallStarted{
		ID:                "tc-1",
		Tool:              codex.CollabAgentToolSpawnAgent,
		Status:            codex.CollabAgentToolCallStatusInProgress,
		SenderThreadId:    "parent-thread",
		ReceiverThreadIds: []string{"child-1"},
		AgentsStates: map[string]codex.CollabAgentState{
			"child-1": {Status: codex.CollabAgentStatusPendingInit},
		},
	})

	if tracker.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", tracker.ActiveCount())
	}

	info, ok := tracker.Agent("child-1")
	if !ok {
		t.Fatal("Agent('child-1') not found")
	}
	if info.Status != codex.CollabAgentStatusPendingInit {
		t.Errorf("Status = %q, want %q", info.Status, codex.CollabAgentStatusPendingInit)
	}
	if info.SpawnedBy != "parent-thread" {
		t.Errorf("SpawnedBy = %q, want 'parent-thread'", info.SpawnedBy)
	}

	// Simulate the spawn completing — agent transitions to running.
	tracker.ProcessEvent(&codex.CollabToolCallCompleted{
		ID:                "tc-1",
		Tool:              codex.CollabAgentToolSpawnAgent,
		Status:            codex.CollabAgentToolCallStatusCompleted,
		SenderThreadId:    "parent-thread",
		ReceiverThreadIds: []string{"child-1"},
		AgentsStates: map[string]codex.CollabAgentState{
			"child-1": {Status: codex.CollabAgentStatusRunning},
		},
	})

	info, _ = tracker.Agent("child-1")
	if info.Status != codex.CollabAgentStatusRunning {
		t.Errorf("Status = %q, want %q", info.Status, codex.CollabAgentStatusRunning)
	}
	if tracker.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", tracker.ActiveCount())
	}

	// Simulate closeAgent completing — agent transitions to completed.
	tracker.ProcessEvent(&codex.CollabToolCallCompleted{
		ID:                "tc-2",
		Tool:              codex.CollabAgentToolCloseAgent,
		Status:            codex.CollabAgentToolCallStatusCompleted,
		SenderThreadId:    "parent-thread",
		ReceiverThreadIds: []string{"child-1"},
		AgentsStates: map[string]codex.CollabAgentState{
			"child-1": {Status: codex.CollabAgentStatusCompleted, Message: codex.Ptr("done")},
		},
	})

	info, _ = tracker.Agent("child-1")
	if info.Status != codex.CollabAgentStatusCompleted {
		t.Errorf("Status = %q, want %q", info.Status, codex.CollabAgentStatusCompleted)
	}
	if info.Message != "done" {
		t.Errorf("Message = %q, want 'done'", info.Message)
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d, want 0", tracker.ActiveCount())
	}
}

func TestAgentTrackerAgentsSnapshot(t *testing.T) {
	tracker := codex.NewAgentTracker()

	tracker.ProcessEvent(&codex.CollabToolCallStarted{
		Tool:           codex.CollabAgentToolSpawnAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"a": {Status: codex.CollabAgentStatusRunning},
			"b": {Status: codex.CollabAgentStatusPendingInit},
		},
	})

	agents := tracker.Agents()
	if len(agents) != 2 {
		t.Fatalf("len(Agents()) = %d, want 2", len(agents))
	}
	if agents["a"].Status != codex.CollabAgentStatusRunning {
		t.Errorf("agents['a'].Status = %q, want running", agents["a"].Status)
	}
	if agents["b"].Status != codex.CollabAgentStatusPendingInit {
		t.Errorf("agents['b'].Status = %q, want pendingInit", agents["b"].Status)
	}
}

func TestAgentTrackerWaitAllDone(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Add a running agent.
	tracker.ProcessEvent(&codex.CollabToolCallStarted{
		Tool:           codex.CollabAgentToolSpawnAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"child-1": {Status: codex.CollabAgentStatusRunning},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Complete the agent in a goroutine.
	go func() {
		time.Sleep(50 * time.Millisecond)
		tracker.ProcessEvent(&codex.CollabToolCallCompleted{
			Tool:           codex.CollabAgentToolCloseAgent,
			SenderThreadId: "parent",
			AgentsStates: map[string]codex.CollabAgentState{
				"child-1": {Status: codex.CollabAgentStatusCompleted},
			},
		})
	}()

	err := tracker.WaitAllDone(ctx)
	if err != nil {
		t.Fatalf("WaitAllDone: %v", err)
	}
}

func TestAgentTrackerWaitAllDoneContextCancel(t *testing.T) {
	tracker := codex.NewAgentTracker()

	tracker.ProcessEvent(&codex.CollabToolCallStarted{
		Tool:           codex.CollabAgentToolSpawnAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"child-1": {Status: codex.CollabAgentStatusRunning},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := tracker.WaitAllDone(ctx)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestAgentTrackerIgnoresNonCollabEvents(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Non-collab events should be silently ignored.
	tracker.ProcessEvent(&codex.TextDelta{Delta: "hello"})
	tracker.ProcessEvent(&codex.TurnCompleted{})
	tracker.ProcessEvent(&codex.ItemStarted{})

	if len(tracker.Agents()) != 0 {
		t.Errorf("Agents() should be empty, got %d entries", len(tracker.Agents()))
	}
}

func TestAgentTrackerMultipleAgents(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Spawn two agents.
	tracker.ProcessEvent(&codex.CollabToolCallStarted{
		Tool:           codex.CollabAgentToolSpawnAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"a": {Status: codex.CollabAgentStatusRunning},
			"b": {Status: codex.CollabAgentStatusRunning},
		},
	})

	if tracker.ActiveCount() != 2 {
		t.Errorf("ActiveCount() = %d, want 2", tracker.ActiveCount())
	}

	// Complete one.
	tracker.ProcessEvent(&codex.CollabToolCallCompleted{
		Tool:           codex.CollabAgentToolCloseAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"a": {Status: codex.CollabAgentStatusCompleted},
			"b": {Status: codex.CollabAgentStatusRunning},
		},
	})

	if tracker.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", tracker.ActiveCount())
	}

	// Complete the other.
	tracker.ProcessEvent(&codex.CollabToolCallCompleted{
		Tool:           codex.CollabAgentToolCloseAgent,
		SenderThreadId: "parent",
		AgentsStates: map[string]codex.CollabAgentState{
			"a": {Status: codex.CollabAgentStatusCompleted},
			"b": {Status: codex.CollabAgentStatusErrored, Message: codex.Ptr("failed")},
		},
	})

	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d, want 0", tracker.ActiveCount())
	}

	info, _ := tracker.Agent("b")
	if info.Message != "failed" {
		t.Errorf("Message = %q, want 'failed'", info.Message)
	}
}
