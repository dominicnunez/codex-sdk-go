# Architecture and ownership

This reference describes the runtime boundaries and the required cases for
verifying their maintenance. Apply the relevant contracts and checks when
changing an affected area. The [README package map](../README.md#packages) and
[protocol guide](protocol.md) cover public API discovery; the [threat
model](threat-model.md) describes security controls and residual risks.

## Package boundaries

[Client](../appserver/protocol/client.go) wraps the protocol
[Transport](../appserver/protocol/transport.go) interface: `Send`, `Notify`,
`OnRequest`, `OnNotify` and `Close`. It provides typed services, request failure
classification and typed notification dispatch. Approval requests arrive through
`Transport.OnRequest`; the public [approval guide](protocol.md#approvals)
describes their schema-shaped parameters and responses. Typed notification APIs
include replacement `On<EventName>` handlers and additive listeners.

The repository includes the [stdio transport](../appserver/transport/stdio.go).
Callers can also supply their own Transport implementation. Process management,
[run helpers](../appserver/run.go), streaming collection and
[conversations](../appserver/conversation.go) live in `appserver/`; protocol
representations and schema coverage stay in `appserver/protocol/`. [OAuth
flow](../login/token.go) lives in `login/`, with credential persistence, token
claims, redaction and auth payload helpers in `login/auth/`.

## Transport completion and shutdown

Transport completion spans serialization, queue admission, writer return,
response publication and termination. Acceptance and abandonment need a shared
ownership boundary: peeking at a channel cannot coordinate a separate claim and
publication. Verify accepted responses and absent-response failures while the
writer is pending, as well as simultaneously ready completion and terminal
signals. Notifications and internal replies retain actual-write completion
rather than merely successful queue admission.

Reader-owned malformed-request and overload replies enter the existing serialized
writer queue without waiting for the external writer. This keeps response
correlation moving while bounded admission capacity is available. Writer execution
still owns actual completion and failure; request-worker replies keep their
synchronous completion contract. Reader rejection payloads have a separate 64 MiB
backing-capacity budget spanning blocked admission, queued envelopes and claimed
writes. Admission failure, terminal discard, skipped execution and writer return
retire that ownership. EOF follows normal terminal policy and may discard an
unclaimed reply; queue admission does not promise an EOF flush.

Terminal payload release spans the inbound queue, requests stored before handler
registration, replay, dequeued execution claims and outbound envelopes.
Admission and draining must remain coordinated so blocked or late producers
cannot refill stopped queues. A writer or callback already claimed can finish.
Preserve accepted outcomes and notification delivery after reader EOF when
verifying cancellation of unclaimed work. [Stdio
writing](../appserver/transport/stdio_write.go) and [transport
state](../appserver/transport/stdio.go) own these transitions.

`TransportLifecycle` is an optional `Done()`/`Err()` capability. Stdio's `Done`
closes after notification admission ends and every reservation has retired,
including queued notifications, pending handler replay and executing callbacks.
A separate reservation count covers zero-byte payloads. EOF preserves accepted
delivery; explicit close discards unclaimed notifications while claimed callbacks
can finish. Close does not synchronously wait for callbacks, so it can be called
from a callback. `ReaderStopped()` is an earlier boundary.

The client uses a capable transport's delivery boundary. For a custom transport
without the capability, `Client.Close` seals client notification entry and waits
asynchronously for entered dispatches to retire. It cannot observe that custom
transport's autonomous termination or its internal queues. Runtime helpers observe
`Client.Done()` after startup replay and prefer an already admitted matching
completion over transport failure. Transport termination does not disable stream
publication: runtime-owned replay and final events retain ordinary bounded
backpressure, so an attached consumer must drain, detach or cancel to make progress.

Request envelopes own decoded trace strings before queue admission. The reader
selects the trace field during envelope scanning and decodes it only after valid
request classification. Duplicate trace objects retain ordinary pointer merging
and null resets through a second borrowed pass; a single occurrence needs no
second envelope scan. Response correlation, notification delivery and invalid
ID/version handling ignore this request-only metadata. Queued requests and handler
replay carry the complete request value, and outgoing serialization copies the
trace strings into the existing owned write payload.

Streaming event publication observes caller cancellation independently of the
lifecycle's completion select. This releases backpressure while the lifecycle
replays startup events or publishes a terminal event. Releasing publication does
not choose the terminal error: a completion already selected by the lifecycle
retains its result or error. The cancellation registration is removed when that
lifecycle finishes.

Turn-start readiness establishes the active ID before replay, but live publication
waits until that replay finishes. A separate publication mutex covers collector
updates, event sends, item accumulation and completion admission. The startup
state mutex is released before replay or any blocking send. This keeps the
existing pending-notification and stream-queue bounds while preserving order at
the handoff in both blocking and streaming helpers. Turn-owned starts use this
same boundary: matching starts received before the response are buffered, and
other turn IDs are excluded from the operation's iterator. Client-wide typed
notification delivery retains its broader scope.

The shared completion owner records the terminal turn and collected items before
classifying the runtime outcome. A `failed` status is a failure independently of
the optional nullable diagnostic object. Detailed `TurnError` values retain their
error chain; absent details do not create synthetic protocol fields. Blocking and
streaming helpers share this classification, while streams retain the terminal
completion event before reporting failure. Foreign thread and turn identities
remain excluded from the active operation.

Stream iteration releases consumer ownership on every exit, including an
application panic or goroutine exit inside the iterator callback. Detachment
discards queued events and wakes blocked producers so collection and lifecycle
completion can continue. Application panics propagate unchanged; detachment does
not cancel the turn or permit a second `Events()` consumer.

Process shutdown has another boundary: parent exit and descendant completion are
different events. [Process management](../appserver/process.go) and
[process-tree support](../internal/process/process.go) coordinate managed Close,
Wait and force-kill. Verify concurrent Wait/Close, descendant liveness,
deadlines and waiting CPU cost. Removing already observed terminal signals from
repeated selects avoids spinning while other cleanup remains live.

Windows job-object records use separate 386 and 64-bit basic-limit declarations
to match the native ABI. The 386 record includes four trailing padding bytes;
the shared extended record then places I/O counters and memory limits at their
native offsets. Independent compiler assertions cover every basic/extended field,
record sizes and the I/O-counter size on all three supported Windows targets.
Native CI tests create and configure a job, attach an owned test child, and verify
that closing the last job handle terminates that child, including in a 386 binary.

Cancellation observed after tree attachment but before process publication uses
that attached owner for force-kill, then releases its handle before the parent
fallback kill and wait. Handle release can supply kill-on-close fallback if
explicit tree termination fails; descendants must not keep inherited pipes open
while the SDK waits for the parent. All cleanup steps are attempted, ordinary
cleanup preserves the original cancellation error, and material tree cleanup
failures are joined to it. Before successful attachment, failure cleanup has only
the command parent to own; current attachment implementations release provisional
handles on failure. No transport is published on these failed-startup paths.

## Notification delivery and listeners

[Notification scheduling](../appserver/transport/stdio_notifications.go) uses
explicit delivery policies for the methods in the checked-in [ServerNotification
schema](../appserver/protocol/schema/json/ServerNotification.json). Changes to
ordering or delivery require the schema inventory and typed handlers, including
methods outside an existing priority switch. Transport selection, typed decoding
and malformed fallback attribution must agree on owner identity. Bound retained
payload bytes across queued, in-flight, fallback and replay ownership; a queue's
event count alone does not establish that bound.

Callback fanout isolates each recipient, including initial replay and direct
service/cache calls. Committed state must still reach later SDK consumers if a
callback or error reporter panics. Preserve the diagnostic origin. Replay
failure must return registration cleanup, and callback finalization must survive
failures in error reporting as well as the primary handler. Listener removal clears vacated backing slots and
discards empty registries while preserving independent dispatch snapshots and
lifecycle unsubscription. Verify shutdown releases queued payloads as well as
preventing further callback admission.

Raw client notification callbacks receive an owned copy of `Params` on each
invocation, so retaining or modifying it cannot change another callback's input
or SDK decoding. Private protocol listeners decode the original bytes without
an extra copy; their payload input remains read-only. Runtime listeners registered
through the public raw APIs receive the same owned copies as application listeners.

## Cache observation

[Thread-state caching](../appserver/protocol/thread_state_cache.go) bridges the
pre-request interval, response publication, initial listener registration and
pinning. Each pending thread start captures at most 64 candidate identities,
their first admitted incarnation and sticky closure evidence before callbacks
or eviction. Temporary reference-counted pins share the latest cache snapshots
and admit at most 64 distinct identities across the client, including staggered
requests with disjoint histories. They can retain at most 64 additional entries
beyond the ordinary unpinned cache budget; application listeners have their own
existing pins. The pending requests do not clone a snapshot per candidate. Settings supplied by
notifications, including explicit empty plugin lists, supersede response
settings. The response fills only settings absent from a started notification.

The handoff to a listener happens under the cache lock before temporary pins are
released. Closure of the captured incarnation remains terminal even if a manual
cache publication reopens the identity. A candidate missing after capacity
overflow, a closure without its start, or distinct remote startup incarnations
using the same ID fails conservatively with `ErrThreadStartObservationLost`.
The schema supplies no request correlation or remote revision to resolve such
ambiguity. This mechanism does not order unrelated reads or refreshes globally.

Startup failures must leave neither listeners nor pins. An owned initial
response must remain available without overwriting newer delivery or reopening a
closed handle. The acquisition interval and later replay are part of the cache
lifecycle, rather than independent successful operations.
Every exit releases temporary pins and candidate maps; cache eviction and order
backing-storage compaction run after release. Each pending request has at most
64 candidate references, and the application controls concurrent pending starts.
When the shared identity budget is exhausted, already captured candidates stay
valid, while uncaptured histories fail conservatively at response handoff.
An overflowed observation never admits new candidate identities even if another
request later frees shared slots: the missing earlier history cannot be recovered.
Cancellation of one request leaves other requests' shared pins intact.
A valid response accompanied
by observation loss remains available to the direct caller, with no registered
listener, because local acquisition failure does not undo a remote start.

## Snapshots and retention

The shared [deep-copy implementation](../internal/deepcopy/deepcopy.go) and its
protocol/runtime callers establish reference ownership across cache,
conversation, collector, emitted event and final result views. New reference
fields belong in that complete boundary rather than partial field lists. Verify
mutation isolation in both directions with nil, empty and populated values, and
measure representative snapshot cost. See [thread-state
benchmarks](thread-state-benchmarks.md) for workloads and prior measurements.

The pointer allocator preserves a defined pointer's reflection type before
remembering it. Direct generic copies and interface metadata retain their Go
types, including typed nils. Repeated references and cycles within the same
concrete pointer type reuse their copied storage.

Bounded histories need bounded backing storage as well as bounded visible
lengths. Trimmed substrings can retain their original allocation, and evicted
slice slots can keep payloads alive. Own trimmed substrings and clear evicted
slots. Check every retention writer, completion aggregation and snapshot,
distinguishing bounded text from complete thread items.

[Stream collection](../appserver/stream_collector.go) also handles opaque
identity tuples. Storage keys and presentation keys have different consumers.
Compatible bare presentation keys need reservation before fallback selection;
test adversarial spellings, insertion-order independence, accounting and
completion cleanup for each tuple.

## Initialization and credentials

Authorization query parsing owns syntax validation for pasted full redirect
URLs, recognized raw queries and browser callbacks. Any malformed pair rejects the whole
query before code extraction, state comparison or authorization publication;
partial values from `url.ParseQuery` are discarded. Manual bare-code input still
uses its trusted-user exception for absent state, and a lower-level callback
with no expected state retains its comparison opt-out for valid queries.
Malformed input must send no token request or publish credentials. Authorization
endpoint configuration is caller-owned and remains separate from inbound query
parsing.

Recognized authorization error responses are terminal failures rather than
authorization codes. Browser callbacks compare the expected state before
publishing such a failure; wrong or missing state leaves the legitimate wait
active. Pasted full URLs and queries use the manual-input state policy and return
the same `ErrAuthorizationFailed` marker without exchanging a token. The shared
authorization waiter cancels competing input contexts on either success or
failure, and Login closes its callback listener on return. Provider error code
and description previews are bounded before error wrapping; callback HTML escapes
the resulting text. These failures do not prove reversal of any remote effect.
Callback close releases its owned listener even if shutdown precedes Serve's
registration, so immediate manual failure cannot leave the listener open until
a delayed serving goroutine runs. Graceful active-handler shutdown and forced
close on shutdown failure retain their existing behavior.

Initialization extensions are serialized and owned before waiting or child
launch. Snapshot failure rejects admission rather than retaining a mutable Go
value as fallback. Process cancellation is rechecked after parameter
preparation. Verify process cancellation after preparation prevents child
launch, and that snapshot failure sends no request, latches no initialized state
and permits a later valid attempt. Encoding enforces nesting-depth limits too;
deep extension payloads can fail before snapshot decoding. Include these
encoding failures when verifying rejection and recovery.

The [token-response boundary](../login/token.go) validates provider-controlled
lifetimes before arithmetic and credential publication for all grant paths.
Verify each grant rejects invalid lifetimes against the target unit's range
before arithmetic, and that failures publish no partial credentials. The [OAuth
threat-model
section](threat-model.md#oauth-callbacks-provider-routing-and-account-identity)
documents the implemented range, token-retention behavior and residual trust in
the provider's claims.
