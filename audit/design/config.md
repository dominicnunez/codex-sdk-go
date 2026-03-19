# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### ReasoningSummaryWrapper accepts any string without validation

**Location:** `config.go:62-69` — ReasoningSummaryWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** Same forward-compatibility design as SessionSourceWrapper. The server may add new
reasoning summary modes. Rejecting unknown strings would break the SDK on server upgrades.
The type already constrains the value to be a string (rejecting non-string JSON), which is
the meaningful validation boundary.
