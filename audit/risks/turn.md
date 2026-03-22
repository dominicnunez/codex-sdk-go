### TurnStartParams and TurnSteerParams reset struct on partial unmarshal failure

**Location:** `44-47`

**Reason:** The audit itself concludes "No code change needed." The reset pattern is correct —
it ensures no partial state leaks on error. The note that future modifications must include
the reset is accurate but not actionable as a code change.

### TurnStartParams and TurnSteerParams reset struct on partial unmarshal failure

**Location:** `116-119`

**Reason:** The audit itself concludes "No code change needed." The reset pattern is correct —
it ensures no partial state leaks on error. The note that future modifications must include
the reset is accurate but not actionable as a code change.
