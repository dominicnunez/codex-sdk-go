### McpToolCallResult.Content and MCP metadata fields use untyped interface{}

**Location:** `197`

**Reason:** The upstream spec defines `McpToolCallResult.content` as `{"items": true, "type": "array"}`
— an array of any type, with no discriminated union or typed structure. Similarly, `Resource.Icons`,
`Resource.Meta`, `Tool.InputSchema`, etc. use open schemas (`true`) that accept arbitrary JSON.
Introducing typed content parts (e.g. `[]McpContentPart`) would be speculative — the spec deliberately
leaves these open for forward compatibility. Using `[]interface{}` (or `json.RawMessage`) is the
correct mapping for `"items": true`. Callers who need specific types can type-assert or re-unmarshal.

### McpToolCallResult.Content and MCP metadata fields use untyped interface{}

**Location:** `197`

**Reason:** The upstream spec defines `McpToolCallResult.content` as `{"items": true, "type": "array"}`
— an array of any type, with no discriminated union or typed structure. Similarly, `Resource.Icons`,
`Resource.Meta`, `Tool.InputSchema`, etc. use open schemas (`true`) that accept arbitrary JSON.
Introducing typed content parts (e.g. `[]McpContentPart`) would be speculative — the spec deliberately
leaves these open for forward compatibility. Using `[]interface{}` (or `json.RawMessage`) is the
correct mapping for `"items": true`. Callers who need specific types can type-assert or re-unmarshal.
