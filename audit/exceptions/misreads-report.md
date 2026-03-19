# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Intentional transport shutdown already preserves the closed reader state

**Location:** `stdio.go:485`, `stdio.go:1767` — transport close and reader-stop paths

**Reason:** The report describes `stopAfterReaderTermination` overwriting the transport state with
an `io: read/write on closed pipe` error after an intentional `Close()`. That does not match the
current code. `closeWithFailure` marks the transport closed before closing the reader, and
`stopAfterReaderTermination` exits immediately when `t.closed` is already set, so it does not
replace the caller-facing closed-state error. The report's own reproductions also pass in this
checkout: `go test -run TestStdioConcurrentSendAndClose -count=100 ./...` and
`go test -race -run TestStdioConcurrentSendAndClose ./...` both succeed.

### The pre-push hook already runs the race-enabled test lane

**Location:** `scripts/hooks/pre-push.sh:7` — local pre-push verification

**Reason:** The current hook already includes `go test -race ./...` between the regular test lane
and `golangci-lint run ./...`. The README also documents that race test step. The finding is
stale against the checked-in hook and does not describe the current repository state.

### Streaming notification backpressure no longer blocks unrelated RPC responses

**Location:** `stdio.go:1161` — `enqueueStreamingNotification`

**Reason:** The current transport no longer does a blocking send from `readLoop`
into `streamingNotifQueue`. When the worker queue is full, streaming
notifications spill into the bounded `streamingBacklog` and a separate drainer
goroutine feeds workers from that backlog. That keeps inbound frame decoding and
response routing independent from handler throughput, so the transport does not
stall unrelated `Send` calls behind blocked streaming handlers.

### Transport starvation coverage for blocked streaming handlers already exists

**Location:** `stdio_internal_test.go:1196` — `TestStdioStreamingBackpressureDoesNotStarveUnrelatedResponses`

**Reason:** The current tree already has the integration regression the finding
claims is missing. The test blocks `item/agentMessage/delta` handlers, floods
streaming notifications past the worker queue, writes an unrelated response, and
asserts that the pending `Send` completes while the streaming handlers remain
blocked. The missing-coverage report is stale against the checked-in test suite.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `process.go:442`, `process.go:450`, `process.go:470`, `process_signal_unix.go:20`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### The subprocess suite already covers the SIGINT-to-exit-130 shutdown path

**Location:** `process_test.go:871` — `TestProcessCloseTreatsInterruptExitCode130AsExpected`

**Reason:** The current process integration tests already exercise the common
shell-handler path where `SIGINT` triggers cleanup and exits with status `130`.
The test initializes a fake child, traps `INT`, exits `130`, and asserts that
`Process.Close()` succeeds after the final notification is drained. The reported
testing gap is stale against the current suite.

### Streamed validation failures already populate the collector summary

**Location:** `run_streamed.go:250`, `run_streamed_test.go:154` — synchronous validation path and regression coverage

**Reason:** The current `runStreamedWithCollector` path already routes synchronous
validation failures through `newCollectedErrorStream`, which records the error in
the collector before returning the terminal error stream. The checked-in tests
cover both nil-context and empty-prompt collector cases and assert that
`Summary().NormalizedErrors` contains the validation error. The finding is stale
against the current implementation and test suite.

### CI already enforces module tidiness

**Location:** `.github/workflows/ci.yml:43`, `tooling_checks_test.go:45` — workflow gate and tooling test

**Reason:** The current GitHub Actions workflow already runs `go mod tidy -diff`
in the main CI job, and the tooling test still verifies the same rule under the
optional tooling lane. The finding no longer matches the checked-in workflow.
