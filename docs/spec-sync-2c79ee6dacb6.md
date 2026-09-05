# Spec sync to 2c79ee6dacb6

This is an in-progress implementation of the schema snapshot in PR #54.
The synced JSON files remain the source of truth. This PR is not ready to merge.

## Implemented

- `thread/items/list`: typed entries, turn association, and both pagination cursors.
- `thread/turns/list`: typed turns, item detail selection, timing metadata, and cursors.
- `thread/revert`: request validation, retained-history cursors, and thread cache update.
- `plugin/reconcile`: changed plugin capabilities and both failure lists.
- Thread history mode, model, reasoning effort, and resume pagination cursors.
- Legacy defaults for omitted `historyMode` (`legacy`) and `itemsView` (`full`).
- Request routing coverage for all 99 client request methods and field coverage for
  the eight new request/response schemas implemented above.
- Tests for request serialization, response decoding, optional zero values,
  required fields, invalid enums, and legacy payloads.

## Remaining

- Auth recovery started/completed and MCP event-stream notifications.
- Realtime item started/completed/transcript-delta notifications and their nested
  discriminated unions; existing-call realtime transport support.
- New turn-start fields and tool output, function-call-output item support,
  asynchronous user questions, and misalignment error details.
- Command approval kind, guardian write-stdin review, and MCP `openaiForm` support.
- Bedrock access-key login and auth mode; collaboration, hook, and activity enums.
- Browser/computer-use config and requirements, app links, and interrupt hooks.
- Account rate-limit metadata, MCP runtime status, skill plugin ID, shell-command
  timeout, raw response usage metadata, and remaining nested schema differences.
- Extend field, enum, union, notification, and round-trip coverage for these changes.

## Validation of the initial implementation

- `go build ./...` and `go vet ./appserver/protocol` pass with Go 1.25.11 on Windows.
- `go test ./appserver/protocol -run 'TestPaginatedHistory|TestPluginReconcile|TestAllRequestMethodsCovered' -count=1` passes.
- `go test ./...` fails only in `TestSpecCoverage` and `TestSpecFieldCoverage`,
  reflecting the unfinished notification types and existing-type updates above.
  All other packages pass. Five top-level notification schema types remain missing.
- The available golangci-lint 2.11.4 reports formatting issues in three unchanged
  files: `account.go`, `account_notifications.go`, and `approval_additional.go`.
  The repository's CI pins 2.11.3; changed Go files pass Go 1.25.11's formatter.

The independent stale-PR cleanup fix is in PR #55. Its CI passes. The September 2
run compared GraphQL's `app/github-actions` login with REST's `github-actions[bot]`
login and skipped every sync PR. The fix uses the paginated REST endpoint while
retaining the author, same-repository, branch-prefix, and current-PR safeguards.
