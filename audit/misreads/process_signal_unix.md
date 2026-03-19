# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `process.go:442`, `process.go:450`, `process.go:470`, `process_signal_unix.go:20`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.
