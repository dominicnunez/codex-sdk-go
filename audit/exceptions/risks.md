# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### StdioTransport.Close does not stop the reader goroutine

**Location:** `stdio.go:140-157` — Close() and readLoop()
**Date:** 2026-02-26

**Reason:** Fixing this requires changing the public API from `io.Reader` to `io.ReadCloser`,
which is a breaking change for all callers. The primary use case is `os.Stdin`, where the reader
goroutine terminates naturally with the process. In library contexts, callers control the underlying
reader and can close it themselves to unblock the scanner. The goroutine leak only matters for
long-running processes that create and discard many StdioTransport instances — a usage pattern
this SDK doesn't target.

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

### Notification handlers silently swallow unmarshal errors

**Location:** `account_notifications.go`, `turn_notifications.go`, and 25 other handlers
**Date:** 2026-02-27

**Reason:** Adding error surfacing requires either an `OnNotificationError` callback on Client
(new public API surface + all 27 handlers need plumbing) or changing handler signatures to return
errors (breaking change). The silent-drop behavior is consistent with JSON-RPC 2.0 notification
semantics where the server doesn't expect acknowledgment. Malformed notifications from the server
indicate a protocol-level bug that would manifest in other ways. The risk of silent data loss is
low relative to the API churn required to surface these errors.

### readLoop silently skips unparseable JSON lines with no diagnostic

**Location:** `stdio.go:250-253` — readLoop JSON unmarshal failure path
**Date:** 2026-02-27

**Reason:** Surfacing dropped-line counts requires new public API (e.g. a `DroppedMessages() uint64`
method on `StdioTransport`). The transport deliberately stays alive on malformed input — a single
bad line should not kill the connection. Pending requests for dropped responses will time out via
their context, which is the correct failure mode. Adding a counter is new API surface
disproportionate to a Low severity debugging-convenience finding.

### McpToolCallResult.Content and MCP metadata fields use untyped interface{}

**Location:** `event_types.go:197` — McpToolCallResult.Content, also `mcp.go` Resource/Tool metadata fields
**Date:** 2026-02-27

**Reason:** The upstream spec defines `McpToolCallResult.content` as `{"items": true, "type": "array"}`
— an array of any type, with no discriminated union or typed structure. Similarly, `Resource.Icons`,
`Resource.Meta`, `Tool.InputSchema`, etc. use open schemas (`true`) that accept arbitrary JSON.
Introducing typed content parts (e.g. `[]McpContentPart`) would be speculative — the spec deliberately
leaves these open for forward compatibility. Using `[]interface{}` (or `json.RawMessage`) is the
correct mapping for `"items": true`. Callers who need specific types can type-assert or re-unmarshal.

### Handler errors in handleApproval are invisible to SDK consumers

**Location:** `client.go:273-274` — handleApproval error return path
**Date:** 2026-02-27

**Reason:** When a user-supplied approval handler returns an error, it propagates to `handleRequest`
in `stdio.go` which replaces it with a generic `"internal handler error"` response on the wire.
The original error is never surfaced to the SDK consumer. Adding observability (e.g. an
`OnHandlerError` callback on `Client`) requires new public API surface. This is the same pattern
as the existing "notification handlers silently swallow unmarshal errors" and "writeMessage errors
silently discarded" exceptions — surfacing internal errors from goroutine-dispatched handlers
requires API additions disproportionate to the severity. Consumers who need observability can
wrap their handler functions with their own error logging before passing them to the SDK.

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

### SessionSourceSubAgent relies on implicit marshaling for SubAgentSource variants

**Location:** `thread.go:231-243` — SessionSourceWrapper.MarshalJSON
**Date:** 2026-02-27

**Reason:** The marshal path for `SessionSourceSubAgent` delegates to default `json.Marshal`,
while the unmarshal path uses explicit dispatch. The audit flags this asymmetry as fragile,
but all current `SubAgentSource` variants marshal correctly via struct tags. Adding an explicit
`MarshalJSON` to mirror the unmarshal dispatch adds code without fixing any bug. If a new
variant is added, the unmarshal dispatch already requires updating — the marshal side fails
visibly (wrong output) rather than silently, which is an adequate safety net.

### ConfigLayerSource type discriminator strings are hardcoded in each MarshalJSON

**Location:** `config.go:134-218` — seven ConfigLayerSource MarshalJSON methods
**Date:** 2026-02-27

**Reason:** Each variant hardcodes its type string (e.g. `"mdm"`, `"system"`) in an anonymous
struct literal. The audit suggests extracting named constants so marshal and unmarshal reference
the same value. However, the type strings are trivial string literals that appear exactly twice
each (once in MarshalJSON, once in the UnmarshalJSON switch), and the roundtrip is covered by
tests. Introducing constants for seven single-use pairs adds indirection without meaningful
safety gain.

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

### TurnStartParams and TurnSteerParams reset struct on partial unmarshal failure

**Location:** `turn.go:44-47`, `turn.go:116-119` — `*p = TurnStartParams{}` on error
**Date:** 2026-02-27

**Reason:** The audit itself concludes "No code change needed." The reset pattern is correct —
it ensures no partial state leaks on error. The note that future modifications must include
the reset is accurate but not actionable as a code change.

### handleApproval includes server-controlled method name in internal error strings

**Location:** `client.go:274,279,284` — error wrapping in handleApproval
**Date:** 2026-02-27

**Reason:** The `req.Method` string from the server is included in error messages, but these
errors never cross a trust boundary. `handleRequest` in `stdio.go` replaces all handler errors
with a generic `"internal handler error"` before sending the JSON-RPC response. The internal
error strings are not logged, stored, or exposed. This is a defense-in-depth observation with
no active vulnerability. Adding truncation/sanitization to internal error formatting adds
complexity without mitigating any concrete risk.

### Process.Close discards signal and kill errors during graceful shutdown

**Location:** `process.go:153-164` — Signal and Kill error handling in Close
**Date:** 2026-02-28

**Reason:** `Close()` discards errors from `Signal(os.Interrupt)` and `Process.Kill()` with `_ =`.
The transport close error takes priority (returned to caller). Signal errors are typically
`os.ErrProcessDone` (process already exited) which is not an error condition. Kill errors after
a grace period timeout are rare and non-actionable — the process is being force-terminated.
Surfacing these would require a structured multi-error return or a callback, which is
disproportionate to the severity. The caller cares whether cleanup succeeded (transport closed),
not about the intermediate signaling steps.

### Config flag values containing "=" are ambiguous on the CLI

**Location:** `process.go:87` — buildArgs config flag construction
**Date:** 2026-02-28

**Reason:** Config flags are constructed as `--config k=v`. If the value itself contains `=`,
the CLI may split on the first `=` and misinterpret the value. This is a CLI-side parsing
concern — the SDK faithfully passes what the caller provides. The CLI's `--config` flag parser
determines how `key=a=b` is interpreted, and the standard convention (split on first `=`) handles
this correctly. Adding quoting or escaping in the SDK without knowing the CLI's parser would be
speculative and could introduce new ambiguity.

### Thread ID filtering in notification listeners lacks a cross-thread contamination test

**Location:** `turn_lifecycle.go:34-38`, `run_streamed.go:87-91` — threadID filter
**Date:** 2026-02-28

**Reason:** Testing cross-thread contamination requires running two concurrent turns on different
threads through the full turn lifecycle, which needs mock infrastructure for concurrent thread/turn
start responses with different thread IDs and interleaved notifications. The current mock transport
supports one response per method, making this test require significant test infrastructure changes.
The filter logic itself is trivial (`carrier.ThreadID != p.threadID`) and exercised in every
existing turn test — only the negative path (mismatched IDs) lacks coverage.

### handleRequest error-code dispatch through StdioTransport lacks integration test

**Location:** `stdio.go:422-441` — errInvalidParams branching in handleRequest
**Date:** 2026-02-28

**Reason:** All approval handler tests use MockTransport which bypasses the real handleRequest
error classification. Testing this code path requires a real StdioTransport with piped readers/
writers, a registered handler that returns an errInvalidParams-wrapped error, and verification
of the JSON-RPC response error code. This is integration-level testing that requires substantially
more setup than unit tests. The branching logic is simple (one errors.Is check) and the risk of
incorrect error codes is low — both branches produce valid JSON-RPC error responses.

### Internal listener sequence counter can theoretically wrap around and collide

**Location:** `client.go:217` — internalListenerSeq uint64 increment
**Date:** 2026-02-28

**Reason:** `internalListenerSeq` is incremented without overflow checking. After 2^64
increments it wraps to 0 and subsequent IDs could collide with still-registered listeners.
However, 2^64 operations is unreachable in any realistic runtime — at 1 billion increments
per second it would take ~584 years. Adding overflow detection or a different ID scheme
is disproportionate to the near-zero probability of occurrence.

### ensureInit holds mutex across RPC round-trip, serializing concurrent callers

**Location:** `process.go:192-206` — ensureInit
**Date:** 2026-03-01

**Reason:** `ensureInit` holds `initMu` across the `Initialize` RPC call. Concurrent
`Run`/`RunStreamed` callers serialize behind this lock, and if the first caller's context
expires mid-init, subsequent callers must retry. Replacing the mutex with a `sync.Once`-like
done channel requires careful error-retry semantics (the current design deliberately retries
on failure by keeping `initDone` false). The serialization only affects the very first call
on a fresh Process — after `initDone` is latched, the lock is held for a single boolean
check. The risk is low and the fix requires non-trivial concurrency redesign for a one-time
startup path.

### Process.Close transport error propagation lacks a unit test

**Location:** `process.go:156-187` — Close transport.Close error path
**Date:** 2026-03-01

**Reason:** Testing that `transport.Close()` errors propagate through `Process.Close()`
requires injecting a mock transport into the unexported `transport` field. The field is
unexported by design (callers should not interact with the transport directly).
Exporting it or adding a test-only constructor solely for this test adds public API surface
for a Low-severity testing gap. The code path is trivial (one `if err != nil` assignment)
and is exercised indirectly by integration tests with real processes.

### Conversation and turn tests use time.Sleep for goroutine synchronization

**Location:** `conversation_test.go:42,67,111,256,315,525,554,623,664,715` — and similar in `run_test.go`, `run_streamed_test.go`
**Date:** 2026-03-01

**Reason:** Nearly every turn-based test uses `time.Sleep(50ms)` between starting a goroutine
that calls `Turn()` and injecting the completion notification. Replacing these with deterministic
signals requires adding a method-call signaling mechanism to MockTransport (e.g. a channel that
fires when `turn/start` is sent). The mock transport currently returns immediately from `Send`,
so the 50ms sleep is reliably sufficient. The fix requires non-trivial test infrastructure
changes across ~15 tests for a low-severity code smell. The tests have never flaked in CI.

### Notification listeners double-unmarshal threadIDCarrier for thread filtering

**Location:** `turn_lifecycle.go:35-36,49-50`, `run_streamed.go:97-101` — threadIDCarrier pre-parse
**Date:** 2026-03-01

**Reason:** Every notification listener in the turn lifecycle first unmarshals a `threadIDCarrier`
to check the threadID, then unmarshals the full notification struct. This compounds with the
existing readLoop double-parse (each notification is parsed 4 times total). Fixing this requires
restructuring all notification listeners to unmarshal the full typed struct first, then check
the threadID field from the result — which changes the filter-then-parse pattern used consistently
across all listeners. The overhead is negligible for the small JSON-RPC payloads that dominate
notification traffic. Same risk profile as the readLoop double-parse exception above.

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

### Unbounded agent map growth in AgentTracker

**Location:** `collab_tracker.go:51-73` — agents map never prunes terminal entries
**Date:** 2026-03-01

**Reason:** `AgentTracker.agents` grows without bound as agents reach terminal states. Adding
automatic pruning requires choosing a retention policy (time-based? count-based? immediate on
terminal?), which changes the observable behavior of `Agents()` — callers currently see the full
history of all agents. A `Prune()` method would be the least disruptive option but adds public
API surface. In practice, sub-agent counts per session are small (tens, occasionally hundreds).
The memory held per `AgentInfo` is ~100 bytes. Even 10,000 agents would consume ~1MB, which is
negligible relative to the process memory. The growth is linear in the number of unique agents
spawned, not in the number of events processed.
