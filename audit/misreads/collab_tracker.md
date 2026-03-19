# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### AgentTracker.ProcessEvent ignores non-CollabToolCallEvent events silently

**Location:** `collab_tracker.go:46-49` — ProcessEvent type switch
**Date:** 2026-02-28

**Reason:** The finding claims "no test verifies that passing non-collab events is a no-op."
This is factually wrong. `TestAgentTrackerIgnoresNonCollabEvents` (collab_tracker_test.go:182-193)
passes `*TextDelta`, `*TurnCompleted`, and `*ItemStarted` events to `ProcessEvent` and asserts
that `tracker.Agents()` remains empty. The exact test the finding requests already exists.
