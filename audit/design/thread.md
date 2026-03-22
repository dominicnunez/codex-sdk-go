### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `538`

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

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `642`

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

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `676`

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

### SessionSourceWrapper.MarshalJSON default case is an unreachable defensive guard

**Location:** `234-245`

**Reason:** The default error branch in `SessionSourceWrapper.MarshalJSON` is only reachable if a
caller manually assigns a type that satisfies `SessionSource` but isn't `sessionSourceLiteral` or
`SessionSourceSubAgent`. Both `UnmarshalJSON` and all SDK code paths only produce these two concrete
types, so the default branch is never triggered under normal usage. The `sessionSourceLiteral` case
handles unknown string values correctly (forward compatibility), so re-marshaling unknown sources
works. The default branch is a compile-time-unreachable defensive guard, not dead code worth removing.

### SessionSourceWrapper accepts any string without validation

**Location:** `176-179`

**Reason:** Forward compatibility by design. The server may introduce new session source
literals in newer protocol versions. Rejecting unknown strings would cause the SDK to break
on server upgrades. The same pattern is used by other union types in the codebase that accept
unknown variants (e.g. `UnknownAskForApproval`, `UnknownCommandAction`). Callers who need
to distinguish known from unknown values can check against the exported constants.

### Zero-field union variants skip unmarshal while variants with fields do not

**Location:** `287-311`

**Reason:** The "add" and "delete" PatchChangeKind branches (and notLoaded/idle/systemError
ThreadStatus branches) construct zero-value structs directly without unmarshaling, while
"update" and "active" unmarshal to capture their fields. This asymmetry is intentional:
unmarshaling into a zero-field struct is wasted work that parses the entire JSON payload
only to discard every field. If the spec adds fields to these types, the struct definitions
will gain fields and the unmarshal call must be added — but that's a spec change that
requires code updates regardless. The current code is correct for the current spec and
avoids unnecessary work.

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `538`

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

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `642`

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

### Params structs use bare interface instead of wrapper type for approval and sandbox policy fields

**Location:** `676`

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

### Zero-field union variants skip unmarshal while variants with fields do not

**Location:** `287-311`

**Reason:** The "add" and "delete" PatchChangeKind branches (and notLoaded/idle/systemError
ThreadStatus branches) construct zero-value structs directly without unmarshaling, while
"update" and "active" unmarshal to capture their fields. This asymmetry is intentional:
unmarshaling into a zero-field struct is wasted work that parses the entire JSON payload
only to discard every field. If the spec adds fields to these types, the struct definitions
will gain fields and the unmarshal call must be added — but that's a spec change that
requires code updates regardless. The current code is correct for the current spec and
avoids unnecessary work.
