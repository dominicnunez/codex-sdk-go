# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### SessionSourceSubAgent relies on implicit marshaling for SubAgentSource variants

**Location:** `thread.go:231-243` — SessionSourceWrapper.MarshalJSON
**Date:** 2026-02-27

**Reason:** The marshal path for `SessionSourceSubAgent` delegates to default `json.Marshal`,
while the unmarshal path uses explicit dispatch. The audit flags this asymmetry as fragile,
but all current `SubAgentSource` variants marshal correctly via struct tags. Adding an explicit
`MarshalJSON` to mirror the unmarshal dispatch adds code without fixing any bug. If a new
variant is added, the unmarshal dispatch already requires updating — the marshal side fails
visibly (wrong output) rather than silently, which is an adequate safety net.
