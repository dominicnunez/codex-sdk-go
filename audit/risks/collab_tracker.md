### Unbounded agent map growth in AgentTracker

**Location:** `collab_tracker.go:51-73` — agents map never prunes terminal entries
**Date:** 2026-03-01

**Reason:** `AgentTracker.agents` grows without bound as agents reach terminal states. Adding
automatic pruning requires choosing a retention policy (time-based? count-based? immediate on
terminal?), which changes the observable behavior of `Agents()` — callers currently see the full
history of all agents. A `Prune()` method would be the least disruptive option but adds public
API surface. In practice, sub-agent counts per session are small (tens, occasionally hundreds).
The memory held per `AgentInfo` is ~100 bytes. Even 10,000 agents would consume ~1MB, which is
negligible relative to the process memory. The growth is linear in the number of unique agents
spawned, not in the number of events processed.
