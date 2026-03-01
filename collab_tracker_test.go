package codex_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestAgentTrackerProcessEvent(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Simulate a spawnAgent event.
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:             codex.CollabToolCallStartedPhase,
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
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:             codex.CollabToolCallCompletedPhase,
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
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:             codex.CollabToolCallCompletedPhase,
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

	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallStartedPhase,
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
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallStartedPhase,
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
		tracker.ProcessEvent(&codex.CollabToolCallEvent{
			Phase:          codex.CollabToolCallCompletedPhase,
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

	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallStartedPhase,
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

func TestAgentTrackerWaitAllDoneEmpty(t *testing.T) {
	tracker := codex.NewAgentTracker()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Empty tracker should return immediately (vacuously true).
	err := tracker.WaitAllDone(ctx)
	if err != nil {
		t.Fatalf("WaitAllDone on empty tracker: %v", err)
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

func TestAgentTrackerAgentNotFound(t *testing.T) {
	tracker := codex.NewAgentTracker()

	info, ok := tracker.Agent("nonexistent")
	if ok {
		t.Error("Agent() returned true for nonexistent thread ID")
	}
	if info.ThreadID != "" {
		t.Errorf("info.ThreadID = %q, want empty", info.ThreadID)
	}
}

func TestAgentTrackerMultipleAgents(t *testing.T) {
	tracker := codex.NewAgentTracker()

	// Spawn two agents.
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallStartedPhase,
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
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallCompletedPhase,
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
	tracker.ProcessEvent(&codex.CollabToolCallEvent{
		Phase:          codex.CollabToolCallCompletedPhase,
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

func TestAgentTrackerConcurrentAccess(t *testing.T) {
	const numAgents = 20
	const numReaders = 10

	tracker := codex.NewAgentTracker()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Goroutines that spawn agents via ProcessEvent.
	for i := range numAgents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			threadID := fmt.Sprintf("agent-%d", i)
			tracker.ProcessEvent(&codex.CollabToolCallEvent{
				Phase:          codex.CollabToolCallStartedPhase,
				Tool:           codex.CollabAgentToolSpawnAgent,
				SenderThreadId: "parent",
				AgentsStates: map[string]codex.CollabAgentState{
					threadID: {Status: codex.CollabAgentStatusRunning},
				},
			})
		}()
	}

	// Goroutines that repeatedly read Agents() and ActiveCount().
	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = tracker.Agents()
				_ = tracker.ActiveCount()
			}
		}()
	}

	// Wait for all spawns and reads to finish before completing agents.
	wg.Wait()

	// Verify all agents were tracked.
	agents := tracker.Agents()
	if len(agents) != numAgents {
		t.Fatalf("len(Agents()) = %d, want %d", len(agents), numAgents)
	}

	// Launch WaitAllDone in a goroutine — it should block until all complete.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- tracker.WaitAllDone(ctx)
	}()

	// Concurrently complete all agents while readers keep hitting the tracker.
	var wg2 sync.WaitGroup

	for i := range numAgents {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			threadID := fmt.Sprintf("agent-%d", i)
			tracker.ProcessEvent(&codex.CollabToolCallEvent{
				Phase:          codex.CollabToolCallCompletedPhase,
				Tool:           codex.CollabAgentToolCloseAgent,
				SenderThreadId: "parent",
				AgentsStates: map[string]codex.CollabAgentState{
					threadID: {Status: codex.CollabAgentStatusCompleted},
				},
			})
		}()
	}

	for range numReaders {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < 50; j++ {
				_ = tracker.Agents()
				_ = tracker.ActiveCount()
			}
		}()
	}

	wg2.Wait()

	// WaitAllDone should have unblocked.
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("WaitAllDone returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("WaitAllDone did not return after all agents completed")
	}

	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d, want 0", tracker.ActiveCount())
	}
}
