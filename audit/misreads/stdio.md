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

### Streaming notification backpressure no longer blocks unrelated RPC responses

**Location:** `stdio.go:1161` — `enqueueStreamingNotification`

**Reason:** The current transport no longer does a blocking send from `readLoop`
into `streamingNotifQueue`. When the worker queue is full, streaming
notifications spill into the bounded `streamingBacklog` and a separate drainer
goroutine feeds workers from that backlog. That keeps inbound frame decoding and
response routing independent from handler throughput, so the transport does not
stall unrelated `Send` calls behind blocked streaming handlers.


# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Reason:** Explanation (can be multiple lines)

### Malformed response accounting already increments when the pending request can be identified

**Location:** `stdio.go:1486` — `handleMalformedResponse`

**Reason:** The current transport increments `malformedCount` before it tries to parse or
normalize the response ID, so malformed server responses are counted even when the transport can
still attribute the parse error back to a pending request. The stale report line predates this
behavior, and the transport tests now assert both the parse-error delivery and the counter bump.


# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Intentional transport shutdown leaks internal pipe errors to callers

**Location:** `stdio.go:485` — `closeWithFailure` and `stdio.go:1767` — `stopAfterReaderTermination`

**Reason:** This behavior does not occur in the current transport. `closeWithFailure` sets
`t.closed = true` under the mutex before it closes the reader. When the read loop wakes up,
`stopAfterReaderTermination` checks `t.closed` at `stdio.go:1769-1771` and returns immediately,
so it never overwrites `scanErr` with the pipe-close error from an intentional shutdown. The
reported reproductions are also stale in this checkout: `go test -run TestStdioConcurrentSendAndClose -count=50 ./...`,
`go test -race ./...`, and `go test -coverprofile=/tmp/c.out ./...` all pass.

### Close cannot race with handleResponse to double-send on pending channel

**Location:** `stdio.go:197-213` — Close() pending request cleanup loop
**Date:** 2026-02-27

**Reason:** The audit claims `Close()` and `handleResponse` can both send into the same `pending.ch`,
causing a goroutine leak when `handleResponse`'s unconditional send blocks on a full buffer.
This cannot happen. `Close()` sets `t.closed = true` at line 191 under the mutex before
iterating `pendingReqs`. `handleResponse` at line 321 checks `t.closed` under the same mutex
and returns immediately if true. So after `Close()` runs, no `handleResponse` call will ever
proceed to delete an entry or send on a channel. In the reverse ordering — `handleResponse`
acquires the lock first, deletes the entry, releases the lock, then sends — `Close()` will not
find that entry in the map, so it never sends into the same channel. The two senders are
mutually exclusive by the `t.closed` flag under `t.mu`.

### Scanner buffer sizes are named constants, not magic numbers

**Location:** `stdio.go:226-227` — readLoop buffer constants
**Date:** 2026-02-27

**Reason:** The audit labels `initialBufferSize` and `maxMessageSize` as "magic numbers" but
the code defines them as named constants with descriptive comments (`// 64KB`, `// 10MB —
file diffs and base64 payloads exceed the default`). They are appropriately scoped to the
function that uses them. The actual concern — that callers can't tune them without modifying
source — is a feature request for configurability, not a code quality defect.

### normalizeID already has a precision guard for large float64 values

**Location:** `stdio.go:48-51` — normalizeID float64-to-uint64 cast
**Date:** 2026-03-01

**Reason:** The audit claims `normalizeID` casts `float64` to `uint64` "without checking whether
the conversion loses precision for integers above 2^53." This is factually wrong. The code at
lines 48-51 does: `u := uint64(v)` then `if v == float64(u)` — this round-trip check is exactly
the precision guard the audit suggests adding. For values above 2^53 where the float64 cannot
represent the integer exactly, `v == float64(u)` will be false, and the code falls through to
`fmt.Sprintf("%v", v)`. The suggested fix ("only use the integer fast-path when
`v == float64(uint64(v))` is exact") is already implemented.

### Internal writes do not deadlock when readerStopped is already closed

**Location:** `stdio.go:376-397` — enqueueWrite `readerStopped` handling
**Date:** 2026-03-03

**Reason:** The finding claims `enqueueWrite(..., watchReaderStop=false)` can take the
`readerStopped` branch before enqueueing and then block forever waiting on `env.done`.
That behavior does not occur. In the second `select`, `readerStopped` is selected again and,
when `watchReaderStop` is false, the function returns `nil` immediately at line 397 instead
of waiting on `env.done`. No deadlock is possible on this path.

### Close and handleResponse race claimed but audit concludes code is safe

**Location:** `stdio.go:201-216, stdio.go:371-378` — Close() and handleResponse() interaction
**Date:** 2026-03-01

**Reason:** The audit's own analysis concludes "This is actually safe" and the suggested fix is
"Add a comment explaining why the `select/default` is safe." A finding that concludes the code
is correct and only needs a comment is not a code defect. The invariants are already documented
in the code: the comment at line 378 states "safe: buffer 1, only one sender claims via delete"
and the Close() comments at lines 198-200 and 210-211 explain the defensive pattern.

### StdioTransport claimed to have no pipe-based integration tests

**Location:** `stdio.go` — StdioTransport test coverage
**Date:** 2026-03-01

**Reason:** The audit claims "There are no tests that exercise the actual readLoop, handleResponse,
handleRequest, and handleNotification codepaths with real pipe-based I/O." This is factually wrong.
`stdio_test.go` contains extensive pipe-based integration tests using `io.Pipe()` → `NewStdioTransport`:
`TestStdioNewlineDelimitedJSON` (pipe I/O with Send/response), `TestStdioConcurrentRequestDispatch`
(server→client requests via pipe), `TestStdioResponseRequestIDMatching` (concurrent sends with pipe),
`TestStdioNotificationDispatch` (server→client notifications via pipe), `TestStdioMixedMessageTypes`
(concurrent requests/responses/notifications), `TestStdioInvalidJSON` (malformed JSON recovery),
`TestStdioContextCancellation`, `TestStdioRequestHandlerPanicRecovery`, `TestStdioScannerBufferOverflow`
(10MB+ message), `TestStdioHandleResponseUnmarshalError`, and `TestStdioConcurrentSendAndClose`.

### normalizeID uint64 overflow described as a new defect

**Location:** `stdio.go:47-51` — normalizeID float64 to uint64 cast
**Date:** 2026-03-01

**Reason:** The audit claims float64 values near `math.MaxUint64` produce undefined behavior in the
uint64 cast. That does not match the current code. `normalizeID` already uses a round-trip guard:
it casts to `uint64`, then only takes the integer fast path when `v == float64(u)`. Values that
cannot be represented exactly fall through to the generic string formatting path instead of being
silently truncated. JSON-RPC IDs near `math.MaxUint64` are also not realistic protocol inputs.

### Send pending request context cancellation described as fragile but audit concludes no bug

**Location:** `stdio.go:91-142` — Send() pending request lifecycle
**Date:** 2026-03-01

**Reason:** The audit's own analysis concludes "No actual bug" and "No code change required. This
is a documentation-level observation." The deferred `delete` is idempotent — if `handleResponse`
already claimed and deleted the entry, the defer is a no-op (deleting a key that no longer exists
in the map). The audit acknowledges the pattern is safe and only speculates about fragility "if
`handleResponse` ever changes to not delete." A finding that explicitly states no bug exists and
proposes no code change is not an actionable finding.

### Close does not wait for readLoop to finish described as a separate bug

**Location:** `stdio.go:192-225` — Close() and readLoop interaction
**Date:** 2026-03-01

**Reason:** The report assumes the transport cannot interrupt the read loop. That is no longer true
in the current code: `NewStdioTransport` requires an `io.ReadCloser`, stores it as `readerCloser`,
and `closeWithFailure` closes that reader during `Close()`. Waiting for `<-t.readerStopped` is not
the missing remediation here, and the claimed "unstoppable reader" bug does not describe the
transport API that actually exists in this repo.

### Write goroutine leak on context cancellation described as a new finding

**Location:** `stdio.go:117-119` — Send write goroutine
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "Write goroutine in Send can leak
on context cancellation" at `stdio.go:86-102`. The finding references different line numbers but
describes identical behavior — the write goroutine may outlive the cancelled context because
`io.Writer.Write` has no context support. Duplicate of existing exception.

### handleApproval error swallowed into generic message described as a new finding

**Location:** `stdio.go:432-451` — handleRequest error translation
**Date:** 2026-03-01

**Reason:** This is the exact same issue as the known exception "Handler errors in handleApproval
are invisible to SDK consumers" at `client.go:273-274`. Both describe the same behavior — handler
errors are replaced with a generic "internal handler error" response on the wire. Duplicate of
existing exception.

### Concurrent Send + Close claimed to be untested

**Location:** `stdio.go:91-142, 192-225` — Send and Close race testing
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for Send racing with Close. This is
factually wrong. `stdio_test.go:1047` contains `TestStdioConcurrentSendAndClose` which launches
10 concurrent senders racing against a Close call, verifying no panics or races occur.

### Send write goroutine leak described as a new finding but covered by existing exception

**Location:** `stdio.go:117-119` — Send write goroutine
**Date:** 2026-03-01

**Reason:** The audit describes the Send write goroutine leaking on context cancellation as a new
Medium-severity bug. This is the exact same issue as the known exception "Write goroutine in Send
can leak on context cancellation" at `stdio.go:86-102`. The finding references different line numbers
but describes identical behavior — the write goroutine may outlive the cancelled context because
`io.Writer.Write` has no context support. The additional claim about "partial writes corrupting the
stream" is incorrect — `writeMessage` acquires `writeMu` and writes atomically (full JSON + newline),
so a concurrent write cannot interleave mid-message. Duplicate of existing exception.

### Notify TOCTOU race described as a new finding but covered by existing exceptions

**Location:** `stdio.go:146-151` — Notify closed check and write race
**Date:** 2026-03-01

**Reason:** The audit describes a TOCTOU race between the `t.closed` check and the subsequent write
goroutine in `Notify`. This is already covered by two known exceptions: "Notify goroutine can write
to writer after context cancellation" and "Notify may succeed even if the transport reader has just
stopped." Both describe the same window where `Close()` can set `closed = true` between the check
and the write. The behavior is benign — notifications are fire-and-forget by definition, and writing
to a closed pipe returns an error that propagates correctly. Duplicate of existing exceptions.

### readLoop error paths claimed to have no test coverage

**Location:** `stdio.go:275-325` — readLoop error handling
**Date:** 2026-03-01

**Reason:** The audit claims the readLoop error conditions have "no direct test coverage." This is
factually wrong. `stdio_test.go` contains: `TestStdioInvalidJSON` which injects malformed JSON lines
and verifies the transport stays alive and subsequent valid requests succeed; `TestStdioScannerBufferOverflow`
which sends a message exceeding `maxMessageSize` and verifies `ScanErr()` returns the buffer overflow
error; and `TestStdioHandleResponseUnmarshalError` which injects a response with a valid ID but
malformed body and verifies the pending caller receives a parse error response instead of timing out.
All three error paths the audit claims are untested have dedicated tests.

### Duplicate request ID check described as non-atomic but audit acknowledges no action needed

**Location:** `stdio.go:108-113` — duplicate ID check and registration
**Date:** 2026-03-01

**Reason:** The audit describes a theoretical window between unlock and write where a response
could match a different request with the same normalized ID. The audit itself concludes "No action
needed — the current design is correct for the expected usage pattern with sequential uint64 IDs."
A finding that states no action is needed and acknowledges correctness is not actionable. The
duplicate check and registration are both under `t.mu.Lock()`, and IDs are monotonically incrementing.

### Notification handler ordering described as new finding but already accepted as known risk

**Location:** `stdio.go:504-514` — handleNotification goroutine dispatch
**Date:** 2026-03-01

**Reason:** The finding itself states "Already documented in `audit/exceptions/risks.md` as accepted risk"
and "No change needed — already accepted." This is a duplicate of the known exception "Notification
handlers dispatched concurrently without ordering guarantees." The suggested fix (adding godoc) is a
documentation enhancement, not a code defect.

### Close does not wait for readLoop goroutine to exit described as a new bug

**Location:** `stdio.go:207-240` — Close() and readLoop interaction
**Date:** 2026-03-01

**Reason:** This finding is stale against the current transport implementation. `NewStdioTransport`
already requires an `io.ReadCloser`, and `closeWithFailure` closes that reader on shutdown. The
report's rationale depends on an older `io.Reader`-only API that no longer exists, so the described
shutdown deadlock is not an accurate reading of the current code.

### writeMessage goroutines can leak on context cancellation described as a new bug

**Location:** `stdio.go:130-146, stdio.go:160-181` — Send and Notify write goroutines
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Write goroutine in Send can leak on context
cancellation" at `stdio.go:86-102`. The finding covers both `Send` and `Notify`, but both have the
same root cause: `io.Writer.Write` has no context or deadline support, so there is no way to
interrupt a blocked write without closing the underlying writer. The known exception documents that
fixing this requires changing the public API to accept `io.WriteCloser`.

### writeMessage errors silently discarded in request/notification handlers described as a new finding

**Location:** `stdio.go:436, stdio.go:452, stdio.go:477, stdio.go:484` — writeMessage error discards
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes
but the issue is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest`
where there is no caller to return an error to. The known exception documents that surfacing these
errors requires new public API surface disproportionate to the severity.

### Transport silently drops unparseable JSON lines described as a new finding

**Location:** `stdio.go:307-309` — readLoop JSON unmarshal failure
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" at `stdio.go:250-253`. Both describe the same behavior: when readLoop
receives a line that fails JSON unmarshal, it silently continues. The known exception documents that
surfacing dropped-line counts requires new public API surface disproportionate to a Low severity
debugging-convenience finding.

### Unbounded goroutine spawning per server request described as new finding

**Location:** `stdio.go:442, 530` — handleRequest and handleNotification goroutine dispatch
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "Unbounded goroutine spawning for incoming
messages" which documents that adding a bounded worker pool requires architectural changes
disproportionate to the threat model (local subprocess over stdio).

### readLoop goroutine leak when child process does not exit described as new bug

**Location:** `stdio.go:36-37, 290` — readLoop and Close interaction
**Date:** 2026-03-01

**Reason:** This claim no longer matches the code. The transport constructor already requires an
`io.ReadCloser`, keeps it in `readerCloser`, and closes it during shutdown. The older explanation
about needing a breaking API change from `io.Reader` to `io.ReadCloser` is obsolete.

### Invalid JSON from server silently skipped described as new finding

**Location:** `stdio.go:307-310` — readLoop JSON unmarshal failure path
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" which documents that surfacing dropped-line counts requires new public
API surface disproportionate to the severity.

### handleRequest writeMessage errors silently discarded described as new finding

**Location:** `stdio.go:437, 478, 485` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" which documents that surfacing write errors requires new public API
surface disproportionate to the severity.

### ScanErr() claimed to never be called in any test

**Location:** `stdio.go:244` — ScanErr public method
**Date:** 2026-03-01

**Reason:** This is factually wrong. `stdio_test.go:848-859` in `TestStdioScannerBufferOverflow`
polls `transport.ScanErr()` in a loop until the reader processes the oversized line, then verifies
the error is non-nil. The test at line 851 calls `transport.ScanErr()` and checks the result.

### Notify after Close claimed to be untested

**Location:** `stdio.go:160` — Notify after Close test coverage
**Date:** 2026-03-01

**Reason:** This is factually wrong. `stdio_test.go:470-478` tests Notify after Close: it calls
`transport.Close()`, then calls `transport.Notify()`, and asserts at line 477 that the error is
non-nil ("Notify after Close did not return error"). Additionally, `mock_transport_verify_test.go:346`
tests the same behavior on MockTransport.

### readLoop silently drops malformed JSON lines described as a new finding

**Location:** `stdio.go:307` — readLoop JSON unmarshal failure path
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "readLoop silently skips unparseable JSON
lines with no diagnostic" at `stdio.go:250-253`. Both describe the same behavior: readLoop
silently continues on JSON parse failure with no observability. The suggested fix (adding an
`OnParseError` callback) is the same new-API-surface approach already discussed and rejected
as disproportionate in the existing exception.

### handleRequest goroutines use transport context described as a design concern

**Location:** `stdio.go:460` — handleRequest handler context
**Date:** 2026-03-01

**Reason:** The audit itself concludes "This is acceptable design but worth documenting" and
suggests no code change beyond documentation. `Close()` cancels `t.ctx` (line 216), so
context-aware handlers see the cancellation immediately. The concern about handlers that don't
respect context cancellation is a general Go programming concern, not specific to this code.
A finding whose own analysis concludes "acceptable design" is not a code defect.

### writeMessage errors silently discarded in handleRequest described as a new finding

**Location:** `stdio.go:437, 453, 478, 485` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "writeMessage errors silently discarded in
handleRequest goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes
but the issue is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest`
where there is no caller to return an error to. The suggested fix (routing through panicHandler or
error callback) is the same new-API-surface approach already discussed and rejected in the existing
exception.

### normalizeID large-float precision concern described as a new defect

**Location:** `stdio.go:57-69` — normalizeID float64 precision
**Date:** 2026-03-01

**Reason:** The finding describes the `float64(u) != v` fallthrough as a bug, but that branch is
the guard against precision loss. `normalizeID` only uses the integer path when the float64 value
round-trips exactly through `uint64`; otherwise it falls back to `fmt.Sprintf("%v", v)`. The code
already behaves the way the finding recommends. Values above the exact float64 integer range are
also not realistic JSON-RPC request IDs in this SDK.

### maxMessageSize described as magic number needing protocol documentation reference

**Location:** `stdio.go:294` — maxMessageSize constant
**Date:** 2026-03-01

**Reason:** Duplicate of the known exception "Scanner buffer sizes are named constants, not magic
numbers" which documents that `maxMessageSize` is a named constant with a descriptive comment.
The suggestion to reference protocol documentation is a documentation enhancement request for
a limit that is a defensive guess (not protocol-defined), not a code quality defect.

### readLoop recovery from oversized messages claimed to be untested

**Location:** `stdio.go:290` — readLoop scanner buffer overflow
**Date:** 2026-03-01

**Reason:** Factually wrong. `stdio_test.go:823` (`TestStdioScannerBufferOverflow`) sends a
message exceeding the 10MB `maxMessageSize`, verifies the reader stops, and asserts that
`ScanErr()` returns an error containing "token too long". The edge case the finding claims
is untested has a dedicated test.

### Silent discard of writeMessage errors described as new finding but covered by existing exception

**Location:** `stdio.go:457, 498, 505, 530` — writeMessage error discards in handleRequest
**Date:** 2026-03-01

**Reason:** Duplicate of existing exception "writeMessage errors silently discarded in handleRequest
goroutine" at `stdio.go:334, 356, 363`. The line numbers differ due to code changes but the issue
is identical: `_ = t.writeMessage(...)` calls in goroutines spawned by `handleRequest` where there
is no caller to return an error to.

### Oversized response recovery does not depend on top-level id appearing early in the frame

**Location:** `stdio.go:1520` — oversized frame parsing

**Reason:** The current oversized-frame path does not rely on a retained prefix. It
continues scanning the oversized frame with `extractOversizedFrameInfo`,
`oversizedFrameScanner`, and `newlineTerminatedReader`, so top-level routing
metadata can still be found after a large `result` or `error` field. The
regression test `TestStdioOversizeResponseWithLateIDUnblocksPendingSend` covers a
valid oversized response where `result` appears before `id`, and `Send`
resolves with the expected parse error instead of timing out.

### Same-thread completion notifications are not routed through dropping transport queues

**Location:** `stdio.go:848-925` — `enqueueTurnScopedNotification` and `turnScopedNotificationWorker`

**Reason:** The report describes `item/completed` and `turn/completed` as going through bounded
critical or terminal queues that evict older entries. That is not how the current transport
works. When those notifications carry a `threadId`, `enqueueNotification` sends them into the
per-thread `turnNotifQueues` map, where they are appended to a dedicated FIFO slice and drained by
`turnScopedNotificationWorker` without any drop-oldest path. The queueing bug described in the
report is stale for this checkout.
