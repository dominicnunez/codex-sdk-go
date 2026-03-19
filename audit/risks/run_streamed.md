### Thread ID filtering in notification listeners lacks a cross-thread contamination test

**Location:** `turn_lifecycle.go:34-38`, `run_streamed.go:87-91` — threadID filter
**Date:** 2026-02-28

**Reason:** Testing cross-thread contamination requires running two concurrent turns on different
threads through the full turn lifecycle, which needs mock infrastructure for concurrent thread/turn
start responses with different thread IDs and interleaved notifications. The current mock transport
supports one response per method, making this test require significant test infrastructure changes.
The filter logic itself is trivial (`carrier.ThreadID != p.threadID`) and exercised in every
existing turn test — only the negative path (mismatched IDs) lacks coverage.

### Notification listeners double-unmarshal threadIDCarrier for thread filtering

**Location:** `turn_lifecycle.go:35-36,49-50`, `run_streamed.go:97-101` — threadIDCarrier pre-parse
**Date:** 2026-03-01

**Reason:** Every notification listener in the turn lifecycle first unmarshals a `threadIDCarrier`
to check the threadID, then unmarshals the full notification struct. This compounds with the
existing readLoop double-parse (each notification is parsed 4 times total). Fixing this requires
restructuring all notification listeners to unmarshal the full typed struct first, then check
the threadID field from the result — which changes the filter-then-parse pattern used consistently
across all listeners. The overhead is negligible for the small JSON-RPC payloads that dominate
notification traffic. Same risk profile as the readLoop double-parse exception above.
