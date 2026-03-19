### TurnStartParams.OutputSchema uses untyped interface{} described as a code quality issue

**Location:** `turn.go:28` — TurnStartParams.OutputSchema
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which covers
this field explicitly. Duplicate of existing exception.

### TurnStartParams.ApprovalPolicy uses interface type described as code quality issue

**Location:** `turn.go:26` — TurnStartParams.ApprovalPolicy field type
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Params structs use bare interface instead
of wrapper type for approval and sandbox policy fields" which covers all bare-interface policy
fields on params structs, including `TurnStartParams.ApprovalPolicy` at `turn.go:24`. The
exception explains that changing the field types to wrapper types would break callers who
construct params with the current signatures, and that the common case (string literal policies)
marshals correctly.

### TurnStartParams SandboxPolicy marshal finding is a duplicate of existing design exception

**Location:** `turn.go:22-33` — TurnStartParams.SandboxPolicy field
**Date:** 2026-02-28

**Reason:** The audit flags that `TurnStartParams.SandboxPolicy` (bare interface `*SandboxPolicy`)
does not inject the `"type"` discriminator for struct variants like `SandboxPolicyWorkspaceWrite`.
The audit itself acknowledges "Already covered by the design exception 'Params structs use bare
interface instead of wrapper type.'" This is the exact same issue documented at
`audit/exceptions/design.md:63-77`, which covers all bare-interface policy fields on params
structs including `TurnStartParams.SandboxPolicy` at `turn.go:30`. Duplicate finding.
