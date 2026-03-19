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

### OutputSchema and DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage

**Location:** `turn.go:28`, `approval.go:739` — OutputSchema and Arguments fields
**Date:** 2026-02-27

**Reason:** The spec defines these as open-schema fields. Using `interface{}` is a deliberate
caller-convenience choice: SDK consumers construct these params and pass Go structs directly
(e.g. a map or typed struct) which `encoding/json` serializes correctly. Changing to
`json.RawMessage` would force every caller to pre-marshal their values, adding friction for
the primary use case. Other open-schema fields that use `json.RawMessage` (e.g. `Turn.Items`)
are on response types where the SDK receives raw JSON — different direction, different tradeoff.

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
