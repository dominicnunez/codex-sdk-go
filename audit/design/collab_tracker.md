# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### AgentTracker signals on every collab event including empty state updates

**Location:** `collab_tracker.go:71-72` — unconditional close(t.updated) signal
**Date:** 2026-02-28

**Reason:** The `close(t.updated)` signal fires even when `states` is empty, causing a spurious
wakeup in `WaitAllDone`. This is standard Go condition-variable semantics — waiters must recheck
their condition after every wakeup. `WaitAllDone` already does this correctly by calling
`t.allDone()` after every channel receive. Guarding the signal with `len(states) > 0` would
be a minor optimization but changes the notification contract — callers who depend on any
collab event (not just state changes) would miss updates. The current behavior is safe and
consistent with the documented wakeup-recheck pattern.
