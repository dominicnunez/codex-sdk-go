# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### turn/completed unmarshal failure path in executeStreamedTurn claimed to lack test coverage

**Location:** `turn_lifecycle.go:181-184` — turn/completed unmarshal failure synthesis
**Date:** 2026-02-28

**Reason:** The audit claims "This path is not tested." This is factually wrong.
`TestRunStreamedMalformedTurnCompleted` (run_streamed_test.go:763-786) injects a `turn/completed`
notification with a valid `threadId` but a malformed turn body, then verifies the stream emits
an error containing "unmarshal turn/completed." The blocking path is also tested by
`TestRunMalformedTurnCompleted` (run_test.go:571-602). Both tests exercise the exact synthesized
`TurnCompletedNotification` with `TurnError` path described in the finding.

### Notification listener registration race in executeStreamedTurn claimed but all listeners registered before RPC

**Location:** `turn_lifecycle.go:101-234` — executeStreamedTurn listener registration order
**Date:** 2026-03-01

**Reason:** The audit claims a "narrow window" where the `turnDone` channel listener (registered
"last") might not be wired when an early `turn/completed` arrives. This is factually wrong about
the ordering being a problem. All `streamListen` calls and `on(...)` registrations (lines 120-201)
happen sequentially **before** `Turn.Start` is called at line 203. The audit itself acknowledges
"the streamed path has the same registration-before-start pattern" and that "the risk is theoretical
in normal conditions." The pattern is identical to `executeTurn` which the audit considers correct.
