### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `20`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `20`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.
