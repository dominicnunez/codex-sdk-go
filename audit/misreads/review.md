# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

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
