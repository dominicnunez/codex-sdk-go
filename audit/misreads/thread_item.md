# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Thread item enum decoding already rejects unknown phase and status values

**Location:** `thread_item.go:46` — thread-item structs with enum-typed fields

**Reason:** The finding assumes `json.Unmarshal` accepts arbitrary strings for these fields, but the
enum types already implement validating `UnmarshalJSON` methods. `MessagePhase`,
`CommandExecutionStatus`, `PatchApplyStatus`, `McpToolCallStatus`, `DynamicToolCallStatus`,
`CollabAgentTool`, and `CollabAgentToolCallStatus` all reject off-spec values in
`event_types.go:60-210`, so decoding a thread item with an invalid enum fails instead of flowing
through as an impossible state.
