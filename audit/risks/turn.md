### TurnStartParams and TurnSteerParams reset struct on partial unmarshal failure

**Location:** `turn.go:44-47`, `turn.go:116-119` — `*p = TurnStartParams{}` on error
**Date:** 2026-02-27

**Reason:** The audit itself concludes "No code change needed." The reset pattern is correct —
it ensures no partial state leaks on error. The note that future modifications must include
the reset is accurate but not actionable as a code change.
