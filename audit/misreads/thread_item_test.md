# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Negative tests for invalid enum decode paths already exist

**Location:** `thread_item_test.go:412` — related coverage also exists in `experimental_test.go:159`, `mcp_test.go:112`, and `external_agent_test.go:204`

**Reason:** The report says the suite never injects invalid thread-item enums, experimental stages,
MCP auth statuses, or external-agent migration item types through real decode paths. That is
factually wrong in the current checkout. `TestThreadItemRejectsInvalidEnums` covers the thread-item
paths, `TestExperimentalFeatureListRejectsInvalidStage` covers experimental stages,
`TestMcpListServerStatusRejectsInvalidAuthStatus` covers MCP auth status decoding, and
`TestExternalAgentConfigDetectRejectsInvalidItemType` covers external-agent item types.
