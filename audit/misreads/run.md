# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### RunResult.Response is not missing a fallback from turn/completed items

**Location:** `run.go:87` — `buildRunResult`

**Reason:** The finding depends on `turn/completed` carrying a populated `Turn.Items` list with the
final agent message. That behavior does not occur in this protocol. The source-of-truth schema at
`specs/v2/TurnCompletedNotification.json:1132-1145` says that for notifications returning a `Turn`,
`items` is an empty list; populated `Turn.Items` is only for `thread/resume` or `thread/fork`
responses. `buildRunResult` therefore is not overlooking a real fallback source in
`turn/completed`; the audit misread the protocol shape.

### Approval flow mid-turn claimed to have no test coverage

**Location:** `run.go:106-126`, `run_streamed.go:110-136` — Run/RunStreamed approval path
**Date:** 2026-03-01

**Reason:** The audit claims "No test exercises the full path where a `Run()` call triggers an
approval request mid-turn." This is factually wrong. `run_test.go:632-679` contains a test that
calls `proc.Run()`, injects a server→client approval request via `mock.InjectServerRequest` at
line 646 mid-turn, verifies the handler was called, then completes the turn with notifications.
`run_streamed_test.go:805-839` does the same for `RunStreamed`. Both tests exercise the full
path through `executeTurn` with approval flow.
