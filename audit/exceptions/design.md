# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### UserInput types rely on custom MarshalJSON for type discriminator injection

**Location:** `turn.go:156-246` — TextUserInput, ImageUserInput, LocalImageUserInput, SkillUserInput, MentionUserInput
**Date:** 2026-02-27

**Reason:** The MarshalJSON methods inject a `"type"` discriminator without storing it as a struct field.
This is the standard Go pattern for discriminated unions — the type tag is a serialization concern,
not domain state. The `UnmarshalUserInput` factory function handles deserialization dispatch.
Embedding these types without their custom marshaler would lose the discriminator, but this applies
to any Go type with custom marshaling and is not specific to this code. The pattern is used
consistently across all UserInput variants and matches other union types in the codebase.

### normalizeID relies on float64 precision for JSON number IDs

**Location:** `stdio.go:39-63` — normalizeID function
**Date:** 2026-02-27

**Reason:** JSON numbers unmarshal as float64 in Go's encoding/json, which
loses precision for integers above 2^53. Two distinct large IDs could
collide. However, JSON-RPC servers use small sequential integers or strings
for request IDs — values above 2^53 are not realistic. Fixing this would
require raw JSON token parsing to bypass float64, which is disproportionate
to the near-zero probability. The standard Go JSON number handling is the
correct default for this protocol.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper use untyped interface{} for Value

**Location:** `approval.go:154-156`, `approval.go:419-421` — Value fields
**Date:** 2026-02-27

**Reason:** These wrappers hold either a string or a specific struct, using
`interface{}` instead of a typed interface with marker methods. Changing
this would alter the public API surface of approval response types, which
is prohibited by the spec compliance rules (types map 1:1 to JSON-RPC
schemas). The custom UnmarshalJSON/MarshalJSON methods already enforce
valid values at runtime, and callers use type switches which are idiomatic
for this pattern. The compile-time safety gain does not justify the
breaking API change.

### Notification handler registration silently overwrites previous handlers

**Location:** `client.go:138-142` — OnNotification
**Date:** 2026-02-27

**Reason:** One handler per method is the intentional dispatch model. The Client routes each
notification method to exactly one handler — the same pattern used by `http.HandleFunc` and
similar Go standard library APIs. Supporting multiple handlers adds complexity (slice management,
ordering semantics, error aggregation) without clear benefit. The `OnNotification` doc comment
states "Only one handler can be registered per method; subsequent calls replace the previous
handler." This is documented behavior, not a bug.

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `thread.go:538`, `thread.go:642`, `thread.go:676`, `turn.go:24`, `turn.go:30` — ApprovalPolicy and SandboxPolicy fields
**Date:** 2026-02-27

**Reason:** The params types (`ThreadStartParams`, `ThreadResumeParams`, `ThreadForkParams`,
`TurnStartParams`) use `*AskForApproval` and `*SandboxPolicy` (bare interfaces) instead of
`*AskForApprovalWrapper` / `*SandboxPolicyWrapper`. The wrapper types handle JSON marshaling
correctly for structured variants (e.g. `ApprovalPolicyReject`), while the bare interface
relies on default marshaling which happens to work for string literals but would produce
incorrect output for struct-typed variants. Fixing this requires changing the public types of
these fields, which breaks callers who construct params with the current signatures. Since
these are public API types that map to spec schemas, the project's spec compliance rules
prohibit changing their signatures. The common case (string literal policies) marshals
correctly, and the structured variants are rarely used in client-to-server params.

### Go type names use Execpolicy casing instead of ExecPolicy

**Location:** `approval.go:165`, `approval.go:430` — ApprovedExecpolicyAmendmentDecision, AcceptWithExecpolicyAmendmentDecision
**Date:** 2026-02-27

**Reason:** The spec schema titles use this exact casing (`ApprovedExecpolicyAmendmentReviewDecision`,
`AcceptWithExecpolicyAmendmentCommandExecutionApprovalDecision`). The Go types mirror the spec
naming to maintain a clear 1:1 mapping. The project's spec compliance rules prohibit renaming
public types that map to spec schemas. While `ExecPolicy` would be more idiomatic Go, diverging
from the spec naming creates a maintenance burden and makes cross-referencing harder.

### RPCError.Is matches on error code only, ignoring message and data

**Location:** `errors.go:52-61` — RPCError.Is
**Date:** 2026-02-27

**Reason:** Code-only matching is the intentional semantic contract for RPCError. JSON-RPC error
codes define the error category (-32600, -32601, etc.), while messages are human-readable context
that may vary between server versions. Matching on code allows `errors.Is(err, sentinelRPCError)`
patterns where the sentinel carries the code but not a specific message. The nil-nil comparison
path (`e.err == nil && t.err == nil`) is unreachable since `NewRPCError` is never called with nil,
but the nil guard is a defensive correctness check, not dead logic worth removing.

### OutputSchema and DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage

**Location:** `turn.go:28`, `approval.go:739` — OutputSchema and Arguments fields
**Date:** 2026-02-27

**Reason:** The spec defines these as open-schema fields. Using `interface{}` is a deliberate
caller-convenience choice: SDK consumers construct these params and pass Go structs directly
(e.g. a map or typed struct) which `encoding/json` serializes correctly. Changing to
`json.RawMessage` would force every caller to pre-marshal their values, adding friction for
the primary use case. Other open-schema fields that use `json.RawMessage` (e.g. `Turn.Items`)
are on response types where the SDK receives raw JSON — different direction, different tradeoff.

### SessionSourceWrapper.MarshalJSON default case is an unreachable defensive guard

**Location:** `thread.go:234-245` — MarshalJSON switch on SessionSource concrete types
**Date:** 2026-02-27

**Reason:** The default error branch in `SessionSourceWrapper.MarshalJSON` is only reachable if a
caller manually assigns a type that satisfies `SessionSource` but isn't `sessionSourceLiteral` or
`SessionSourceSubAgent`. Both `UnmarshalJSON` and all SDK code paths only produce these two concrete
types, so the default branch is never triggered under normal usage. The `sessionSourceLiteral` case
handles unknown string values correctly (forward compatibility), so re-marshaling unknown sources
works. The default branch is a compile-time-unreachable defensive guard, not dead code worth removing.

### SessionSourceWrapper accepts any string without validation

**Location:** `thread.go:176-179` — SessionSourceWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** Forward compatibility by design. The server may introduce new session source
literals in newer protocol versions. Rejecting unknown strings would cause the SDK to break
on server upgrades. The same pattern is used by other union types in the codebase that accept
unknown variants (e.g. `UnknownAskForApproval`, `UnknownCommandAction`). Callers who need
to distinguish known from unknown values can check against the exported constants.

### ReasoningSummaryWrapper accepts any string without validation

**Location:** `config.go:62-69` — ReasoningSummaryWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** Same forward-compatibility design as SessionSourceWrapper. The server may add new
reasoning summary modes. Rejecting unknown strings would break the SDK on server upgrades.
The type already constrains the value to be a string (rejecting non-string JSON), which is
the meaningful validation boundary.

### Notify goroutine can write to writer after context cancellation

**Location:** `stdio.go:153-156` — Notify write goroutine
**Date:** 2026-02-27

**Reason:** Same root cause as the existing Send goroutine exception. When `Notify` returns
early via `ctx.Done()` or `readerStopped`, the goroutine running `writeMessage` continues and
may deliver a notification the caller believes was not sent. The write itself is safe (protected
by `writeMu`), and notifications are fire-and-forget by definition — a delivered notification
has no negative side effect. Fixing this requires `io.WriteCloser` (same API change discussed
in the Send exception), which is disproportionate to the severity.

### TurnStartParams custom UnmarshalJSON does not round-trip ApprovalPolicy and SandboxPolicy

**Location:** `turn.go:34-60` — TurnStartParams.UnmarshalJSON Alias delegation
**Date:** 2026-02-27

**Reason:** The `type Alias` trick delegates non-Input fields to default `encoding/json`
unmarshaling, which cannot populate bare interface fields (`*AskForApproval`, `*SandboxPolicy`)
without a registered custom unmarshaler. These fields are always nil after unmarshaling even
when present in JSON. This is the same root cause as the existing "Params structs use bare
interface instead of wrapper type" exception — changing the field types to wrapper types would
fix it but is prohibited by spec compliance rules (public API types map 1:1 to schemas). In
practice, `TurnStartParams` is constructed by SDK callers and marshaled for sending; the
unmarshal path is only used when the SDK receives these params in tests or echo scenarios,
not in normal client operation.

### LoginId field uses spec casing instead of Go acronym convention

**Location:** `account_notifications.go:25` — AccountLoginCompletedNotification.LoginId
**Date:** 2026-02-27

**Reason:** The spec schema (`AccountLoginCompletedNotification.json`) defines the wire field
as `"loginId"`. The Go field name `LoginId` mirrors the spec. Renaming to `LoginID` would be
more idiomatic Go, but the project's spec compliance rules prohibit renaming public fields
that map to spec schemas. The JSON struct tag preserves wire compatibility regardless.

### AuthorizationUrl field uses spec casing instead of Go acronym convention

**Location:** `mcp.go:83` — McpServerOauthLoginResponse.AuthorizationUrl
**Date:** 2026-02-27

**Reason:** The spec schema (`McpServerOauthLoginResponse.json`) defines the wire field as
`"authorizationUrl"`. The Go field name `AuthorizationUrl` mirrors the spec. Renaming to
`AuthorizationURL` would be more idiomatic Go, but the project's spec compliance rules
prohibit renaming public fields that map to spec schemas. The JSON struct tag preserves
wire compatibility regardless.

### ErrEmptyResult is a plain sentinel without method context for errors.As

**Location:** `client.go:329`, `client.go:362-364` — sendRequest and sendRequestRaw empty result handling
**Date:** 2026-02-27

**Reason:** Callers who want to know which method returned empty must parse the error string, since
`ErrEmptyResult` is a plain `errors.New` sentinel — not a struct with fields. Introducing an
`EmptyResultError` struct type with a `Method` field would add a new public type to the API for a
Low severity ergonomic improvement. The method name is already present in the `fmt.Errorf` wrapper
string for human-readable diagnostics, and `errors.Is(err, ErrEmptyResult)` works correctly for
programmatic detection. The typed error pattern used by `RPCError`/`TransportError`/`TimeoutError`
is justified by their higher severity and richer payloads.

### Credential redaction on sensitive types can be bypassed if embedded in another struct

**Location:** `approval.go:900-938` — ChatgptAuthTokensRefreshResponse, also `account.go:154-207` — ApiKeyLoginAccountParams, ChatgptAuthTokensLoginAccountParams
**Date:** 2026-02-28

**Reason:** These types implement MarshalJSON, String, GoString, and Format on pointer receivers
to redact credentials. If embedded in another struct that overrides these methods, redaction would
not apply. However, no struct in the codebase embeds these types, and the redaction works correctly
for all current usage patterns. This is defense-in-depth by design — the types are terminal
(never embedded), and the redaction methods cover all standard serialization paths. Adding
compile-time enforcement (e.g. a noCopy-style marker) would be speculative prevention for a
scenario that doesn't exist.

### Notify may succeed even if the transport reader has just stopped

**Location:** `stdio.go:135-156` — Notify method
**Date:** 2026-02-27

**Reason:** Same design tradeoff as the existing Send goroutine exception. The write goroutine
acquires `writeMu` and calls `io.Writer.Write`, which has no context or deadline support.
Between the `t.closed` check and the goroutine running, the reader could stop — but the
`select` at the wait phase handles this correctly. If the write completes before `readerStopped`
fires, `Notify` returns nil even though the transport is dying. This is benign: the notification
is fire-and-forget by definition, and the next Send call will fail with the transport error.
Fixing this requires the same `io.WriteCloser` API change discussed in the Send goroutine
exception, which is disproportionate to the severity.

### Zero-field union variants skip unmarshal while variants with fields do not

**Location:** `event_types.go:186-204` — PatchChangeKindWrapper.UnmarshalJSON, also `thread.go:287-311` — ThreadStatusWrapper.UnmarshalJSON
**Date:** 2026-02-28

**Reason:** The "add" and "delete" PatchChangeKind branches (and notLoaded/idle/systemError
ThreadStatus branches) construct zero-value structs directly without unmarshaling, while
"update" and "active" unmarshal to capture their fields. This asymmetry is intentional:
unmarshaling into a zero-field struct is wasted work that parses the entire JSON payload
only to discard every field. If the spec adds fields to these types, the struct definitions
will gain fields and the unmarshal call must be added — but that's a spec change that
requires code updates regardless. The current code is correct for the current spec and
avoids unnecessary work.

### RunResult.Thread reflects thread metadata at turn start, not post-turn live state

**Location:** `turn_lifecycle.go:87` — buildRunResult receives thread snapshot from turnLifecycleParams
**Date:** 2026-02-28

**Reason:** `RunResult.Thread` provides the thread metadata context (ID, config, cwd, model, etc.)
captured when the conversation was started or last completed. The `RunResult.Turn` field carries
the actual turn data from the server's `turn/completed` notification, which is always current.
Updating `RunResult.Thread` from the server response would require the `turn/completed`
notification to carry the full `Thread` object — which it doesn't (it only carries the `Turn`).
The current behavior is consistent: the Thread field is stable metadata, the Turn field is live
per-turn data. Callers who need post-turn thread state use `Conversation.Thread()`.

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
