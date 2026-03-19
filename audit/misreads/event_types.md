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

### McpToolCallResult uses untyped interface{} slices described as a code quality issue

**Location:** `event_types.go:212-213` — McpToolCallResult.Content and StructuredContent
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "McpToolCallResult.Content and MCP
metadata fields use untyped interface{}" which explains that the upstream spec defines these as
open-schema fields (`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping
for a spec that deliberately leaves the type open. Duplicate of existing exception.

### McpToolCallResult.Content untyped slices described as a new finding but covered by existing exception

**Location:** `event_types.go:212` — McpToolCallResult.Content
**Date:** 2026-03-01

**Reason:** The audit describes `McpToolCallResult.Content` being `[]interface{}` as a code quality
issue. This is the exact same issue as the known exception "McpToolCallResult.Content and MCP metadata
fields use untyped interface{}" which explains the upstream spec defines these as open-schema fields
(`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping for a spec that
deliberately leaves the type open. Duplicate of existing exception.

### McpToolCallResult.Content uses []interface{} described as new finding but covered by existing exception

**Location:** `event_types.go:212` — McpToolCallResult.Content
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "McpToolCallResult.Content and MCP metadata fields use
untyped interface{}" which explains that the upstream spec defines these as open-schema fields
(`"items": true, "type": "array"`). Using `[]interface{}` is the correct mapping.
