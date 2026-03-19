# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Stream summary takes a full snapshot on each call

**Location:** `stream_collector.go:151` — `Summary()`
**Date:** 2026-03-03

**Reason:** `Summary()` intentionally returns a deep copy so callers can mutate the returned
value without racing or corrupting collector state. Eliminating full-copy work would require
changing the API contract (for example immutable views or incremental subscriptions), which is
an architectural shift disproportionate to this low-severity finding. The current behavior is
correct by design and prioritized for data isolation and thread safety.
