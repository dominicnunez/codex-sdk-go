# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Stream collector retention for output deltas and latest plan text is already byte-bounded

**Location:** `stream_collector.go:12` — collector retention limits

**Reason:** The current collector already enforces byte budgets for retained command output deltas
and latest plan text. `streamCollectorOutputDeltaBytesLimit`,
`CommandExecutionLifecycle.DroppedOutputDeltaBytes`, `streamCollectorPlanTextBytesLimit`, and
`StreamSummary.DroppedLatestPlanTextBytes` are already present in the checked-in code, so the
reported unbounded-retention path does not exist in this checkout.
