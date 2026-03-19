# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### McpToolCallResult.Content and MCP metadata fields use untyped interface{}

**Location:** `event_types.go:197` — McpToolCallResult.Content, also `mcp.go` Resource/Tool metadata fields
**Date:** 2026-02-27

**Reason:** The upstream spec defines `McpToolCallResult.content` as `{"items": true, "type": "array"}`
— an array of any type, with no discriminated union or typed structure. Similarly, `Resource.Icons`,
`Resource.Meta`, `Tool.InputSchema`, etc. use open schemas (`true`) that accept arbitrary JSON.
Introducing typed content parts (e.g. `[]McpContentPart`) would be speculative — the spec deliberately
leaves these open for forward compatibility. Using `[]interface{}` (or `json.RawMessage`) is the
correct mapping for `"items": true`. Callers who need specific types can type-assert or re-unmarshal.
