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

Terminal payload release spans the inbound queue, requests stored before handler
registration, replay, dequeued execution claims and outbound envelopes.
Admission and draining must remain coordinated so blocked or late producers
cannot refill stopped queues. A writer or callback already claimed can finish.
Preserve accepted outcomes and notification delivery after reader EOF when
verifying cancellation of unclaimed work. [Stdio
writing](../appserver/transport/stdio_write.go) and [transport
state](../appserver/transport/stdio.go) own these transitions.

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

[Thread-state caching](../appserver/protocol/thread_state_cache.go) bridges
response publication, initial listener registration and pinning. Callbacks and
eviction can occur during acquisition. Observation ownership must transfer
atomically: absence from the cache cannot distinguish ordinary eviction from an
evicted closure marker.

Startup failures must leave neither listeners nor pins. An owned initial
response must remain available without overwriting newer delivery or reopening a
closed handle. The acquisition interval and later replay are part of the cache
lifecycle, rather than independent successful operations.

## Snapshots and retention

The shared [deep-copy implementation](../internal/deepcopy/deepcopy.go) and its
protocol/runtime callers establish reference ownership across cache,
conversation, collector, emitted event and final result views. New reference
fields belong in that complete boundary rather than partial field lists. Verify
mutation isolation in both directions with nil, empty and populated values, and
measure representative snapshot cost. See [thread-state
benchmarks](thread-state-benchmarks.md) for workloads and prior measurements.

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
