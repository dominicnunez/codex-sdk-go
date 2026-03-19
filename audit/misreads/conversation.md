# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Conversation multi-turn accumulation claimed to be untested

**Location:** `conversation.go:102-122` — Conversation.Turn multi-turn path
**Date:** 2026-02-28

**Reason:** The audit claims "conversation_test.go tests StartConversation and a single Turn, but
does not test the multi-turn accumulation path where onComplete appends turns to c.thread.Turns."
This is factually wrong. `TestConversationMultiTurn` (conversation_test.go:12-95) executes two turns
on the same Conversation, then asserts `len(thread.Turns) == 2` at line 92-93. The `Thread()` snapshot
method and multi-turn accumulation are both tested.

### TurnStreamed captures stale thread snapshot for RunResult

**Location:** `conversation.go:162-165` — turnStreamedLifecycle thread capture
**Date:** 2026-02-28

**Reason:** This finding claims to be "a separate semantic issue" from the mutex race (finding 2),
stating that "even with the lock fix, the snapshot semantics are ambiguous." The race condition
is already captured by the mutex finding (which remains in the report). The "ambiguous semantics"
claim is incorrect — the design exception at `audit/exceptions/design.md:249-260` already documents
that `RunResult.Thread` deliberately reflects thread metadata at turn-start time, not post-turn state.
The semantics are defined and accepted, not ambiguous. This finding is a duplicate of the mutex
race + the existing design exception.

### Conversation.Thread() deep-copy semantics claimed to be untested

**Location:** `conversation.go:45-56` — Thread() deep-copy
**Date:** 2026-03-01

**Reason:** The audit claims "This invariant (append-safe but mutation-visible) is not tested."
This is factually wrong. `conversation_test.go:505-515` contains a test that calls `conv.Thread()`,
appends a Turn to the returned snapshot, then calls `conv.Thread()` again and asserts the length
is unchanged — verifying that the Conversation's internal state is unaffected by mutations to
the snapshot.

### Concurrent turn rejection claimed to be untested

**Location:** `conversation.go:173-178` — activeTurn exclusion logic
**Date:** 2026-03-01

**Reason:** The audit claims the `errTurnInProgress` concurrent-exclusion logic has no test.
This is factually wrong. `conversation_test.go` contains four dedicated concurrent turn rejection tests:
`TestConversationConcurrentTurnRejected` (line 507), `TestConversationConcurrentTurnStreamedRejected`
(line 651), `TestConversationConcurrentTurnVsTurnStreamedRejected` (line 697), and
`TestConversationConcurrentTurnStreamedVsTurnRejected` (line 746). These test all four combinations
of Turn vs TurnStreamed racing and assert the second call returns an error.

### Conversation.Thread() claimed to not deep-copy TokenUsage or other top-level fields

**Location:** `conversation.go:51-83` — Thread() deep-copy
**Date:** 2026-03-01

**Reason:** The finding claims Thread() "does not deep-copy TokenUsage or other potential top-level fields."
`TokenUsage` does not exist on the `Thread` struct (thread.go:20-37). Every pointer and slice field on
Thread is deep-copied: `Name`, `AgentNickname`, `AgentRole`, `Path` (all `*string` — cloned via
`cloneStringPtr`), `GitInfo` (`*GitInfo` — field-by-field deep copy), `Source` (`SessionSourceWrapper` —
JSON round-trip clone), `Status` (`ThreadStatusWrapper` — JSON round-trip clone), `Turns` (`[]Turn` —
slice copy with per-item deep copy of Items and Error). The concern about "future fields" is speculative.

### Multi-turn state accumulation claimed to be untested

**Location:** `conversation.go`, `conversation_test.go` — multi-turn testing
**Date:** 2026-03-01

**Reason:** Already in known exceptions. `TestConversationMultiTurn` (conversation_test.go:13-96)
executes two turns on the same Conversation, then asserts `len(thread.Turns) == 2` at line 93-94.
The multi-turn accumulation path is tested.

### Conversation.TurnStreamed concurrent call rejection claimed to be untested

**Location:** `conversation.go:238` — TurnStreamed activeTurn check
**Date:** 2026-03-01

**Reason:** Factually wrong. `conversation_test.go` contains four dedicated concurrent turn
rejection tests: `TestConversationConcurrentTurnRejected` (line 507),
`TestConversationConcurrentTurnStreamedRejected` (line 651),
`TestConversationConcurrentTurnVsTurnStreamedRejected` (line 697), and
`TestConversationConcurrentTurnStreamedVsTurnRejected` (line 746). These test all four
combinations of Turn vs TurnStreamed racing and assert `errTurnInProgress` is returned.

### Concurrent Turn exclusion claimed to lack real-timing test but test exists

**Location:** `conversation.go:200-235` — errTurnInProgress guard
**Date:** 2026-03-01

**Reason:** The audit claims the test is "only in a sequential setup." This is factually wrong.
`TestConversationConcurrentTurnRejected` (conversation_test.go:507) starts a turn in a goroutine,
waits 50ms for it to become active, then calls `Turn` from the main goroutine — this IS a concurrent
test with real timing. The mock hasn't responded yet when the second call happens, so the first turn
is genuinely active. All four concurrent combinations (Turn/Turn, TurnStreamed/TurnStreamed,
Turn/TurnStreamed, TurnStreamed/Turn) are tested at lines 507, 651, 697, and 746.

### Thread() deep-copy does not have a gap in Turn field cloning

**Location:** `conversation.go:68-81` — Thread() clone logic
**Date:** 2026-03-01

**Reason:** The audit claims the clone logic is "scattered" and risks missing fields, but the `Turn`
struct has exactly four fields: `ID` (string, value-copied), `Status` (TurnStatus string, value-copied),
`Items` (deep-copied item-by-item via `cloneThreadItemWrapper`), and `Error` (deep-copied with
`CodexErrorInfo` and `AdditionalDetails` handled explicitly). Every field is correctly cloned.
The finding itself admits "the current code is correct" — the concern is purely speculative about
hypothetical future fields, which is not an actionable finding.

### Clone fallback comments no longer describe dropped unknown variants

**Location:** `conversation.go:312`, `conversation_internal_test.go:455` — clone fallback semantics

**Reason:** The current comment says the fallback preserves unexpected
in-memory values with a reflective deep clone when the JSON round-trip path does
not work, which matches the implementation and tests. The regression test
`TestCloneFallbacksPreserveUncloneableValues` explicitly verifies that these
fallback helpers preserve uncloneable values instead of dropping them.
