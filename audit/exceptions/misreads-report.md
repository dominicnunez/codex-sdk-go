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
