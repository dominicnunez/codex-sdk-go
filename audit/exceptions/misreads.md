# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

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

### TurnStartParams SandboxPolicy marshal finding is a duplicate of existing design exception

**Location:** `turn.go:22-33` — TurnStartParams.SandboxPolicy field
**Date:** 2026-02-28

**Reason:** The audit flags that `TurnStartParams.SandboxPolicy` (bare interface `*SandboxPolicy`)
does not inject the `"type"` discriminator for struct variants like `SandboxPolicyWorkspaceWrite`.
The audit itself acknowledges "Already covered by the design exception 'Params structs use bare
interface instead of wrapper type.'" This is the exact same issue documented at
`audit/exceptions/design.md:63-77`, which covers all bare-interface policy fields on params
structs including `TurnStartParams.SandboxPolicy` at `turn.go:30`. Duplicate finding.

### handleApproval marshalForWire pointer-to-result described as a potential bug but works correctly

**Location:** `client.go:324` — marshalForWire(&result) call
**Date:** 2026-02-28

**Reason:** The audit describes a theoretical scenario where `marshalForWire(&result)` could fail
to satisfy the `wireMarshaler` interface check for future types. The audit itself concludes
"The current code is correct for all existing types" and "No change needed for current types.
This is noted for awareness." A finding that describes correct behavior and requires no change
is not an actionable finding — it is a speculative concern about hypothetical future types.

### turn/completed unmarshal failure path in executeStreamedTurn claimed to lack test coverage

**Location:** `turn_lifecycle.go:181-184` — turn/completed unmarshal failure synthesis
**Date:** 2026-02-28

**Reason:** The audit claims "This path is not tested." This is factually wrong.
`TestRunStreamedMalformedTurnCompleted` (run_streamed_test.go:763-786) injects a `turn/completed`
notification with a valid `threadId` but a malformed turn body, then verifies the stream emits
an error containing "unmarshal turn/completed." The blocking path is also tested by
`TestRunMalformedTurnCompleted` (run_test.go:571-602). Both tests exercise the exact synthesized
`TurnCompletedNotification` with `TurnError` path described in the finding.

### normalizeID already has a precision guard for large float64 values

**Location:** `stdio.go:48-51` — normalizeID float64-to-uint64 cast
**Date:** 2026-03-01

**Reason:** The audit claims `normalizeID` casts `float64` to `uint64` "without checking whether
the conversion loses precision for integers above 2^53." This is factually wrong. The code at
lines 48-51 does: `u := uint64(v)` then `if v == float64(u)` — this round-trip check is exactly
the precision guard the audit suggests adding. For values above 2^53 where the float64 cannot
represent the integer exactly, `v == float64(u)` will be false, and the code falls through to
`fmt.Sprintf("%v", v)`. The suggested fix ("only use the integer fast-path when
`v == float64(uint64(v))` is exact") is already implemented.

### Close and handleResponse race claimed but audit concludes code is safe

**Location:** `stdio.go:201-216, stdio.go:371-378` — Close() and handleResponse() interaction
**Date:** 2026-03-01

**Reason:** The audit's own analysis concludes "This is actually safe" and the suggested fix is
"Add a comment explaining why the `select/default` is safe." A finding that concludes the code
is correct and only needs a comment is not a code defect. The invariants are already documented
in the code: the comment at line 378 states "safe: buffer 1, only one sender claims via delete"
and the Close() comments at lines 198-200 and 210-211 explain the defensive pattern.

### internalListenerSeq described as inconsistent but acknowledged as correct

**Location:** `client.go:56` — internalListenerSeq counter
**Date:** 2026-03-01

**Reason:** The audit's own conclusion states "No actual bug" and "No change needed — the mutex
protection is sufficient and the pattern is deliberate since the listener map also needs the lock."
A finding that explicitly states no bug exists and no change is needed is not actionable.
`internalListenerSeq` is always accessed under `listenersMu.Lock()` because the listener map
operations require the same lock — using a separate atomic would be unnecessary.

### StdioTransport claimed to have no pipe-based integration tests

**Location:** `stdio.go` — StdioTransport test coverage
**Date:** 2026-03-01

**Reason:** The audit claims "There are no tests that exercise the actual readLoop, handleResponse,
handleRequest, and handleNotification codepaths with real pipe-based I/O." This is factually wrong.
`stdio_test.go` contains extensive pipe-based integration tests using `io.Pipe()` → `NewStdioTransport`:
`TestStdioNewlineDelimitedJSON` (pipe I/O with Send/response), `TestStdioConcurrentRequestDispatch`
(server→client requests via pipe), `TestStdioResponseRequestIDMatching` (concurrent sends with pipe),
`TestStdioNotificationDispatch` (server→client notifications via pipe), `TestStdioMixedMessageTypes`
(concurrent requests/responses/notifications), `TestStdioInvalidJSON` (malformed JSON recovery),
`TestStdioContextCancellation`, `TestStdioRequestHandlerPanicRecovery`, `TestStdioScannerBufferOverflow`
(10MB+ message), `TestStdioHandleResponseUnmarshalError`, and `TestStdioConcurrentSendAndClose`.

### Conversation.Thread() deep-copy semantics claimed to be untested

**Location:** `conversation.go:45-56` — Thread() deep-copy
**Date:** 2026-03-01

**Reason:** The audit claims "This invariant (append-safe but mutation-visible) is not tested."
This is factually wrong. `conversation_test.go:505-515` contains a test that calls `conv.Thread()`,
appends a Turn to the returned snapshot, then calls `conv.Thread()` again and asserts the length
is unchanged — verifying that the Conversation's internal state is unaffected by mutations to
the snapshot.

### normalizeID uint64 overflow described as new finding but covered by existing precision exception

**Location:** `stdio.go:47-51` — normalizeID float64 to uint64 cast
**Date:** 2026-03-01

**Reason:** The audit claims float64 values near `math.MaxUint64` produce undefined behavior in the
uint64 cast. This is a subset of the existing exception "normalizeID relies on float64 precision for
JSON number IDs" — values near `math.MaxUint64` are even more unrealistic than values above 2^53.
JSON-RPC IDs are small sequential integers; the existing round-trip check `v == float64(u)` at
lines 49-50 already guards against precision loss. The overflow edge case is not reachable in any
realistic protocol usage.

### handleApproval pointer-to-value wireMarshaler dispatch re-flagged as new finding

**Location:** `client.go:324` — marshalForWire(&result) call
**Date:** 2026-03-01

**Reason:** This is a duplicate of the existing exception "handleApproval marshalForWire pointer-to-result
described as a potential bug but works correctly." The audit itself concludes "No change needed for
current types" — all existing approval response types use pointer receivers, and the `&result` pattern
works correctly. A speculative concern about hypothetical future types is not an actionable finding.

### RPCError.Is nil-nil path flagged as unreachable dead code

**Location:** `errors.go:61-70` — RPCError.Is nil guard
**Date:** 2026-03-01

**Reason:** This is covered by the existing exception for RPCError.Is which states: "The nil-nil
comparison path is unreachable since `NewRPCError` is never called with nil, but the nil guard is
a defensive correctness check, not dead logic worth removing." Defensive nil checks in `Is()`
implementations are standard Go practice — they prevent panics if the type is ever constructed
outside the canonical constructor.

### Send pending request context cancellation described as fragile but audit concludes no bug

**Location:** `stdio.go:91-142` — Send() pending request lifecycle
**Date:** 2026-03-01

**Reason:** The audit's own analysis concludes "No actual bug" and "No code change required. This
is a documentation-level observation." The deferred `delete` is idempotent — if `handleResponse`
already claimed and deleted the entry, the defer is a no-op (deleting a key that no longer exists
in the map). The audit acknowledges the pattern is safe and only speculates about fragility "if
`handleResponse` ever changes to not delete." A finding that explicitly states no bug exists and
proposes no code change is not an actionable finding.

### Approval flow mid-turn claimed to have no test coverage

**Location:** `run.go:106-126`, `run_streamed.go:110-136` — Run/RunStreamed approval path
**Date:** 2026-03-01

**Reason:** The audit claims "No test exercises the full path where a `Run()` call triggers an
approval request mid-turn." This is factually wrong. `run_test.go:632-679` contains a test that
calls `proc.Run()`, injects a server→client approval request via `mock.InjectServerRequest` at
line 646 mid-turn, verifies the handler was called, then completes the turn with notifications.
`run_streamed_test.go:805-839` does the same for `RunStreamed`. Both tests exercise the full
path through `executeTurn` with approval flow.

### Config values passed to CLI args without sanitization described as security risk

**Location:** `process.go:89` — buildArgs config flag construction
**Date:** 2026-03-01

**Reason:** The audit claims config values concatenated into CLI args could allow shell metacharacter
injection or flag misinterpretation. This is incorrect. `exec.Command` does not invoke a shell — each
argument is passed as a discrete `argv` element, so shell metacharacters have no effect. The `--config`
flag and `k=v` value are passed as two separate arguments (not one), so the value cannot be
misinterpreted as a flag. The `=` ambiguity concern is already covered by the known exception
"Config flag values containing '=' are ambiguous on the CLI." The security framing is misleading
because `exec.Command` eliminates the actual attack vector.

### Close does not wait for readLoop to finish described as a separate bug

**Location:** `stdio.go:192-225` — Close() and readLoop interaction
**Date:** 2026-03-01

**Reason:** The audit suggests waiting for `<-t.readerStopped` at the end of `Close()`. This would
deadlock because the readLoop is blocked on `scanner.Scan()` which cannot be interrupted without
closing the underlying reader. This is the same root cause as the known exception "StdioTransport.Close
does not stop the reader goroutine" — you can't usefully wait for something you can't stop. The
suggested fix would make `Close()` hang indefinitely on a stuck reader.

### Write goroutine leak on context cancellation described as a new finding

**Location:** `stdio.go:117-119` — Send write goroutine
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "Write goroutine in Send can leak
on context cancellation" at `stdio.go:86-102`. The finding references different line numbers but
describes identical behavior — the write goroutine may outlive the cancelled context because
`io.Writer.Write` has no context support. Duplicate of existing exception.

### handleApproval error swallowed into generic message described as a new finding

**Location:** `stdio.go:432-451` — handleRequest error translation
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "Handler errors in handleApproval
are invisible to SDK consumers" at `client.go:273-274`. Both describe the same behavior — handler
errors are replaced with a generic "internal handler error" response on the wire. Duplicate of
existing exception.

### McpToolCallResult uses untyped interface{} slices described as a code quality issue

**Location:** `event_types.go:212-213` — McpToolCallResult.Content and StructuredContent
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "McpToolCallResult.Content and MCP
metadata fields use untyped interface{}" which explains that the upstream spec defines these as
open-schema fields (`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping
for a spec that deliberately leaves the type open. Duplicate of existing exception.

### DynamicToolCallParams.Arguments uses untyped interface{} described as a code quality issue

**Location:** `approval.go:754` — DynamicToolCallParams.Arguments
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which explains
the deliberate caller-convenience tradeoff. Duplicate of existing exception.

### TurnStartParams.OutputSchema uses untyped interface{} described as a code quality issue

**Location:** `turn.go:28` — TurnStartParams.OutputSchema
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which covers
this field explicitly. Duplicate of existing exception.

### Concurrent notification listener subscribe/unsubscribe claimed to be untested

**Location:** `client.go:214-235` — addNotificationListener concurrent safety
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for subscribe/unsubscribe racing with
dispatch. This is factually wrong. `listener_test.go:33-63` contains `TestConcurrentInternalListeners`
which runs 10 goroutines each performing 50 iterations of subscribe, dispatch, and unsubscribe
concurrently — designed to be run with `-race`.

### Concurrent Send + Close claimed to be untested

**Location:** `stdio.go:91-142, 192-225` — Send and Close race testing
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for Send racing with Close. This is
factually wrong. `stdio_test.go:1047` contains `TestStdioConcurrentSendAndClose` which launches
10 concurrent senders racing against a Close call, verifying no panics or races occur.

### Send write goroutine leak described as a new finding but covered by existing exception

**Location:** `stdio.go:117-119` — Send write goroutine
**Date:** 2026-03-01

**Reason:** The audit describes the Send write goroutine leaking on context cancellation as a new
Medium-severity bug. This is the exact same issue as the known exception "Write goroutine in Send
can leak on context cancellation" at `stdio.go:86-102`. The finding references different line numbers
but describes identical behavior — the write goroutine may outlive the cancelled context because
`io.Writer.Write` has no context support. The additional claim about "partial writes corrupting the
stream" is incorrect — `writeMessage` acquires `writeMu` and writes atomically (full JSON + newline),
so a concurrent write cannot interleave mid-message. Duplicate of existing exception.

### Notify TOCTOU race described as a new finding but covered by existing exceptions

**Location:** `stdio.go:146-151` — Notify closed check and write race
**Date:** 2026-03-01

**Reason:** The audit describes a TOCTOU race between the `t.closed` check and the subsequent write
goroutine in `Notify`. This is already covered by two known exceptions: "Notify goroutine can write
to writer after context cancellation" and "Notify may succeed even if the transport reader has just
stopped." Both describe the same window where `Close()` can set `closed = true` between the check
and the write. The behavior is benign — notifications are fire-and-forget by definition, and writing
to a closed pipe returns an error that propagates correctly. Duplicate of existing exceptions.

### McpToolCallResult.Content untyped slices described as a new finding but covered by existing exception

**Location:** `event_types.go:212` — McpToolCallResult.Content
**Date:** 2026-03-01

**Reason:** The audit describes `McpToolCallResult.Content` being `[]interface{}` as a code quality
issue. This is the exact same issue as the known exception "McpToolCallResult.Content and MCP metadata
fields use untyped interface{}" which explains the upstream spec defines these as open-schema fields
(`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping for a spec that
deliberately leaves the type open. Duplicate of existing exception.

### Resource and Tool untyped interface{} fields described as a new finding but covered by existing exception

**Location:** `mcp.go:26-51` — Resource and Tool type fields
**Date:** 2026-03-01

**Reason:** The audit describes multiple `interface{}` fields on Resource and Tool types as a code
quality issue. These fields (`Icons`, `Meta`, `Annotations`, `InputSchema`, `OutputSchema`) are all
covered by the known exception "McpToolCallResult.Content and MCP metadata fields use untyped
interface{}" which explicitly mentions `mcp.go` Resource/Tool metadata fields. The upstream spec
uses open schemas (`true`) for these fields. Duplicate of existing exception.

### DynamicToolCallParams.Arguments untyped interface{} described as a new finding but covered by existing exception

**Location:** `approval.go:753` — DynamicToolCallParams.Arguments
**Date:** 2026-03-01

**Reason:** The audit describes `DynamicToolCallParams.Arguments` being `interface{}` as a code
quality issue. This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which explains
the deliberate caller-convenience tradeoff. Duplicate of existing exception.

### readLoop error paths claimed to have no test coverage

**Location:** `stdio.go:275-325` — readLoop error handling
**Date:** 2026-03-01

**Reason:** The audit claims the readLoop error conditions have "no direct test coverage." This is
factually wrong. `stdio_test.go` contains: `TestStdioInvalidJSON` which injects malformed JSON lines
and verifies the transport stays alive and subsequent valid requests succeed; `TestStdioScannerBufferOverflow`
which sends a message exceeding `maxMessageSize` and verifies `ScanErr()` returns the buffer overflow
error; and `TestStdioHandleResponseUnmarshalError` which injects a response with a valid ID but
malformed body and verifies the pending caller receives a parse error response instead of timing out.
All three error paths the audit claims are untested have dedicated tests.

### Concurrent turn rejection claimed to be untested

**Location:** `conversation.go:173-178` — activeTurn exclusion logic
**Date:** 2026-03-01

**Reason:** The audit claims the `errTurnInProgress` concurrent-exclusion logic has no test.
This is factually wrong. `conversation_test.go` contains four dedicated concurrent turn rejection tests:
`TestConversationConcurrentTurnRejected` (line 507), `TestConversationConcurrentTurnStreamedRejected`
(line 651), `TestConversationConcurrentTurnVsTurnStreamedRejected` (line 697), and
`TestConversationConcurrentTurnStreamedVsTurnRejected` (line 746). These test all four combinations
of Turn vs TurnStreamed racing and assert the second call returns an error.

### Result() described as blocking forever on cancelled context

**Location:** `run_streamed.go:51-56` — Stream.Result() blocking semantics
**Date:** 2026-03-01

**Reason:** The audit itself acknowledges "it does close `s.done` (via `defer close(s.done)`), so
this actually works." The lifecycle goroutine at `run_streamed.go:121-123` always closes `s.done`
via defer, including on context cancellation — so `Result()` never blocks forever. The remaining
concern ("nil means the turn did not complete successfully" has no error return) is documented API
behavior at line 49-50: "Returns nil if the turn errored (the error was already surfaced through
the Events iterator)." This is an API design preference, not a bug.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper use interface{} instead of sealed interface

**Location:** `approval.go:179`, `approval.go:463` — Value fields
**Date:** 2026-03-01

**Reason:** This is already covered by the known exception "ReviewDecisionWrapper and
CommandExecutionApprovalDecisionWrapper use untyped interface{} for Value" which explains that
changing these to sealed interfaces would alter the public API surface, violating the spec
compliance rules. The custom UnmarshalJSON/MarshalJSON methods already enforce valid values
at runtime. Duplicate of existing exception.

### FuzzyFileSearch claimed to be missing from approval handler dispatch

**Location:** `client.go:248-294` — handleRequest approval dispatch
**Date:** 2026-03-01

**Reason:** The audit claims `fuzzyFileSearch` should be routed through `handleRequest` as a
server→client approval request. This is incorrect. `fuzzyFileSearch` is a **client→server**
request — it appears in `specs/ClientRequest.json` and is implemented as
`FuzzyFileSearchService.Search()` which calls `sendRequest` (fuzzy_search.go:53). It is correctly
absent from the server→client approval dispatch in `handleRequest`. The `request_coverage_test.go`
comment at line 189 ("server→client request (approval flow)") is misleading, but the code is
correct — `fuzzyFileSearch` is tested in `fuzzy_search_test.go` as a normal client→server method.

### Config key=value concatenation allows parsing ambiguity

**Location:** `process.go:94` — buildArgs config flag construction
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "Config flag values containing '=' are ambiguous on the
CLI" at `process.go:87`. Both describe the same issue: `--config k=v` concatenation is ambiguous
when keys or values contain `=`. The existing exception already documents why this is a CLI-side
parsing concern and not an SDK defect. The additional suggestion to validate keys is a feature
request, not a bug.

### cloneThreadItemWrapper panics on marshal/unmarshal failure

**Location:** `conversation.go:89-102` — cloneThreadItemWrapper panic behavior
**Date:** 2026-03-01

**Reason:** Covered by existing exception "cloneThreadItemWrapper uses JSON round-trip for deep copy"
which explicitly states: "The error path now panics (instead of silently returning the original),
ensuring the deep-copy guarantee is never silently broken." The panic is a deliberate design choice
already analyzed and accepted. The alternative (returning an error from `Thread()`) would change the
public API signature — a breaking change disproportionate to the risk, since marshal/unmarshal failure
on types with tested JSON methods indicates a logic bug, not a runtime condition to handle gracefully.

### Duplicate approval dispatch tests across two files

**Location:** `approval_test.go`, `dispatch_test.go` — approval handler tests
**Date:** 2026-03-01

**Reason:** The audit claims these tests are "identical" and "redundant." They test different
aspects. `TestApprovalHandlerDispatch` (approval_test.go) registers all 7 handlers and verifies
each handler **was called** (checking invocation). `TestKnownApprovalHandlerDispatch` (dispatch_test.go)
is table-driven, registers one handler per case, and verifies the **response has no error** (checking
dispatch correctness). `TestMissingApprovalHandler` tests with zero handlers set;
`TestMissingApprovalHandlerReturnsMethodNotFound` tests with a specific handler missing while others
are registered. These are complementary, not duplicative.

### ChatgptAuthTokensRefreshParams described as carrying auth tokens that need redaction tests

**Location:** `credential_redact_test.go` — missing test claim
**Date:** 2026-03-01

**Reason:** The audit claims `ChatgptAuthTokensRefreshParams` is "the request type that carries auth
tokens" and needs redaction tests. This is factually wrong. `ChatgptAuthTokensRefreshParams` contains
only `Reason` (a string enum) and `PreviousAccountID` (optional string) — neither is a credential.
The type carries the *reason* for a token refresh request (e.g. "expired"), not the actual tokens.
The *response* type (`ChatgptAuthTokensRefreshResponse`) carries the new `AccessToken` and already
has `MarshalJSON` redaction with full test coverage in `credential_redact_test.go`. There is nothing
to redact on the params type.

### Duplicate request ID check described as non-atomic but audit acknowledges no action needed

**Location:** `stdio.go:108-113` — duplicate ID check and registration
**Date:** 2026-03-01

**Reason:** The audit describes a theoretical window between unlock and write where a response
could match a different request with the same normalized ID. The audit itself concludes "No action
needed — the current design is correct for the expected usage pattern with sequential uint64 IDs."
A finding that states no action is needed and acknowledges correctness is not actionable. The
duplicate check and registration are both under `t.mu.Lock()`, and IDs are monotonically incrementing.

### Concurrent notification listener subscribe/unsubscribe claimed to be untested

**Location:** `client.go:214-235` — addNotificationListener concurrent safety
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for subscribe/unsubscribe racing with
dispatch. This is factually wrong. `listener_test.go:33-63` contains `TestConcurrentInternalListeners`
which runs 10 goroutines each performing 50 iterations of subscribe, dispatch, and unsubscribe
concurrently — designed to be run with `-race`.

### Stream.Events() single-use enforcement claimed to be untested

**Location:** `run_streamed.go:39-46` — Events() CompareAndSwap enforcement
**Date:** 2026-03-01

**Reason:** The audit claims there is no test verifying that calling `Events()` twice yields
`ErrStreamConsumed` on the second call. This is factually wrong. `run_streamed_test.go` contains
multiple tests for this: lines 890-901 call `Events()` twice and assert the second iterator yields
`ErrStreamConsumed`; lines 1007-1023 do the same in a different test scenario; and
`TestStreamEventsConcurrentConsumption` (line 1025) tests concurrent access with 10 goroutines
racing to consume, asserting exactly 1 winner and N-1 `ErrStreamConsumed` results.

### handleApproval pointer-to-result wireMarshaler dispatch described as fragile but works correctly

**Location:** `client.go:397` — marshalForWire(&result) call
**Date:** 2026-03-01

**Reason:** This is a duplicate of the existing exception "handleApproval marshalForWire pointer-to-result
described as a potential bug but works correctly." The audit itself concludes the code works correctly
today and "No immediate change needed." All existing approval response types implement `marshalWire`
on value receivers, which are callable on pointer receivers in Go. The concern about a future change
to pointer receivers is speculative — not an actionable finding.

### Conversation.Thread() deep-copy panics on failure described as a code quality issue

**Location:** `conversation.go:89-136` — cloneThreadItemWrapper, cloneSessionSourceWrapper, cloneThreadStatusWrapper
**Date:** 2026-03-01

**Reason:** Covered by existing exception "cloneThreadItemWrapper uses JSON round-trip for deep copy"
which explicitly states: "The error path now panics (instead of silently returning the original),
ensuring the deep-copy guarantee is never silently broken." The panic is a deliberate design choice
already analyzed and accepted.

### Conversation.Thread() claimed to not deep-copy TokenUsage or other top-level fields

**Location:** `conversation.go:51-83` — Thread() deep-copy
**Date:** 2026-03-01

**Reason:** The finding claims Thread() "does not deep-copy TokenUsage or other potential top-level fields."
`TokenUsage` does not exist on the `Thread` struct (thread.go:20-37). Every pointer and slice field on
Thread is deep-copied: `Name`, `AgentNickname`, `AgentRole`, `Path` (all `*string` — cloned via
`cloneStringPtr`), `GitInfo` (`*GitInfo` — field-by-field deep copy), `Source` (`SessionSourceWrapper` —
JSON round-trip clone), `Status` (`ThreadStatusWrapper` — JSON round-trip clone), `Turns` (`[]Turn` —
slice copy with per-item deep copy of Items and Error). The concern about "future fields" is speculative.

### ExecArgs values described as needing shell metacharacter validation

**Location:** `process.go:84-113` — buildArgs ExecArgs handling
**Date:** 2026-03-01

**Reason:** The finding itself acknowledges "`exec.Command` does not use a shell" and "This is safe."
The concern about the Codex CLI interpreting `--config "key=$(cmd)"` is speculative — `exec.Command`
passes each argument as a discrete `argv` element, so `$(cmd)` is a literal string, not a shell expansion.
The CLI's parsing of its own arguments is outside the SDK's responsibility. The finding concludes with
"The current `exec.Command` usage is safe against shell injection" — confirming no vulnerability exists.

### Multi-turn state accumulation claimed to be untested

**Location:** `conversation.go`, `conversation_test.go` — multi-turn testing
**Date:** 2026-03-01

**Reason:** Already in known exceptions. `TestConversationMultiTurn` (conversation_test.go:13-96)
executes two turns on the same Conversation, then asserts `len(thread.Turns) == 2` at line 93-94.
The multi-turn accumulation path is tested.

### Notification handler ordering described as new finding but already accepted as known risk

**Location:** `stdio.go:504-514` — handleNotification goroutine dispatch
**Date:** 2026-03-01

**Reason:** The finding itself states "Already documented in `audit/exceptions/risks.md` as accepted risk"
and "No change needed — already accepted." This is a duplicate of the known exception "Notification
handlers dispatched concurrently without ordering guarantees." The suggested fix (adding godoc) is a
documentation enhancement, not a code defect.

### Close does not wait for readLoop goroutine to exit described as a new bug

**Location:** `stdio.go:207-240` — Close() and readLoop interaction
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "StdioTransport.Close does not stop the
reader goroutine." The suggested fix (wait for `<-t.readerStopped` with a timeout) would deadlock
because the readLoop is blocked on `scanner.Scan()` which cannot be interrupted without closing the
underlying reader. The known exception documents that fixing this requires changing the public API
from `io.Reader` to `io.ReadCloser` — the same root cause and the same disproportionate fix.

### writeMessage goroutines can leak on context cancellation described as a new bug

**Location:** `stdio.go:130-146, stdio.go:160-181` — Send and Notify write goroutines
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Write goroutine in Send can leak on context
cancellation" at `stdio.go:86-102`. The finding covers both `Send` and `Notify`, but both have the
same root cause: `io.Writer.Write` has no context or deadline support, so there is no way to
interrupt a blocked write without closing the underlying writer. The known exception documents that
fixing this requires changing the public API to accept `io.WriteCloser`.

### writeMessage errors silently discarded in request/notification handlers described as a new finding

**Location:** `stdio.go:436, stdio.go:452, stdio.go:477, stdio.go:484` — writeMessage error discards
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes
but the issue is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest`
where there is no caller to return an error to. The known exception documents that surfacing these
errors requires new public API surface disproportionate to the severity.

### Transport silently drops unparseable JSON lines described as a new finding

**Location:** `stdio.go:307-309` — readLoop JSON unmarshal failure
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" at `stdio.go:250-253`. Both describe the same behavior: when readLoop
receives a line that fails JSON unmarshal, it silently continues. The known exception documents that
surfacing dropped-line counts requires new public API surface disproportionate to a Low severity
debugging-convenience finding.

### Clone functions panic on marshal failure described as a code quality issue

**Location:** `conversation.go:89-102, conversation.go:106-119, conversation.go:122-136` — clone panic behavior
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "cloneThreadItemWrapper uses JSON round-trip
for deep copy" which explicitly states: "The error path now panics (instead of silently returning
the original), ensuring the deep-copy guarantee is never silently broken." The panic is a deliberate
design choice already analyzed and accepted. The alternative (returning an error from `Thread()`)
would change the public API signature.

### Unbounded goroutine spawning per server request described as new finding

**Location:** `stdio.go:442, 530` — handleRequest and handleNotification goroutine dispatch
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Unbounded goroutine spawning for incoming
messages" which documents that adding a bounded worker pool requires architectural changes
disproportionate to the threat model (local subprocess over stdio).

### readLoop goroutine leak when child process does not exit described as new bug

**Location:** `stdio.go:36-37, 290` — readLoop and Close interaction
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "StdioTransport.Close does not stop the
reader goroutine" which documents that fixing this requires changing the public API from
`io.Reader` to `io.ReadCloser`, a breaking change for all callers.

### Invalid JSON from server silently skipped described as new finding

**Location:** `stdio.go:307-310` — readLoop JSON unmarshal failure path
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" which documents that surfacing dropped-line counts requires new public
API surface disproportionate to the severity.

### handleRequest writeMessage errors silently discarded described as new finding

**Location:** `stdio.go:437, 478, 485` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" which documents that surfacing write errors requires new public API
surface disproportionate to the severity.

### ensureInit holds mutex during blocking Initialize RPC described as new finding

**Location:** `process.go:232-246` — ensureInit mutex held across RPC
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "ensureInit holds mutex across RPC
round-trip, serializing concurrent callers" which documents that replacing the mutex with a
`sync.Once`-like done channel requires non-trivial concurrency redesign for a one-time startup
path.

### TestErrorCodeConstants described as tautological

**Location:** `jsonrpc_test.go:230-249` — table-driven test comparing constants to literals
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "TestErrorCodeConstants verifies constants
against their literal definitions" which explains the test serves as executable documentation
that the constants match the JSON-RPC 2.0 spec values.

### ScanErr() claimed to never be called in any test

**Location:** `stdio.go:244` — ScanErr public method
**Date:** 2026-03-01

**Reason:** This is factually wrong. `stdio_test.go:848-859` in `TestStdioScannerBufferOverflow`
polls `transport.ScanErr()` in a loop until the reader processes the oversized line, then verifies
the error is non-nil. The test at line 851 calls `transport.ScanErr()` and checks the result.

### Notify after Close claimed to be untested

**Location:** `stdio.go:160` — Notify after Close test coverage
**Date:** 2026-03-01

**Reason:** This is factually wrong. `stdio_test.go:470-478` tests Notify after Close: it calls
`transport.Close()`, then calls `transport.Notify()`, and asserts at line 477 that the error is
non-nil ("Notify after Close did not return error"). Additionally, `mock_transport_verify_test.go:346`
tests the same behavior on MockTransport.

### SessionSourceSubAgent round-trip serialization described as losing type discriminator

**Location:** `thread.go:263-277` — SessionSourceWrapper.MarshalJSON SubAgent case
**Date:** 2026-03-01

**Reason:** The finding claims `json.Marshal(v)` for `SessionSourceSubAgent` "may not produce the
correct wire format with type discriminators." This is incorrect. `SessionSourceSubAgent` has a
`json:"subAgent"` struct tag (thread.go:66), and `SubAgentSourceThreadSpawn` has a `json:"thread_spawn"`
struct tag (thread.go:89-95). Default `json.Marshal` produces `{"subAgent":{"thread_spawn":{...}}}`,
which matches the format expected by `UnmarshalJSON` (line 213 checks for key `"subAgent"`, line 243
checks for key `"thread_spawn"`). The round-trip is correct. This is also a duplicate of the known
exception "SessionSourceSubAgent relies on implicit marshaling for SubAgentSource variants."

### security_test.go described as testing documentation content instead of security behavior

**Location:** `security_test.go` — all tests
**Date:** 2026-03-01

**Reason:** These tests verify that SECURITY.md exists and contains required sections (reporting
guidance, security scope, dependency policy). They are documentation enforcement tests, not
security behavior tests. This is intentional — actual security behavior (credential redaction,
wire protocol safety) is tested in `credential_redact_test.go` and the transport tests.
The documentation tests ensure the security policy file stays complete and accurate as the
project evolves.

### readLoop silently drops malformed JSON lines described as a new finding

**Location:** `stdio.go:307` — readLoop JSON unmarshal failure path
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" at `stdio.go:250-253`. Both describe the same behavior: readLoop
silently continues on JSON parse failure with no observability. The suggested fix (adding an
`OnParseError` callback) is the same new-API-surface approach already discussed and rejected
as disproportionate in the existing exception.

### handleRequest goroutines use transport context described as a design concern

**Location:** `stdio.go:460` — handleRequest handler context
**Date:** 2026-03-01

**Reason:** The audit itself concludes "This is acceptable design but worth documenting" and
suggests no code change beyond documentation. `Close()` cancels `t.ctx` (line 216), so
context-aware handlers see the cancellation immediately. The concern about handlers that don't
respect context cancellation is a general Go programming concern, not specific to this code.
A finding whose own analysis concludes "acceptable design" is not a code defect.

### writeMessage errors silently discarded in handleRequest described as a new finding

**Location:** `stdio.go:437, 453, 478, 485` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes
but the issue is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest`
where there is no caller to return an error to. The suggested fix (routing through panicHandler or
error callback) is the same new-API-surface approach already discussed and rejected in the existing
exception.

### Stream.Events iterator doesn't drain on early break described as a new finding

**Location:** `run_streamed.go:136-142` — Events iterator early-break behavior
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Stream background goroutine blocks if
consumer stops iterating without cancelling context" which describes the identical behavior —
the lifecycle goroutine blocks on send when the consumer stops reading and the buffer fills.
The existing exception documents that context cancellation is the correct cleanup mechanism
and that adding a `done` channel would complicate the iterator contract.

### TurnStartParams.ApprovalPolicy uses interface type described as code quality issue

**Location:** `turn.go:26` — TurnStartParams.ApprovalPolicy field type
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Params structs use bare interface instead
of wrapper type for approval and sandbox policy fields" which covers all bare-interface policy
fields on params structs, including `TurnStartParams.ApprovalPolicy` at `turn.go:24`. The
exception explains that changing the field types to wrapper types would break callers who
construct params with the current signatures, and that the common case (string literal policies)
marshals correctly.

### ThreadStartParams.ApprovalPolicy uses interface type described as code quality issue

**Location:** `thread.go:659` — ThreadStartParams.ApprovalPolicy field type
**Date:** 2026-03-01

**Reason:** Same duplicate as above. The known exception "Params structs use bare interface instead
of wrapper type" at `thread.go:538` et al. covers this field. `ThreadStartParams.ApprovalPolicy`
is explicitly listed in the exception's location set.

### normalizeID loses precision for large float64 IDs described as a new finding

**Location:** `stdio.go:57-69` — normalizeID float64 precision
**Date:** 2026-03-01

**Reason:** Duplicate of the known exception "normalizeID relies on float64 precision for JSON
number IDs." Additionally, the finding describes the `float64(u) != v` fallthrough as a problem,
but this IS the precision guard — the code at lines 59-62 does `u := uint64(v)` then checks
`v == float64(u)`, falling through to `fmt.Sprintf("%v", v)` when precision is lost. The code
correctly handles the edge case the finding describes. The exception documents that values above
2^53 are not realistic for JSON-RPC IDs.

### Conversation.TurnStreamed concurrent call rejection claimed to be untested

**Location:** `conversation.go:238` — TurnStreamed activeTurn check
**Date:** 2026-03-01

**Reason:** Factually wrong. `conversation_test.go` contains four dedicated concurrent turn
rejection tests: `TestConversationConcurrentTurnRejected` (line 507),
`TestConversationConcurrentTurnStreamedRejected` (line 651),
`TestConversationConcurrentTurnVsTurnStreamedRejected` (line 697), and
`TestConversationConcurrentTurnStreamedVsTurnRejected` (line 746). These test all four
combinations of Turn vs TurnStreamed racing and assert `errTurnInProgress` is returned.

### Stream early-break cleanup behavior claimed to be untested

**Location:** `run_streamed.go:126` — lifecycle goroutine cleanup on early break
**Date:** 2026-03-01

**Reason:** Factually wrong. `run_streamed_test.go:507` contains `TestRunStreamedEarlyBreak`
which starts `RunStreamed`, reads 1 event, breaks out of the `Events()` loop, then verifies
`Result()` returns within 3 seconds (not hanging). This tests exactly the scenario the finding
describes — early break from the iterator followed by lifecycle goroutine cleanup.

### Approval handler error path claimed to only be tested for ChatGPT token refresh

**Location:** `approval.go:386` — handleApproval error paths
**Date:** 2026-03-01

**Reason:** Factually wrong. `stdio_test.go:729` (`TestStdioApprovalInvalidParamsReturnsErrorCode`)
tests the unmarshal-error path for `applyPatchApproval` through a real `StdioTransport`, sending
invalid JSON params and verifying the `-32602` error code. `stdio_test.go:776`
(`TestStdioApprovalHandlerErrorReturnsErrorCode`) tests the handler-error path for the same type.
Since `handleApproval` is a generic function, testing one approval type exercises the generic
unmarshal-error and handler-error code paths for all types.

### Stream channel buffer size described as magic number

**Location:** `run_streamed.go:18` — streamChannelBuffer constant
**Date:** 2026-03-01

**Reason:** The audit itself concludes "This is a named constant, not a magic number — no issue
here" and "Suggested fix: None needed." A finding that self-invalidates is not actionable.

### maxMessageSize described as magic number needing protocol documentation reference

**Location:** `stdio.go:294` — maxMessageSize constant
**Date:** 2026-03-01

**Reason:** Duplicate of the known exception "Scanner buffer sizes are named constants, not magic
numbers" which documents that `maxMessageSize` is a named constant with a descriptive comment.
The suggestion to reference protocol documentation is a documentation enhancement request for
a limit that is a defensive guess (not protocol-defined), not a code quality defect.

### readLoop recovery from oversized messages claimed to be untested

**Location:** `stdio.go:290` — readLoop scanner buffer overflow
**Date:** 2026-03-01

**Reason:** Factually wrong. `stdio_test.go:823` (`TestStdioScannerBufferOverflow`) sends a
message exceeding the 10MB `maxMessageSize`, verifies the reader stops, and asserts that
`ScanErr()` returns an error containing "token too long". The edge case the finding claims
is untested has a dedicated test.

### Notification listener registration race in executeStreamedTurn claimed but all listeners registered before RPC

**Location:** `turn_lifecycle.go:101-234` — executeStreamedTurn listener registration order
**Date:** 2026-03-01

**Reason:** The audit claims a "narrow window" where the `turnDone` channel listener (registered
"last") might not be wired when an early `turn/completed` arrives. This is factually wrong about
the ordering being a problem. All `streamListen` calls and `on(...)` registrations (lines 120-201)
happen sequentially **before** `Turn.Start` is called at line 203. The audit itself acknowledges
"the streamed path has the same registration-before-start pattern" and that "the risk is theoretical
in normal conditions." The pattern is identical to `executeTurn` which the audit considers correct.

### ExecArgs flag bypass via space-separated value form described as a gap but no actual bypass exists

**Location:** `process.go:93-105` — buildArgs flag rejection
**Date:** 2026-03-01

**Reason:** The audit describes the "real gap" as future CLI aliases or short flags (e.g. `-m` for
`--model`) bypassing the check. This is speculation about future CLI changes, not an existing bug.
The current code correctly rejects all current flag forms. The finding's own suggested fix is
"Add a comment documenting this limitation" — a documentation suggestion, not a code defect.
Typed safety flags are always appended after ExecArgs with last-wins semantics, so even a missed
flag form would be overridden.

### Silent discard of writeMessage errors described as new finding but covered by existing exception

**Location:** `stdio.go:457, 498, 505, 530` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "writeMessage errors silently discarded in handleRequest
goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes but the issue
is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest` where there
is no caller to return an error to.

### McpToolCallResult.Content uses []interface{} described as new finding but covered by existing exception

**Location:** `event_types.go:212` — McpToolCallResult.Content
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "McpToolCallResult.Content and MCP metadata fields use
untyped interface{}" which explains that the upstream spec defines these as open-schema fields
(`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping.

### DynamicToolCallParams.Arguments uses interface{} described as new finding but covered by existing exception

**Location:** `approval.go:754, thread_item.go:156, thread_item.go:180, turn.go:31` — interface{} fields
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "OutputSchema and DynamicToolCallParams.Arguments use bare
interface{} instead of json.RawMessage" which explains the deliberate caller-convenience tradeoff.

### Concurrent Turn exclusion claimed to lack real-timing test but test exists

**Location:** `conversation.go:200-235` — errTurnInProgress guard
**Date:** 2026-03-01

**Reason:** The audit claims the test is "only in a sequential setup." This is factually wrong.
`TestConversationConcurrentTurnRejected` (conversation_test.go:507) starts a turn in a goroutine,
waits 50ms for it to become active, then calls `Turn` from the main goroutine — this IS a concurrent
test with real timing. The mock hasn't responded yet when the second call happens, so the first turn
is genuinely active. All four concurrent combinations (Turn/Turn, TurnStreamed/TurnStreamed,
Turn/TurnStreamed, TurnStreamed/Turn) are tested at lines 507, 651, 697, and 746.

### Process.Close grace period and SIGKILL escalation claimed to be untested

**Location:** `process.go:197-228` — Process.Close shutdown sequence
**Date:** 2026-03-01

**Reason:** The audit claims "the test suite doesn't spawn a real subprocess" and "the isSignalError
helper is also untested." Both claims are factually wrong. `TestProcessCloseForceKill`
(process_test.go:472-512) spawns a real subprocess that traps SIGINT, calls `Close()`, and verifies
it completes within 10 seconds — exercising the SIGINT→grace period→SIGKILL path. `isSignalError`
has dedicated tests in `process_internal_test.go` covering nil error, non-ExitError, signal-killed
process, and normal exit cases.

### Approval handler wire-format fidelity for redacted types claimed to be untested

**Location:** `approval.go:913-930` — ChatgptAuthTokensRefreshResponse marshalWire
**Date:** 2026-03-01

**Reason:** The audit claims "no test verifies that marshalForWire is actually used in the approval
response path." This is factually wrong. `credential_redact_test.go:62-98`
(`TestCredentialTypesRedactWithAllFormatVerbs/ChatgptAuthTokensRefresh/wireProtocol`) registers an
`OnChatgptAuthTokensRefresh` handler returning a real token, simulates a server request through the
transport via `InjectServerRequest`, and verifies the response JSON contains the unredacted token
and does not contain `[REDACTED]`. This exercises the full `handleApproval` → `marshalForWire` path.

### Thread service tests for empty-response methods claimed to discard meaningful responses

**Location:** `thread_test.go:451,494,599` — TestThreadSetName, TestThreadArchive, TestThreadCompactStart
**Date:** 2026-03-01

**Reason:** The audit claims these tests "set up mock responses with thread data, call the service
method, then discard the response with `_ = response`" and that "the mock response data is set up
but never validated." This misreads the response types. `ThreadSetNameResponse`, `ThreadArchiveResponse`,
and `ThreadCompactStartResponse` are all empty structs (per spec), and their service methods pass `nil`
as the deserialization target to `sendRequest`. There is nothing to validate on the response — `_ = response`
is correct. The mock response data setup is superfluous boilerplate, but discarding an empty struct
is not a testing gap. (Note: `TestThreadUnsubscribe` is a separate case — `ThreadUnsubscribeResponse`
has a `Status` field that the test genuinely does not validate.)

### Approval handler dispatch claimed to be identically tested in two files

**Location:** `approval_test.go:243`, `dispatch_test.go:208` — approval handler dispatch tests
**Date:** 2026-03-01

**Reason:** The audit claims these tests are identical and one should be removed. They test different
aspects. `TestApprovalHandlerDispatch` (approval_test.go) registers all 7 handlers simultaneously
and verifies each handler **was called** via boolean flags (integration-style test of the full
handler set). `TestKnownApprovalHandlerDispatch` (dispatch_test.go) is table-driven, registers one
handler per test case in isolation, and verifies the **response has no error** (unit-style test of
individual dispatch correctness). These are complementary: one tests all handlers working together,
the other tests each handler in isolation.

### security_test.go described as testing markdown content instead of SDK security behavior

**Location:** `security_test.go:9-109` — all tests
**Date:** 2026-03-01

**Reason:** The audit says these tests "provide a false sense of security test coverage" and should
be deleted. They are documentation enforcement tests, not security behavior tests — and that is
intentional. They verify that SECURITY.md exists and contains required sections (reporting guidance,
security scope, dependency policy). Actual security behavior (credential redaction, wire protocol
safety) is tested in `credential_redact_test.go` and transport tests. The documentation tests ensure
the security policy file stays complete as the project evolves.
