### Thread item enum decoding already rejects unknown phase and status values

**Location:** `thread_item.go:46` — thread-item structs with enum-typed fields

**Reason:** The finding assumes `json.Unmarshal` accepts arbitrary strings for these fields, but the
enum types already implement validating `UnmarshalJSON` methods. `MessagePhase`,
`CommandExecutionStatus`, `PatchApplyStatus`, `McpToolCallStatus`, `DynamicToolCallStatus`,
`CollabAgentTool`, and `CollabAgentToolCallStatus` all reject off-spec values in
`event_types.go:60-210`, so decoding a thread item with an invalid enum fails instead of flowing
through as an impossible state.
