# Spec sync to 2c79ee6dacb6

PR #54 updates the SDK to the checked-in upstream snapshot at `2c79ee6dacb6`.
The JSON schemas in `appserver/protocol/schema/json/` are the source of truth.

## Protocol changes

- Added thread item/turn pagination, history revert, and plugin reconciliation.
- Added thread history mode, model, reasoning effort, pagination cursors, and turn
  detail/timing metadata. Legacy history defaults remain supported.
- Added auth recovery, MCP event-stream, and realtime item notifications with
  typed handlers and removable listeners. Realtime items and presentations use
  discriminated unions with forward-compatible unknown-variant preservation.
- Added tool-output turn inputs, function-call-output history items, asynchronous
  user questions, and misalignment error details.
- Added command approval kinds and both OpenAI MCP form spellings, preserving
  arbitrary JSON schemas while retaining the existing typed standard form API.
- Added Bedrock access-key login. Normal JSON and formatting redact credentials;
  only the transport's wire serializer includes credentials.
- Added browser/computer-use configuration, requirements, runtime status, usage
  metadata, skill attribution, shell-command timeout, and new auth/hook/collaboration enums.
- Expanded nested field coverage to catch metadata previously dropped from
  thread and MCP-server responses.

Existing raw JSON and interface fields continue to preserve schema extensions,
including guardian actions and raw Responses API items. Unreferenced definitions
such as `ThreadRealtimeStartTransport`, `ManagedHooksRequirements`, and `AppConfig`
do not add new client request methods in this snapshot.

## Validation

- `go test ./...`, `go build ./...`, `go vet ./appserver/protocol`, and
  `go mod tidy -diff` pass on Windows with Go 1.25.11.
- Protocol lint passes with the locally available golangci-lint 2.11.4;
  CI verifies the repository's pinned 2.11.3 and Linux race tests.
- Tests cover all 99 request methods, top-level and nested schema fields,
  typed notification dispatch, realtime/tool-output round trips, malformed
  payloads, legacy defaults, arbitrary MCP schemas, and credential redaction.

The stale-PR cleanup fix was merged separately as PR #55 after a clean Codex
review and passing CI. It uses the paginated REST bot identity instead of
comparing GraphQL's `app/github-actions` with REST's `github-actions[bot]`.
