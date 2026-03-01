# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Request handler goroutines are untracked and can outlive Close

**Location:** `stdio.go:354` — handleRequest goroutine dispatch
**Date:** 2026-02-27

**Reason:** Same root cause as the existing Send and Notify goroutine leak exceptions.
Tracking goroutines with a WaitGroup causes Close() to deadlock when handler goroutines
are blocked in `io.Writer.Write` (which has no context or deadline support). The only fix
is changing the writer API to `io.WriteCloser` so the writer can be closed to unblock
stuck writes — a breaking change disproportionate to the severity. In practice, the writer
is stdout to a child process and these goroutines terminate when the process exits.

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
