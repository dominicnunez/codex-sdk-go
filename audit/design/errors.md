# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### RPCError.Is matches on error code only, ignoring message and data

**Location:** `errors.go:52-61` — RPCError.Is
**Date:** 2026-02-27

**Reason:** Code-only matching is the intentional semantic contract for RPCError. JSON-RPC error
codes define the error category (-32600, -32601, etc.), while messages are human-readable context
that may vary between server versions. Matching on code allows `errors.Is(err, sentinelRPCError)`
patterns where the sentinel carries the code but not a specific message. The nil-nil comparison
path (`e.err == nil && t.err == nil`) is unreachable since `NewRPCError` is never called with nil,
but the nil guard is a defensive correctness check, not dead logic worth removing.
