### AgentTracker.ProcessEvent ignores non-CollabToolCallEvent events silently

**Location:** `46-49`

**Reason:** The finding claims "no test verifies that passing non-collab events is a no-op."
This is factually wrong. `TestAgentTrackerIgnoresNonCollabEvents` (collab_tracker_test.go:182-193)
passes `*TextDelta`, `*TurnCompleted`, and `*ItemStarted` events to `ProcessEvent` and asserts
that `tracker.Agents()` remains empty. The exact test the finding requests already exists.
