# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### UpdatePatchChangeKind MarshalJSON inconsistency with AddPatchChangeKind and DeletePatchChangeKind

**Location:** `event_types.go:147-153` — UpdatePatchChangeKind.MarshalJSON
**Date:** 2026-02-27

**Reason:** The audit claims `AddPatchChangeKind` (line 128) and `DeletePatchChangeKind` (line 136)
use "anonymous struct literals" for marshaling, and that `UpdatePatchChangeKind` is inconsistent
by using `map[string]interface{}`. This is factually wrong. All three types use the map pattern:
`AddPatchChangeKind` uses `map[string]string{"type": "add"}`, `DeletePatchChangeKind` uses
`map[string]string{"type": "delete"}`, and `UpdatePatchChangeKind` uses `map[string]interface{}`
(interface{} because it has an optional `move_path` field). There is no intra-file inconsistency.
The anonymous struct pattern exists in other files (`approval.go`, `config.go`, `review.go`)
but not in the types the audit references.

### Close cannot race with handleResponse to double-send on pending channel

**Location:** `stdio.go:197-213` — Close() pending request cleanup loop
**Date:** 2026-02-27

**Reason:** The audit claims `Close()` and `handleResponse` can both send into the same `pending.ch`,
causing a goroutine leak when `handleResponse`'s unconditional send blocks on a full buffer.
This cannot happen. `Close()` sets `t.closed = true` at line 191 under the mutex before
iterating `pendingReqs`. `handleResponse` at line 321 checks `t.closed` under the same mutex
and returns immediately if true. So after `Close()` runs, no `handleResponse` call will ever
proceed to delete an entry or send on a channel. In the reverse ordering — `handleResponse`
acquires the lock first, deletes the entry, releases the lock, then sends — `Close()` will not
find that entry in the map, so it never sends into the same channel. The two senders are
mutually exclusive by the `t.closed` flag under `t.mu`.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `approval.go:148-150`, `approval.go:409-411`, `approval.go:672-674`, `approval.go:802-804`, `review.go:87-89`, `event_types.go:188-190`, `event_types.go:313-315` — MarshalJSON on FileChangeWrapper, CommandActionWrapper, ParsedCommandWrapper, DynamicToolCallOutputContentItemWrapper, ReviewTargetWrapper, PatchChangeKindWrapper, WebSearchActionWrapper
**Date:** 2026-02-27

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### AccountWrapper nil receiver check is reachable via pointer field

**Location:** `account.go:96-97` — AccountWrapper.MarshalJSON pointer receiver
**Date:** 2026-02-27

**Reason:** The audit claims `AccountWrapper` is "used as a value type in struct fields" and
therefore the `a == nil` check on the pointer receiver is unreachable dead code. This is incorrect.
`GetAccountResponse` at `account.go:16` declares the field as `*AccountWrapper` (pointer), not
`AccountWrapper` (value). When this pointer field is nil, `json.Marshal` calls
`(*AccountWrapper)(nil).MarshalJSON()`, making the nil check both reachable and correct. The
pointer receiver is appropriate here precisely because the field is a pointer type.

### ReviewDecisionWrapper unmarshal correctly handles double-nested network_policy_amendment

**Location:** `approval.go:198-204` — ReviewDecisionWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** The audit claims that unmarshaling `raw` (the value at key `"network_policy_amendment"`)
into `NetworkPolicyAmendmentDecision` loses data because the struct expects a `"network_policy_amendment"`
JSON field that isn't present. This is incorrect. The spec (`ExecCommandApprovalResponse.json:71-82`,
`ApplyPatchApprovalResponse.json:71-82`) defines double nesting: the outer object has key
`"network_policy_amendment"` whose value is another object with an inner `"network_policy_amendment"`
key containing the actual `NetworkPolicyAmendment` data. So `raw` is
`{"network_policy_amendment":{"action":"...","host":"..."}}`, which correctly deserializes into
`NetworkPolicyAmendmentDecision{NetworkPolicyAmendment: ...}`. The roundtrip is not broken.

### RawResponseItemCompletedNotification intentionally omitted from Go types

**Location:** `specs/v2/RawResponseItemCompletedNotification.json` — no Go counterpart
**Date:** 2026-02-27

**Reason:** The audit claims the spec has no Go type and `TestSpecCoverage` may not catch it.
This is incorrect. `spec_coverage_test.go:90-92` explicitly documents that this schema "is not
referenced in ServerNotification.json" and "is not part of the wire protocol; implementing it
would be dead code." The test at line 105 explicitly exempts it. The server never emits this
notification — `ServerNotification.json` does not reference it — so there is nothing to drop.

### ThreadStartParams.ApprovalPolicy bare interface marshaling flagged as new finding but already covered

**Location:** `thread.go:586` — ThreadStartParams.ApprovalPolicy field type
**Date:** 2026-02-27

**Reason:** The audit re-flagged the bare interface typing of `ApprovalPolicy` on params structs
as a new Medium-severity bug. The audit itself acknowledges "This finding is already covered by
the exception and is noted here for completeness — no new action required." The existing exception
"Params structs use bare interface instead of wrapper type" at `thread.go:538` et al. already
covers this exact issue. This is a duplicate, not a new finding.

### AppsListParams.ForceRefetch described as missing omitempty but it has omitempty

**Location:** `apps.go:11` — ForceRefetch field tag
**Date:** 2026-02-27

**Reason:** The audit claims `ForceRefetch bool` has "no `omitempty` tag" and that "the zero value
`false` is always serialized." This is factually wrong. The actual field declaration is
`ForceRefetch bool \`json:"forceRefetch,omitempty"\`` — it already has omitempty. With `bool` +
`omitempty`, the `false` value is *omitted* (not sent), which is the opposite of what the audit
describes. The stated problem ("missing omitempty sends false as default") does not occur.

### Scanner buffer sizes are named constants, not magic numbers

**Location:** `stdio.go:226-227` — readLoop buffer constants
**Date:** 2026-02-27

**Reason:** The audit labels `initialBufferSize` and `maxMessageSize` as "magic numbers" but
the code defines them as named constants with descriptive comments (`// 64KB`, `// 10MB —
file diffs and base64 payloads exceed the default`). They are appropriately scoped to the
function that uses them. The actual concern — that callers can't tune them without modifying
source — is a feature request for configurability, not a code quality defect.

### handleApproval marshal error does not leak internal structure

**Location:** `client.go:254-256` — json.Marshal error in handleApproval
**Date:** 2026-02-27

**Reason:** The audit claims the raw `json.Marshal` error leaks type information across the trust
boundary and is "visible in any error-logging or debugging path before it reaches the transport."
This is incorrect. The error propagates directly to `handleRequest` in `stdio.go:314`, which
immediately replaces it with a hardcoded `"internal handler error"` message (stdio.go:327) before
sending the JSON-RPC response. The original error string is never logged, stored, or exposed to
any external party. There is no logging or debugging path in this code — the error goes from
`handleApproval` return → `handleRequest` goroutine → generic error response. The internal type
information never crosses any trust boundary.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper field names differ because the specs differ

**Location:** `approval.go:189-195` — ReviewDecisionWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** The audit claims the naming difference between `ApprovedExecpolicyAmendmentDecision.ProposedExecpolicyAmendment`
(in `ReviewDecisionWrapper`) and `AcceptWithExecpolicyAmendmentDecision.ExecpolicyAmendment`
(in `CommandExecutionApprovalDecisionWrapper`) is a code-level inconsistency. This is incorrect.
These are two different spec schemas with different JSON field names:
`ApplyPatchApprovalResponse.json` / `ExecCommandApprovalResponse.json` define outer key
`"approved_execpolicy_amendment"` with inner field `"proposed_execpolicy_amendment"`, while
`CommandExecutionRequestApprovalResponse.json` defines outer key `"acceptWithExecpolicyAmendment"`
with inner field `"execpolicy_amendment"`. The Go types faithfully mirror the specs. The naming
difference originates in the protocol definition, not in the Go code.

### Comment about silently ignoring unmarshal errors claimed to be copy-pasted across 25+ handlers

**Location:** `streaming.go:100`, `thread_notifications.go:18`, `turn_notifications.go:18`, `account_notifications.go:17`, `realtime.go:18` — notification handlers
**Date:** 2026-02-28

**Reason:** The audit claims the comment `// Silently ignore unmarshal errors (notification is malformed)`
appears in "every notification handler" and is "copy-pasted across 25+ handler methods." This is
factually wrong. The comment exists in exactly one location: `streaming.go:100`. The other file:line
references (`thread_notifications.go:18`, `turn_notifications.go:18`, `account_notifications.go:17`,
`realtime.go:18`) point to struct type definitions, not comments or handler methods. Most notification
handlers silently return on unmarshal error without any comment at all — but the finding's claim that
a specific comment is duplicated across 25+ methods does not match the code.

### go.mod specifies go 1.25 which does not exist

**Location:** `go.mod:3` — go directive version
**Date:** 2026-02-27

**Reason:** The audit claims "Go 1.25 has not been released" and "As of February 2026, Go 1.24
is the latest stable release." This is factually incorrect. Go 1.25 was released on August 12,
2025 — over six months before this audit. The `go 1.25` directive in go.mod is valid and refers
to an existing, stable Go release.

### Process.Wait claimed to have zero test coverage

**Location:** `process.go:191-198` — Process.Wait method
**Date:** 2026-02-28

**Reason:** The audit claims "Process.Wait() has zero test coverage. No test calls Wait()."
This is factually wrong. `process_test.go` calls `proc.Wait()` at lines 97, 153, 245, and 333.
The Wait+Close race is untested, but the method itself is exercised in multiple tests.

### Conversation multi-turn accumulation claimed to be untested

**Location:** `conversation.go:102-122` — Conversation.Turn multi-turn path
**Date:** 2026-02-28

**Reason:** The audit claims "conversation_test.go tests StartConversation and a single Turn, but
does not test the multi-turn accumulation path where onComplete appends turns to c.thread.Turns."
This is factually wrong. `TestConversationMultiTurn` (conversation_test.go:12-95) executes two turns
on the same Conversation, then asserts `len(thread.Turns) == 2` at line 92-93. The `Thread()` snapshot
method and multi-turn accumulation are both tested.

### Streamed error paths claimed to have no coverage

**Location:** `run_streamed_test.go` — streamed error path tests
**Date:** 2026-02-28

**Reason:** The audit claims "these are the three non-happy-path branches in executeStreamedTurn and
none are exercised." Two of the three paths are tested: `turn/completed` with `Turn.Error` is tested
by `TestRunStreamedTurnError` (run_streamed_test.go:128-173) and `TestConversationTurnStreamedTurnError`
(conversation_test.go:348-390). Context cancellation during streaming is tested by
`TestRunStreamedContextCancellation` (run_streamed_test.go:88-107) and
`TestConversationTurnStreamedContextCancel` (conversation_test.go:392-416). Only the `turn/completed`
unmarshal failure path genuinely lacks a test, but the blanket claim "none are exercised" is false.

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

### AgentTracker.ProcessEvent ignores non-CollabToolCallEvent events silently

**Location:** `collab_tracker.go:46-49` — ProcessEvent type switch
**Date:** 2026-02-28

**Reason:** The finding claims "no test verifies that passing non-collab events is a no-op."
This is factually wrong. `TestAgentTrackerIgnoresNonCollabEvents` (collab_tracker_test.go:182-193)
passes `*TextDelta`, `*TurnCompleted`, and `*ItemStarted` events to `ProcessEvent` and asserts
that `tracker.Agents()` remains empty. The exact test the finding requests already exists.
