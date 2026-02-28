# Audit Exceptions

> Items validated as false positives or accepted as won't-fix.
> Managed by willie audit loop. Do not edit format manually.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

## False Positives

<!-- Findings where the audit misread the code or described behavior that doesn't occur -->

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

## Won't Fix

<!-- Real findings not worth fixing — architectural cost, external constraints, etc. -->

### StdioTransport.Close does not stop the reader goroutine

**Location:** `stdio.go:140-157` — Close() and readLoop()
**Date:** 2026-02-26

**Reason:** Fixing this requires changing the public API from `io.Reader` to `io.ReadCloser`,
which is a breaking change for all callers. The primary use case is `os.Stdin`, where the reader
goroutine terminates naturally with the process. In library contexts, callers control the underlying
reader and can close it themselves to unblock the scanner. The goroutine leak only matters for
long-running processes that create and discard many StdioTransport instances — a usage pattern
this SDK doesn't target.

### Write goroutine in Send can leak on context cancellation

**Location:** `stdio.go:86-102` — Send's write goroutine
**Date:** 2026-02-27

**Reason:** The goroutine runs `writeMessage`, which acquires `writeMu` and calls
`io.Writer.Write`. Go's `io.Writer` interface has no context or deadline support, so
there's no way to interrupt a blocked write without closing the underlying writer.
Fixing this requires either changing the public API to accept `io.WriteCloser` (breaking),
or wrapping all writes in a deadline-aware layer that introduces complexity disproportionate
to the risk. In practice, the writer is stdout to a child process — writes block only if the
process is hung, at which point the entire SDK session is stuck regardless.

### Notification handlers silently swallow unmarshal errors

**Location:** `account_notifications.go`, `turn_notifications.go`, and 25 other handlers
**Date:** 2026-02-27

**Reason:** Adding error surfacing requires either an `OnNotificationError` callback on Client
(new public API surface + all 27 handlers need plumbing) or changing handler signatures to return
errors (breaking change). The silent-drop behavior is consistent with JSON-RPC 2.0 notification
semantics where the server doesn't expect acknowledgment. Malformed notifications from the server
indicate a protocol-level bug that would manifest in other ways. The risk of silent data loss is
low relative to the API churn required to surface these errors.

### McpToolCallResult.Content and MCP metadata fields use untyped interface{}

**Location:** `event_types.go:197` — McpToolCallResult.Content, also `mcp.go` Resource/Tool metadata fields
**Date:** 2026-02-27

**Reason:** The upstream spec defines `McpToolCallResult.content` as `{"items": true, "type": "array"}`
— an array of any type, with no discriminated union or typed structure. Similarly, `Resource.Icons`,
`Resource.Meta`, `Tool.InputSchema`, etc. use open schemas (`true`) that accept arbitrary JSON.
Introducing typed content parts (e.g. `[]McpContentPart`) would be speculative — the spec deliberately
leaves these open for forward compatibility. Using `[]interface{}` (or `json.RawMessage`) is the
correct mapping for `"items": true`. Callers who need specific types can type-assert or re-unmarshal.

## Intentional Design Decisions

<!-- Findings that describe behavior which is correct by design -->

### Notification handler registration silently overwrites previous handlers

**Location:** `client.go:138-142` — OnNotification
**Date:** 2026-02-27

**Reason:** One handler per method is the intentional dispatch model. The Client routes each
notification method to exactly one handler — the same pattern used by `http.HandleFunc` and
similar Go standard library APIs. Supporting multiple handlers adds complexity (slice management,
ordering semantics, error aggregation) without clear benefit. The `OnNotification` doc comment
states "Only one handler can be registered per method; subsequent calls replace the previous
handler." This is documented behavior, not a bug.
