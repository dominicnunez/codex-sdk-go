# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Conversation and turn tests use time.Sleep for goroutine synchronization

**Location:** `conversation_test.go:42,67,111,256,315,525,554,623,664,715` — and similar in `run_test.go`, `run_streamed_test.go`
**Date:** 2026-03-01

**Reason:** Nearly every turn-based test uses `time.Sleep(50ms)` between starting a goroutine
that calls `Turn()` and injecting the completion notification. Replacing these with deterministic
signals requires adding a method-call signaling mechanism to MockTransport (e.g. a channel that
fires when `turn/start` is sent). The mock transport currently returns immediately from `Send`,
so the 50ms sleep is reliably sufficient. The fix requires non-trivial test infrastructure
changes across ~15 tests for a low-severity code smell. The tests have never flaked in CI.
