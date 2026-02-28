package codex

import (
	"context"
	"sync"
)

// AgentInfo tracks the observed state of a single sub-agent.
type AgentInfo struct {
	ThreadID  string
	Status    CollabAgentStatus
	Message   string
	Tool      CollabAgentTool
	SpawnedBy string
}

// isTerminal returns true if the agent has reached a final status.
func (a *AgentInfo) isTerminal() bool {
	switch a.Status {
	case CollabAgentStatusCompleted, CollabAgentStatusErrored,
		CollabAgentStatusShutdown, CollabAgentStatusNotFound:
		return true
	}
	return false
}

// AgentTracker maintains a live map of agent states by processing collab events.
// Safe for concurrent reads via RWMutex.
type AgentTracker struct {
	mu      sync.RWMutex
	agents  map[string]*AgentInfo
	updated chan struct{} // closed + replaced on every state change
}

// NewAgentTracker creates an AgentTracker ready to process events.
func NewAgentTracker() *AgentTracker {
	return &AgentTracker{
		agents:  make(map[string]*AgentInfo),
		updated: make(chan struct{}),
	}
}

// ProcessEvent updates the tracker from a stream event.
// Call this inside your Events() range loop.
func (t *AgentTracker) ProcessEvent(event Event) {
	if e, ok := event.(*CollabToolCallEvent); ok {
		t.processCollab(e.Tool, e.SenderThreadId, e.AgentsStates)
	}
}

func (t *AgentTracker) processCollab(tool CollabAgentTool, sender string, states map[string]CollabAgentState) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for threadID, state := range states {
		info, exists := t.agents[threadID]
		if !exists {
			info = &AgentInfo{ThreadID: threadID}
			t.agents[threadID] = info
		}
		info.Status = state.Status
		if state.Message != nil {
			info.Message = *state.Message
		}
		info.Tool = tool
		if tool == CollabAgentToolSpawnAgent && !exists {
			info.SpawnedBy = sender
		}
	}

	close(t.updated)
	t.updated = make(chan struct{})
}

// Agents returns a snapshot of all tracked agents.
func (t *AgentTracker) Agents() map[string]AgentInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]AgentInfo, len(t.agents))
	for k, v := range t.agents {
		result[k] = *v
	}
	return result
}

// Agent returns info for a specific agent thread ID.
func (t *AgentTracker) Agent(threadID string) (AgentInfo, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, ok := t.agents[threadID]
	if !ok {
		return AgentInfo{}, false
	}
	return *info, true
}

// ActiveCount returns the number of agents in running or pendingInit status.
func (t *AgentTracker) ActiveCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, info := range t.agents {
		if !info.isTerminal() {
			count++
		}
	}
	return count
}

// WaitAllDone blocks until all tracked agents reach a terminal status
// or the context is cancelled.
func (t *AgentTracker) WaitAllDone(ctx context.Context) error {
	for {
		t.mu.RLock()
		if t.allDone() {
			t.mu.RUnlock()
			return nil
		}
		ch := t.updated
		t.mu.RUnlock()

		select {
		case <-ch:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// allDone returns true if all tracked agents are in a terminal state.
// Returns true on an empty tracker (vacuously true).
// Must be called with at least a read lock held.
func (t *AgentTracker) allDone() bool {
	for _, info := range t.agents {
		if !info.isTerminal() {
			return false
		}
	}
	return true
}
