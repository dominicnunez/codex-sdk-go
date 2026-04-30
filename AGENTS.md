# codex-sdk-go project

## Spec Compliance

**The JSON schemas in `specs/` are the source of truth for the protocol surface.**

Do NOT:
- Rename, remove, or change signatures of public methods (they map 1:1 to Codex JSON-RPC methods)
- Rename public types, fields, or constants that map to spec schemas
- Change JSON-RPC method names, parameter shapes, or notification types
- Alter approval request/response type names or structures
- Remove or restructure `sdk/enums.go` constants

Do:
- Fix internal implementation (error handling, transport, retries, etc.)
- Add unexported helpers, improve test coverage
- Tighten types (e.g. `interface{}` → concrete type) as long as the public API stays compatible
- Fix bugs in request construction, response parsing, or notification dispatch

**When in doubt:** check the type against the corresponding `specs/*.json` schema before changing it.

Run `go test ./sdk -run TestSpecCoverage` to verify all specs have corresponding Go types.

## Architecture

### Zero Dependencies
This SDK uses **stdlib only** — no external dependencies. Do NOT introduce any. Check `go.mod`: it should only have the module line and Go version.

### Transport Layer
- `Transport` interface: `Send`, `Notify`, `OnRequest`, `OnNotify`, `Close`
- `StdioTransport`: production implementation over stdin/stdout
- All JSON-RPC framing handled internally — no external JSON-RPC libraries

### Client Pattern
`Client` wraps a `Transport` and provides typed methods for every JSON-RPC request. Timeout handling, error classification (`RPCError`, `TimeoutError`, `CanceledError`, `TransportError`), and notification dispatch all live here.

### Notification Handlers
Register via `client.On<EventName>(func(notif <Type>))`. Client dispatches incoming notifications to registered handlers by method name.

### Approval Flow
Server→client requests for user approval (command exec, file write, etc.) flow through `Transport.OnRequest`. Each approval type has `*Params` and `*Response` types matching specs.

### Test Infrastructure
- `MockTransport`: instant responses, records calls, supports injection
- `SlowMockTransport`: delayed responses for timeout testing
- `TestSpecCoverage`: ensures every spec schema has a Go type
