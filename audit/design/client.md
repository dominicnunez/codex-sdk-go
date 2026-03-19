### Notification handler registration silently overwrites previous handlers

**Location:** `client.go:138-142` — OnNotification
**Date:** 2026-02-27

**Reason:** One handler per method is the intentional dispatch model. The Client routes each
notification method to exactly one handler — the same pattern used by `http.HandleFunc` and
similar Go standard library APIs. Supporting multiple handlers adds complexity (slice management,
ordering semantics, error aggregation) without clear benefit. The `OnNotification` doc comment
states "Only one handler can be registered per method; subsequent calls replace the previous
handler." This is documented behavior, not a bug.

### ErrEmptyResult is a plain sentinel without method context for errors.As

**Location:** `client.go:329`, `client.go:362-364` — sendRequest and sendRequestRaw empty result handling
**Date:** 2026-02-27

**Reason:** Callers who want to know which method returned empty must parse the error string, since
`ErrEmptyResult` is a plain `errors.New` sentinel — not a struct with fields. Introducing an
`EmptyResultError` struct type with a `Method` field would add a new public type to the API for a
Low severity ergonomic improvement. The method name is already present in the `fmt.Errorf` wrapper
string for human-readable diagnostics, and `errors.Is(err, ErrEmptyResult)` works correctly for
programmatic detection. The typed error pattern used by `RPCError`/`TransportError`/`TimeoutError`
is justified by their higher severity and richer payloads.

### Internal notification listeners dispatched synchronously in handleNotification

**Location:** `client.go:192-209` — handleNotification dispatches public + internal listeners sequentially
**Date:** 2026-03-01

**Reason:** Internal listeners run sequentially within the goroutine spawned by `StdioTransport.handleNotification`.
If a listener blocks (e.g. on a full channel), it delays subsequent listeners for the same notification.
This is acceptable by design: the stream path now uses a bounded queue that applies backpressure while
an `Events()` consumer is attached, and it detaches the queue if iteration stops early so `Result()`
can still complete. Making listeners async would lose ordering guarantees and complicate error
propagation. The sequential dispatch matches the single-goroutine-per-notification model that the
transport layer establishes.
