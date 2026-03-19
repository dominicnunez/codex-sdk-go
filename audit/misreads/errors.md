# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### RPCError.Is nil-nil path flagged as unreachable dead code

**Location:** `errors.go:61-70` — RPCError.Is nil guard
**Date:** 2026-03-01

**Reason:** This is covered by the existing exception for RPCError.Is which states: "The nil-nil
comparison path is unreachable since `NewRPCError` is never called with nil, but the nil guard is
a defensive correctness check, not dead logic worth removing." Defensive nil checks in `Is()`
implementations are standard Go practice — they prevent panics if the type is ever constructed
outside the canonical constructor.
