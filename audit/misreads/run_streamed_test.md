### Streamed validation failures already populate the collector summary

**Location:** `154`

**Reason:** The current `runStreamedWithCollector` path already routes synchronous
validation failures through `newCollectedErrorStream`, which records the error in
the collector before returning the terminal error stream. The checked-in tests
cover both nil-context and empty-prompt collector cases and assert that
`Summary().NormalizedErrors` contains the validation error. The finding is stale
against the current implementation and test suite.

### Streamed validation failures already populate the collector summary

**Location:** `154`

**Reason:** The current `runStreamedWithCollector` path already routes synchronous
validation failures through `newCollectedErrorStream`, which records the error in
the collector before returning the terminal error stream. The checked-in tests
cover both nil-context and empty-prompt collector cases and assert that
`Summary().NormalizedErrors` contains the validation error. The finding is stale
against the current implementation and test suite.

### Streamed error paths claimed to have no coverage

**Location:** `N/A`

**Reason:** The audit claims "these are the three non-happy-path branches in executeStreamedTurn and
none are exercised." Two of the three paths are tested: `turn/completed` with `Turn.Error` is tested
by `TestRunStreamedTurnError` (run_streamed_test.go:128-173) and `TestConversationTurnStreamedTurnError`
(conversation_test.go:348-390). Context cancellation during streaming is tested by
`TestRunStreamedContextCancellation` (run_streamed_test.go:88-107) and
`TestConversationTurnStreamedContextCancel` (conversation_test.go:392-416). Only the `turn/completed`
unmarshal failure path genuinely lacks a test, but the blanket claim "none are exercised" is false.
