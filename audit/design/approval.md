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

### Go type names use Execpolicy casing instead of ExecPolicy

**Location:** `approval.go:165`, `approval.go:430` — ApprovedExecpolicyAmendmentDecision, AcceptWithExecpolicyAmendmentDecision
**Date:** 2026-02-27

**Reason:** The spec schema titles use this exact casing (`ApprovedExecpolicyAmendmentReviewDecision`,
`AcceptWithExecpolicyAmendmentCommandExecutionApprovalDecision`). The Go types mirror the spec
naming to maintain a clear 1:1 mapping. The project's spec compliance rules prohibit renaming
public types that map to spec schemas. While `ExecPolicy` would be more idiomatic Go, diverging
from the spec naming creates a maintenance burden and makes cross-referencing harder.

### OutputSchema and DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage

**Location:** `turn.go:28`, `approval.go:739` — OutputSchema and Arguments fields
**Date:** 2026-02-27

**Reason:** The spec defines these as open-schema fields. Using `interface{}` is a deliberate
caller-convenience choice: SDK consumers construct these params and pass Go structs directly
(e.g. a map or typed struct) which `encoding/json` serializes correctly. Changing to
`json.RawMessage` would force every caller to pre-marshal their values, adding friction for
the primary use case. Other open-schema fields that use `json.RawMessage` (e.g. `Turn.Items`)
are on response types where the SDK receives raw JSON — different direction, different tradeoff.
