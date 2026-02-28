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
	mu     sync.RWMutex
	agents map[string]*AgentInfo
	cond   *sync.Cond
}

// NewAgentTracker creates an AgentTracker ready to process events.
func NewAgentTracker() *AgentTracker {
	t := &AgentTracker{
		agents: make(map[string]*AgentInfo),
	}
	t.cond = sync.NewCond(t.mu.RLocker())
	return t
}

// ProcessEvent updates the tracker from a stream event.
// Call this inside your Events() range loop.
func (t *AgentTracker) ProcessEvent(event Event) {
	switch e := event.(type) {
	case *CollabToolCallStarted:
		t.processCollab(e.Tool, e.SenderThreadId, e.AgentsStates)
	case *CollabToolCallCompleted:
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

	t.cond.Broadcast()
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
	done := make(chan struct{})
	go func() {
		t.mu.RLock()
		defer t.mu.RUnlock()
		for !t.allDone() {
			t.cond.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Unblock the waiting goroutine.
		t.cond.Broadcast()
		return ctx.Err()
	}
}

// allDone returns true if all tracked agents are in a terminal state.
// Must be called with at least a read lock held.
func (t *AgentTracker) allDone() bool {
	if len(t.agents) == 0 {
		return false
	}
	for _, info := range t.agents {
		if !info.isTerminal() {
			return false
		}
	}
	return true
}
