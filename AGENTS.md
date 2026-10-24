# codex-sdk-go agent guidance

## Repository contracts

The checked-in JSON schemas in `appserver/protocol/schema/json/` define the
protocol surface. Inspect each affected carrier's schema, including nested
properties, requiredness, nullability, enums, defaults and union alternatives.

- Do not rename or remove public Go methods or change their signatures. Do not
  rename public types, fields or constants that map to the schemas.
- Do not change JSON-RPC method names, parameter/response shapes or notification
  types. Preserve approval request/response type names and structures.
- Schema-aligned corrections to these APIs are allowed to resolve demonstrated
  mismatches. Document source-incompatible changes and the caller migration.
- Preserve exported enum names, values and schema contracts, including those in
  `appserver/protocol/enums.go`. Reorganizing declarations must preserve those
  contracts; file organization alone is not a reason to change the public API.
- Preserve supported legacy public fields without making them required when
  absent from the current schema.
- Use the standard library only. `go.mod` contains the module and Go version,
  without external dependencies. Use its required Go version and the default
  JSON backend; do not restore the legacy backend override.

Within the assigned scope, fix internal error handling, transport and retry
behavior, request construction, response parsing and notification dispatch.
Unexported helpers, test improvements and schema-aligned Go field types are
permitted subject to the contracts above.

## Ownership

| Area | Owner |
| --- | --- |
| Protocol types, services, notification admission and schema coverage | `appserver/protocol/` |
| Stdio framing, request completion and notification queues | `appserver/transport/` |
| Process management, run/stream helpers and conversations | `appserver/` |
| Complete reference copying | `internal/deepcopy/` and its existing callers |
| OAuth flow and credential/token handling | `login/` and `login/auth/` |

## Protocol and decoder changes

- Enforce admission at existing owning codecs or service boundaries across every
  public carrier. Reject failed admission before successful response or callback
  publication. Inspect custom wire structs and assignment lists; a named Go type
  does not prove field preservation or enum enforcement. Shared types may
  have different constraints in different carrier schemas.
- Validate untagged `anyOf` alternatives independently in schema order. Preserve
  valid alternatives with malformed inactive properties, and reject malformed
  shared properties according to the schema.
- Check union marker and JSON method sets. Preserve supported values, pointers,
  typed nils, unknown raw variants, nested carriers and application extensions.
  Test constructed branches separately; a pointer to a nil slice can select a
  branch that serialization must not silently erase.
- Preserve absent, null, empty and populated collections, explicit false/zero
  values and schema defaults through serialization and copying. Trace omitted
  defaults to their reference codec; verify default and explicit round trips.
- Preserve each decoder's receiver updates, retained references, duplicate map
  merges, null resets and error contract. Check named/anonymous envelopes and
  malformed direct receiver calls using independent reference decoders and the
  prior revision. The default backend may continue after custom decoder errors.
- Keep public records method-free when their carriers own validation; promoted
  JSON methods can consume an application envelope. Run established owner checks
  before new restrictions, and verify mixed failures retain their precedence.
- Classify paths by schema, not by their names or server-produced examples. Do
  not resolve opaque request/response strings or normalize approval path evidence
  before the application sees it. Check request selectors and returned groups
  together when changing path behavior.

## Transport, notifications and state

- `StdioTransport` owns queue admission, response acceptance and terminal payload
  release. Coordinate acceptance/abandonment and admission/draining at shared
  boundaries. Preserve accepted outcomes and notification delivery after reader
  EOF, and actual-write completion for notifications and internal replies.
- Derive notification coverage from `ServerNotification.json` and typed handlers.
  Every known method needs a delivery policy; transport, typed decoding and
  malformed fallback must agree on owner identity. Bound retained bytes across
  queued, in-flight, fallback and replay ownership, not just event counts.
- Isolate callback recipients, including replay and direct cache/service calls.
  Preserve later delivery and finalization after callback or error-reporter
  failures, and preserve the diagnostic origin. Listener removal must release
  backing slots and empty registries
  without invalidating dispatch snapshots or lifecycle unsubscription.
- The Client's thread-state cache owns observation listeners and pins. Transfer
  response/registration ownership atomically across eviction; failed startup
  must leave neither listeners nor pins. Preserve the owned initial response
  without overwriting newer state or reopening a closed handle.
- Cache and runtime snapshots use `internal/deepcopy` through existing clone
  helpers. Preserve independent cache, conversation, collector, event and result
  views; verify mutations in both directions, nil/empty values and representative
  snapshot cost when changing reference ownership.
- Bound history backing storage as well as visible lengths. Own trimmed
  substrings and clear evicted slots across writers, aggregation and snapshots.
  Keep opaque tuple storage and presentation keys free of collisions, preserving
  compatible bare keys, insertion-order independence, accounting and cleanup.
- Process helpers own parent/descendant cleanup. Distinguish parent exit from
  descendant completion; remove observed terminal signals from repeated selects
  and verify waiting CPU cost, deadlines, concurrent Wait/Close and force-kill.
- Own serialized initialization extensions before waiting or child launch. A
  snapshot failure rejects admission without retaining a mutable fallback;
  recheck process cancellation after parameter preparation.

## Diagnostics and credentials

- Bound rejected peer-value previews before quoting and SDK wrapping. Trace
  escaping growth and the complete error chain, including original identifier
  display values as well as canonical matching keys; frame limits are insufficient.
- Establish native error ownership from the destination and caller. Preserve
  application JSON/text/streaming codec error identity and chains. Use
  `internal/jsonencode` at every SDK-owned protocol marshaling boundary before
  enclosing codecs wrap errors. Error types, message prefixes and private marker
  methods do not prove ownership or seal nested application interfaces.
- JSON selection changes must preserve duplicate, alias, null and malformed-prefix
  rules through typed delivery and fallback attribution. Distinguish ignored
  metadata work from selected-payload decoding. Verify successful typed/cache
  paths, malformed fields before/after dense metadata and nested recovery;
  measure temporary allocations and complete transport operations as well as
  selection helpers.
- At the shared token-response boundary, validate provider lifetimes against the
  target unit's range before arithmetic. Verify all grant paths reject failures
  without publishing partial credentials.

## Verification

Before publishing code changes, run the applicable checks defined in
[the CI workflow](.github/workflows/ci.yml), which owns the commands and pinned
tool versions. For schema changes, also run
`go test ./appserver/protocol -run TestSpecCoverage`; this proves type-name
coverage, not field preservation, enum admission or round-trip correctness.
For documentation-only changes, validate links and content. Configured hooks,
required reviews and CI still apply.

## Required task references

Before editing an affected area, read the relevant sections below and apply their
contracts and verification cases. These cases remain requirements where applicable;
the examples are not exhaustive. The explicit protections above still apply.

- Schema, codec, path or protocol admission changes: [protocol maintenance](docs/protocol-maintenance.md) and the affected public behavior in the [protocol guide](docs/protocol.md).
- Transport, notification, cache, snapshot, process or initialization changes: the affected ownership and lifecycle sections in [architecture](docs/architecture.md).
- Diagnostic or JSON-backend changes: [diagnostics and JSON selection](docs/protocol-maintenance.md#diagnostics-and-json-selection) and [backend compatibility](docs/json-backend-migration.md).
- Security-control or credential changes: the applicable controls and limits in the [threat model](docs/threat-model.md).
- Snapshot or transport performance changes: [thread-state benchmarks](docs/thread-state-benchmarks.md) or [transport benchmarks](docs/transport-benchmarks.md), as applicable.
- Schema synchronization changes: [spec-sync maintenance](docs/spec-sync.md).
