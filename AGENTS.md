# codex-sdk-go agent guidance

## Repository contracts

- The checked-in JSON schemas in `appserver/protocol/schema/json/` define the
  protocol surface. Inspect the affected carrier's schema, including nested
  definitions, before changing its Go representation or codec.
- Preserve schema-backed names, enum values, wire methods and parameter/response
  shapes. Schema-aligned API corrections may be source-incompatible; document
  their migration. Do not remove legacy public fields merely because the current
  schema omits them. File layout, including `enums.go`, is not a wire contract.
- Use the standard library only. `go.mod` contains the module and Go version,
  without external dependencies. Use Go 1.27.3 or later with its default JSON
  backend; do not restore the legacy backend override.

## Ownership

| Area | Owner |
| --- | --- |
| Protocol types, services, notification admission and schema coverage | `appserver/protocol/` |
| Stdio framing, request completion and notification queues | `appserver/transport/` |
| Process management, run/stream helpers and conversations | `appserver/` |
| Complete reference copying | `internal/deepcopy/` and its existing callers |
| OAuth flow and credential/token handling | `login/` and `login/auth/` |

## Change boundaries

- Enforce schema admission at the existing owning codec or service boundary,
  across every public carrier. Keep plain public records method-free when their
  carriers own validation; JSON method promotion can consume application envelopes.
- Preserve absent, null, empty and zero values, union branches and schema defaults.
  Validate each carrier's actual contract; shared Go types can have different
  constraints. Classify paths by schema, without resolving opaque strings locally.
- Protocol record codecs and `OptionalNullable[T]` have distinct receiver-update
  contracts. Check the default `encoding/json` behavior rather than assuming
  custom decoder errors stop enclosing traversal. Keep existing record checks
  before new restrictions.
- Preserve application codec error ownership. Bound peer-controlled diagnostics
  before wrapping, and use `internal/jsonencode` for SDK-owned protocol marshaling.
  A concrete error type, message prefix or private marker does not prove ownership.
- Cache and runtime snapshot helpers use `internal/deepcopy` for complete
  reference copying. Use those helpers rather than adding partial field-copy paths.
- `StdioTransport` owns queue admission, response acceptance and terminal payload
  release. The Client's thread-state cache owns observation listeners and pins;
  process helpers own parent/descendant cleanup. Preserve these owner boundaries.

## Verification

The code gates in `.github/workflows/ci.yml` are:

```sh
gofmt -l .  # output must be empty
go mod tidy -diff
go build ./...
go vet ./...
go test ./...
go test -race ./...
golangci-lint run --config=.golangci.yml ./...  # v2.14.0, matching CI
```

For schema work, also run
`go test ./appserver/protocol -run TestSpecCoverage`. This checks type-name
coverage; it does not prove field preservation, enum admission or round trips.
Validate links and content for documentation-only changes. Configured hooks,
required reviews and CI still apply.

## Task references

- Protocol codecs, schema cases and test fixtures: [protocol maintenance](docs/protocol-maintenance.md).
- Runtime ownership and lifecycle cases: [architecture](docs/architecture.md).
- Public API behavior, path semantics and migrations: [protocol guide](docs/protocol.md).
- Security controls and residual limits: [threat model](docs/threat-model.md).
- JSON compatibility: [backend migration](docs/json-backend-migration.md).
- Snapshot costs and transport parsing: [thread-state benchmarks](docs/thread-state-benchmarks.md) and [transport benchmarks](docs/transport-benchmarks.md).
- Schema synchronization: [spec-sync maintenance](docs/spec-sync.md).
