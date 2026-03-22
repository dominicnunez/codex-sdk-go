### Stream collector retention for output deltas and latest plan text is already byte-bounded

**Location:** `12`

**Reason:** The current collector already enforces byte budgets for retained command output deltas
and latest plan text. `streamCollectorOutputDeltaBytesLimit`,
`CommandExecutionLifecycle.DroppedOutputDeltaBytes`, `streamCollectorPlanTextBytesLimit`, and
`StreamSummary.DroppedLatestPlanTextBytes` are already present in the checked-in code, so the
reported unbounded-retention path does not exist in this checkout.
