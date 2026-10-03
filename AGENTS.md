# codex-sdk-go project

## Spec Compliance

**The JSON schemas in `appserver/protocol/schema/json/` are the source of truth for the protocol surface.**

Do NOT:
- Rename, remove, or change signatures of public methods (they map 1:1 to Codex JSON-RPC methods)
- Rename public types, fields, or constants that map to spec schemas
- Change JSON-RPC method names, parameter shapes, or notification types
- Alter approval request/response type names or structures
- Remove or restructure `appserver/protocol/enums.go` constants

Do:
- Fix internal implementation (error handling, transport, retries, etc.)
- Add unexported helpers, improve test coverage
- Tighten types (e.g. `interface{}` → concrete type) as long as the public API stays compatible
- Fix bugs in request construction, response parsing, or notification dispatch

**When in doubt:** check the type against the corresponding `appserver/protocol/schema/json/*.json` schema before changing it.

Run `go test ./appserver/protocol -run TestSpecCoverage` to verify all specs have corresponding Go types.

For schema field changes, inventory properties, requiredness, nullability and
union branches, including nested definitions. Check custom wire structs and
assignment lists through public calls; type-name coverage cannot detect field
loss. Preserve present empty values and legacy public fields without requiring
fields absent from the current schema.
For nullable collections, distinguish absent, null, empty and populated values
with JSON member checks. Trace aliases, duplicate/reset and reused receiver
behavior; presence state must preserve the shared deep-copy contract.
Check scalar zero values separately from nullability: schema defaults must not
silently omit counters, and nonnil optional pointers must retain false or zero.
Test constructed union branches separately from decoded values, including
pointers to nil slices. Nullability must not erase an explicitly selected branch.

## Architecture

### Zero Dependencies
This SDK uses **stdlib only** — no external dependencies. Do NOT introduce any. Check `go.mod`: it should only have the module line and Go version.

### Transport Layer
- `Transport` interface: `Send`, `Notify`, `OnRequest`, `OnNotify`, `Close`
- Runtime transports are provided by callers or companion packages

### Client Pattern

For transport completion changes, trace serialization and queue admission through
writer return, response publication and termination. Acceptance and abandonment
must share an ownership boundary; a channel peek cannot cover a separate claim
and publication. Verify accepted outcomes and absent-response failures with both
writer-pending and simultaneously ready terminal signals. Preserve actual-write
completion for notifications and internal replies.
For terminal payload release, coordinate queue admission and draining at one
ownership boundary. Trace pre-handler storage, replay and dequeued execution
claims; preserve accepted outcomes and notification delivery after reader EOF.

`Client` wraps a `Transport` and provides typed methods for every JSON-RPC request. Timeout handling, error classification (`RPCError`, `TimeoutError`, `CanceledError`, `TransportError`), and notification dispatch all live here.

### Notification Handlers
Register via `client.On<EventName>(func(notif <Type>))`. Client dispatches incoming notifications to registered handlers by method name.

For transport ordering or delivery changes, derive the affected inventory from
`ServerNotification.json` and typed handlers, including methods outside existing
priority switches. Check that transport, typed and malformed fallback decoders
agree on owner identity. Verify that shutdown releases queued payloads as well
as preventing further callback admission. Require an explicit delivery policy
for every known schema method, and verify that callback finalization survives
failures in error-reporting hooks as well as in the primary handler.
For peer-controlled buffers, bound retained data as well as event counts across
queued, in-flight, fallback and replay ownership.
For callback fanout, isolate each recipient at the owning boundary, including
initial replay and direct service/cache calls. Verify committed state reaches
later SDK consumers after a callback or error reporter panics, and that replay
failure still returns registration cleanup. Preserve the diagnostic origin.
For peer-controlled JSON selection, check temporary allocations across ignored
fields, duplicate selected fields, identity length and nesting depth. Preserve
the actual duplicate, alias, null and malformed-prefix rules; measure complete
transport operations as well as isolated selection helpers.
Follow identity selection into typed callbacks and runtime fallback attribution;
distinguish ignored-property work from necessary semantic decoding of selected
payloads. Check malformed semantic fields before and after dense metadata,
including nested recovery and successful typed/cache paths.
When replacing shared decoders, compare the complete public error contract,
including outer wrappers, error precedence and direct receiver calls with invalid
JSON. Keep reference models independent and verify new regressions against the
prior revision instead of copying a helper without its calling boundary.
For map-valued fields, preserve ordered duplicate object merges and null resets,
including decoding into receivers that already contain map entries.
Compare receiver updates and retained references on type errors as well as on
success. Include envelopes and user-defined wrappers: a custom decoder error
can stop outer traversal and change error precedence, field context and offsets.

### Approval Flow
Server→client requests for user approval (command exec, file write, etc.) flow through `Transport.OnRequest`. Each approval type has `*Params` and `*Response` types matching specs.
Classify path fields by their schema, not their names: opaque legacy strings
remain request evidence, while AbsolutePathBuf fields have separate validation.
Do not resolve or normalize approval metadata before the application sees it.

### Test Infrastructure
- `MockTransport`: instant responses, records calls, supports injection
- `SlowMockTransport`: delayed responses for timeout testing
- `TestSpecCoverage`: ensures every spec schema has a Go type

### Runtime Packages
Protocol types and generated schema coverage stay in `appserver/protocol/`. Process management, single-turn `Run`, streamed run helpers, and conversation helpers live in `appserver/`. Stdio framing lives in `appserver/transport/`.

For shutdown waits, separate parent completion from descendant completion. Remove
observed terminal signals from repeated selects; verify waiting CPU cost as well
as deadlines through managed Close, including concurrent Wait/Close and force-kill.

When adding reference-valued protocol fields, trace ownership through cache,
conversation, collector, emitted events and final result views. Use the existing
complete copy boundary rather than a partial field list; verify mutation in both
directions, nil/empty values and representative snapshot cost.
For initialization extensions, own serialized JSON before waiting or child
launch. A snapshot error must fail admission; do not retain a mutable Go value
as fallback. Recheck process cancellation after parameter preparation.

### Login Packages
Codex OAuth login flow lives in `login/`. Credential persistence, token claims, redaction, and `chatgptAuthTokens` payload helpers live in `login/auth/`.
