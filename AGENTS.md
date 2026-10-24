## Repository contracts

The checked-in JSON schemas in `appserver/protocol/schema/json/` define the
protocol surface. Inspect each affected carrier's schema before changing its Go
representation or codec.

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

## Required outcomes

- Enforce each carrier's schema at its existing admission owner before successful
  response or callback publication. Preserve union, presence, default and path
  semantics, including supported legacy fields and application extensions.
  Classify paths by schema; do not resolve opaque strings or normalize approval
  evidence locally.
- Preserve decoder receiver, reference and error contracts. Keep public records
  method-free when their carriers own validation, and preserve established check
  precedence when adding restrictions. Use the supported default JSON backend.
- Keep transport acceptance, abandonment and cleanup coordinated. Preserve
  accepted outcomes, required notification delivery and actual-write completion
  for notifications and internal replies.
- Give every schema notification a delivery policy and consistent owner identity.
  Bound retained payload memory across its full lifecycle. Isolate callback
  failures, preserve diagnostic origin and release listener resources safely.
- Preserve atomic cache observation and owned initial responses without
  overwriting newer state or reopening closed state. Failed observation startup
  must leave no listeners or pins. Use shared copy helpers for independent
  snapshots.
- Bound history backing storage and preserve opaque identity accounting and
  presentation. Process shutdown must cover descendant cleanup without spinning.
- Own initialization data before waiting or launching a child. Reject failed
  snapshots and honor process cancellation before launch.
- Bound SDK-owned peer-value diagnostics before wrapping while preserving
  application codec error ownership. Use `internal/jsonencode` for SDK-owned
  protocol marshaling; preserve JSON selection and fallback attribution contracts.
- Validate provider-controlled token lifetimes before arithmetic and credential
  publication; failures must not publish partial credentials.

## Required task references

Before editing an affected area, read the relevant sections below and apply their
contracts and verification cases. These cases remain requirements where applicable;
the examples are not exhaustive. The explicit protections above still apply.

- Schema, codec, path or protocol admission changes: [protocol maintenance](docs/protocol-maintenance.md) and the affected public behavior in the [protocol guide](docs/protocol.md).
- Transport, notification, cache, snapshot, process or initialization changes: the affected ownership and lifecycle sections in [architecture](docs/architecture.md).
- Diagnostic or JSON-backend changes: [decoder compatibility](docs/protocol-maintenance.md#decoder-compatibility) and [diagnostics and JSON selection](docs/protocol-maintenance.md#diagnostics-and-json-selection).
- Security-control or credential changes: the applicable controls and limits in the [threat model](docs/threat-model.md).
- Snapshot or transport performance changes: [thread-state benchmarks](docs/thread-state-benchmarks.md) or [transport benchmarks](docs/transport-benchmarks.md), as applicable.
- Schema synchronization changes: [spec-sync maintenance](docs/spec-sync.md).
