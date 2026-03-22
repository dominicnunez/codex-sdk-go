### Streamed validation failures already populate the collector summary

**Location:** `250`

**Reason:** The current `runStreamedWithCollector` path already routes synchronous
validation failures through `newCollectedErrorStream`, which records the error in
the collector before returning the terminal error stream. The checked-in tests
cover both nil-context and empty-prompt collector cases and assert that
`Summary().NormalizedErrors` contains the validation error. The finding is stale
against the current implementation and test suite.

### Approval flow mid-turn claimed to have no test coverage

**Location:** `110-136`

**Reason:** The audit claims "No test exercises the full path where a `Run()` call triggers an
approval request mid-turn." This is factually wrong. `run_test.go:632-679` contains a test that
calls `proc.Run()`, injects a server→client approval request via `mock.InjectServerRequest` at
line 646 mid-turn, verifies the handler was called, then completes the turn with notifications.
`run_streamed_test.go:805-839` does the same for `RunStreamed`. Both tests exercise the full
path through `executeTurn` with approval flow.

### Result() described as blocking forever on cancelled context

**Location:** `51-56`

**Reason:** The audit itself acknowledges "it does close `s.done` (via `defer close(s.done)`), so
this actually works." The lifecycle goroutine at `run_streamed.go:121-123` always closes `s.done`
via defer, including on context cancellation — so `Result()` never blocks forever. The remaining
concern ("nil means the turn did not complete successfully" has no error return) is documented API
behavior at line 49-50: "Returns nil if the turn errored (the error was already surfaced through
the Events iterator)." This is an API design preference, not a bug.

### Stream.Events() single-use enforcement claimed to be untested

**Location:** `39-46`

**Reason:** The audit claims there is no test verifying that calling `Events()` twice yields
`ErrStreamConsumed` on the second call. This is factually wrong. `run_streamed_test.go` contains
multiple tests for this: lines 890-901 call `Events()` twice and assert the second iterator yields
`ErrStreamConsumed`; lines 1007-1023 do the same in a different test scenario; and
`TestStreamEventsConcurrentConsumption` (line 1025) tests concurrent access with 10 goroutines
racing to consume, asserting exactly 1 winner and N-1 `ErrStreamConsumed` results.

### Stream.Events iterator doesn't drain on early break described as a new finding

**Location:** `136-142`

**Reason:** This is a duplicate of the known exception "Stream background goroutine blocks if
consumer stops iterating without cancelling context" which describes the identical behavior —
the lifecycle goroutine blocks on send when the consumer stops reading and the buffer fills.
The existing exception documents that context cancellation is the correct cleanup mechanism
and that adding a `done` channel would complicate the iterator contract.

### Stream early-break cleanup behavior claimed to be untested

**Location:** `126`

**Reason:** Factually wrong. `run_streamed_test.go:507` contains `TestRunStreamedEarlyBreak`
which starts `RunStreamed`, reads 1 event, breaks out of the `Events()` loop, then verifies
`Result()` returns within 3 seconds (not hanging). This tests exactly the scenario the finding
describes — early break from the iterator followed by lifecycle goroutine cleanup.

### Stream channel buffer size described as magic number

**Location:** `18`

**Reason:** The audit itself concludes "This is a named constant, not a magic number — no issue
here" and "Suggested fix: None needed." A finding that self-invalidates is not actionable.

### newErrorStream does not bypass the consumed guard

**Location:** `151-160`

**Reason:** The audit claims `newErrorStream` "bypasses consumed guard, allowing double iteration."
This is wrong. `newErrorStream` creates a `Stream` with `consumed` at its zero value (`false`).
The `Events()` method uses `consumed.CompareAndSwap(false, true)` — the first call succeeds and
returns the events iterator; the second call fails the CAS and returns `ErrStreamConsumed`. The
`consumed` guard works identically for `newErrorStream` and `newStream`. The `events` field is
unexported, so callers cannot access the iterator except through `Events()`.

### Collector summaries drop stream buffer overflow errors

**Location:** `57`

**Reason:** `guardedChan.send` already records `ErrStreamOverflow` and routes it through the
collector via `setOverflowHandler`, which `RunStreamedWithCollector` installs before the lifecycle
starts. The regression test `TestRunStreamedWithCollectorReportsOverflowInSummary` passes, so the
reported overflow gap does not exist in the current code.

### Streamed validation failures already populate the collector summary

**Location:** `250`

**Reason:** The current `runStreamedWithCollector` path already routes synchronous
validation failures through `newCollectedErrorStream`, which records the error in
the collector before returning the terminal error stream. The checked-in tests
cover both nil-context and empty-prompt collector cases and assert that
`Summary().NormalizedErrors` contains the validation error. The finding is stale
against the current implementation and test suite.

### Approval flow mid-turn claimed to have no test coverage

**Location:** `110-136`

**Reason:** The audit claims "No test exercises the full path where a `Run()` call triggers an
approval request mid-turn." This is factually wrong. `run_test.go:632-679` contains a test that
calls `proc.Run()`, injects a server→client approval request via `mock.InjectServerRequest` at
line 646 mid-turn, verifies the handler was called, then completes the turn with notifications.
`run_streamed_test.go:805-839` does the same for `RunStreamed`. Both tests exercise the full
path through `executeTurn` with approval flow.
