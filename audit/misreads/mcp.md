# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Resource and Tool untyped interface{} fields described as a new finding but covered by existing exception

**Location:** `mcp.go:26-51` — Resource and Tool type fields
**Date:** 2026-03-01

**Reason:** The audit describes multiple `interface{}` fields on Resource and Tool types as a code
quality issue. These fields (`Icons`, `Meta`, `Annotations`, `InputSchema`, `OutputSchema`) are all
covered by the known exception "McpToolCallResult.Content and MCP metadata fields use untyped
interface{}" which explicitly mentions `mcp.go` Resource/Tool metadata fields. The upstream spec
uses open schemas (`true`) for these fields. Duplicate of existing exception.

### MCP server auth status is already validated during response decoding

**Location:** `mcp.go:26` — `McpAuthStatus.UnmarshalJSON`

**Reason:** The report missed the existing enum validator. `McpAuthStatus` already rejects unknown
wire values via `unmarshalEnumString`, and `McpServerStatus.AuthStatus` uses that type directly, so
`McpServerStatus.UnmarshalJSON` does not accept arbitrary `authStatus` strings. The end-to-end
decode failure is exercised by `mcp_test.go:112-130`.
