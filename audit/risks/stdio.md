# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Write goroutine in Send can leak on context cancellation

**Location:** `stdio.go:86-102` — Send's write goroutine
**Date:** 2026-02-27

**Reason:** The goroutine runs `writeMessage`, which acquires `writeMu` and calls
`io.Writer.Write`. Go's `io.Writer` interface has no context or deadline support, so
there's no way to interrupt a blocked write without closing the underlying writer.
Fixing this requires either changing the public API to accept `io.WriteCloser` (breaking),
or wrapping all writes in a deadline-aware layer that introduces complexity disproportionate
to the risk. In practice, the writer is stdout to a child process — writes block only if the
process is hung, at which point the entire SDK session is stuck regardless.

### writeMessage errors silently discarded in handleRequest goroutine

**Location:** `stdio.go:334, 356, 363` — three `_ = t.writeMessage()` calls
**Date:** 2026-02-27

**Reason:** Surfacing write errors requires new public API (a callback or `ScanErr()`-style
retrieval method). The three call sites are in goroutines spawned by `handleRequest` where
there is no caller to return an error to. The comments acknowledge "nothing more we can do."
If a write fails, the server will time out the request — adding client-side signaling requires
API surface disproportionate to the severity (Low) and the practical impact (writes to stdout
rarely fail mid-session).

### readLoop silently skips unparseable JSON lines with no diagnostic

**Location:** `stdio.go:250-253` — readLoop JSON unmarshal failure path
**Date:** 2026-02-27

**Reason:** Surfacing dropped-line counts requires new public API (e.g. a `DroppedMessages() uint64`
method on `StdioTransport`). The transport deliberately stays alive on malformed input — a single
bad line should not kill the connection. Pending requests for dropped responses will time out via
their context, which is the correct failure mode. Adding a counter is new API surface
disproportionate to a Low severity debugging-convenience finding.

### readLoop starts before handlers are registered, risking dropped early messages

**Location:** `stdio.go:85-86` — NewStdioTransport starts readLoop immediately
**Date:** 2026-02-27

**Reason:** Fixing this requires either adding a `Start()` method to the public API (breaking
the constructor-starts-transport contract) or buffering messages internally until handlers are set
(adding complexity and a new failure mode). In practice, the JSON-RPC protocol requires the client
to send `initialize` before the server sends any messages, so the handler registration in
`NewClient` always completes before any server messages arrive. The theoretical race window
(goroutine scheduling between `go t.readLoop()` and `transport.OnNotify`/`transport.OnRequest`)
is not reachable under the protocol's actual message ordering.

### readLoop double-parses every incoming JSON message for routing

**Location:** `stdio.go:271-303` — readLoop routing parse
**Date:** 2026-02-28

**Reason:** Every incoming message is fully tokenized twice: once in readLoop to extract
routing fields (id, method), and again in the handler to unmarshal the full typed struct.
Fixing this requires replacing the standard json.Unmarshal routing parse with a custom
tokenizer or streaming decoder that stops after finding the two top-level routing keys.
This is a significant change to core transport parsing for a low-severity performance
concern. The current double-parse is correct and simple. For the vast majority of messages
(small JSON-RPC payloads), the overhead is negligible. The only case where it matters is
large file diffs, which are infrequent relative to total message volume.

### A reader that cannot be closed cannot be force-unblocked during transport shutdown

**Location:** `stdio.go:275-281`, `stdio.go:518-539` — constructor contract and shutdown close path
**Date:** 2026-03-02

**Reason:** `NewStdioTransport` now requires an `io.ReadCloser` and stores that closer so
`Close` can always call `readerCloser.Close()` to unblock the read loop for normal stdio-style
readers (`os.File`, pipes, sockets). The remaining risk is narrower: Go's stdlib still offers no
generic guarantee that an arbitrary `io.ReadCloser` implementation will make a blocked `Read`
return promptly when `Close` is called. Eliminating that edge case would require a stronger
interruptible-reader contract or a different transport abstraction, which is disproportionate to
this lifecycle concern.

### handleRequest error-code dispatch through StdioTransport lacks integration test

**Location:** `stdio.go:422-441` — errInvalidParams branching in handleRequest
**Date:** 2026-02-28

**Reason:** All approval handler tests use MockTransport which bypasses the real handleRequest
error classification. Testing this code path requires a real StdioTransport with piped readers/
writers, a registered handler that returns an errInvalidParams-wrapped error, and verification
of the JSON-RPC response error code. This is integration-level testing that requires substantially
more setup than unit tests. The branching logic is simple (one errors.Is check) and the risk of
incorrect error codes is low — both branches produce valid JSON-RPC error responses.

### Unbounded goroutine spawning for incoming messages

**Location:** `stdio.go:414,478` — handleRequest and handleNotification dispatch
**Date:** 2026-03-01

**Reason:** Every incoming server request and notification spawns a new goroutine via `go func()`.
Adding a bounded worker pool or semaphore requires architectural changes to the transport layer:
introducing a configurable concurrency limit, deciding on backpressure semantics (drop vs block),
and handling the interaction between the semaphore and the transport's shutdown sequence. The SDK
communicates with a single local process over stdio — the message rate is bounded by the server's
output speed, which is itself bounded by LLM inference latency. Unbounded goroutine creation is
only a concern if the server is malicious or buggy, neither of which is a realistic threat model
for a local subprocess. The fix requires disproportionate complexity for a scenario outside the
SDK's threat model.

### Notification handlers dispatched concurrently without ordering guarantees

**Location:** `stdio.go:478` — handleNotification goroutine dispatch
**Date:** 2026-03-01

**Reason:** `handleNotification` dispatches each notification handler in a new goroutine.
Two rapid notifications can arrive in wire order but be delivered out of order. For streaming
deltas (`item/agentMessage/delta`), this could cause text reassembly corruption. Fixing this
requires either sequential dispatch (which blocks the readLoop on slow handlers) or an ordered
queue per notification method (significant transport-layer redesign). The SDK's internal
listeners already handle ordering at the consumer level — `streamSendEvent` delivers events
through a channel that preserves insertion order. External `OnNotification` handlers receive
raw notifications where ordering is the caller's responsibility. The architectural cost of
transport-level ordering guarantees is disproportionate to the severity.

### No test for transport readLoop shutdown-during-dispatch behavior

**Location:** `stdio.go` — readLoop/handleNotification/handleRequest
**Date:** 2026-03-01

**Reason:** Testing that `Close()` during active handler dispatch completes gracefully without
goroutine leaks requires registering slow handlers, injecting messages, calling `Close()`, and
verifying goroutine counts (e.g. via `goleak`). The existing `TestStdioConcurrentSendAndClose`
exercises concurrent close but does not verify in-flight handler completion. Building this test
requires either adding `goleak` as a test dependency or using `runtime.NumGoroutine` snapshots
with timing-sensitive assertions. The handlers already recover from panics and the transport's
context cancellation unblocks context-aware handlers. The risk of a goroutine leak on close is
low given the process-scoped lifecycle of StdioTransport.
