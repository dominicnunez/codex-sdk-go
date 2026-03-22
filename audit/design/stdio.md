### Notify goroutine can write to writer after context cancellation

**Location:** `153-156`

**Reason:** Same root cause as the existing Send goroutine exception. When `Notify` returns
early via `ctx.Done()` or `readerStopped`, the goroutine running `writeMessage` continues and
may deliver a notification the caller believes was not sent. The write itself is safe (protected
by `writeMu`), and notifications are fire-and-forget by definition — a delivered notification
has no negative side effect. Fixing this requires `io.WriteCloser` (same API change discussed
in the Send exception), which is disproportionate to the severity.

### Notify may succeed even if the transport reader has just stopped

**Location:** `135-156`

**Reason:** Same design tradeoff as the existing Send goroutine exception. The write goroutine
acquires `writeMu` and calls `io.Writer.Write`, which has no context or deadline support.
Between the `t.closed` check and the goroutine running, the reader could stop — but the
`select` at the wait phase handles this correctly. If the write completes before `readerStopped`
fires, `Notify` returns nil even though the transport is dying. This is benign: the notification
is fire-and-forget by definition, and the next Send call will fail with the transport error.
Fixing this requires the same `io.WriteCloser` API change discussed in the Send goroutine
exception, which is disproportionate to the severity.

### Inbound messages are decoded once for routing and again for typed handlers

**Location:** `527`

**Reason:** The current transport intentionally performs a lightweight envelope decode for routing (`id`, `method`, protocol checks) and leaves typed decoding to request/notification handlers. Eliminating the second decode would require broader restructuring across dispatch and handler contracts for marginal benefit in expected SDK workloads. The current design keeps routing logic simple, explicit, and resilient under malformed input while preserving typed parsing boundaries.
