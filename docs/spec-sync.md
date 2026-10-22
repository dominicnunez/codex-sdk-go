# Spec-sync maintenance

The `spec-sync` workflow checks the upstream Codex app-server schemas on its schedule and through manual dispatch. When it detects changes, it commits the schema snapshot to a bot-owned `spec-sync/*` branch and opens a pull request.

## Superseded pull requests

Spec-sync workflow runs are serialized with `cancel-in-progress: false`. A run creates or finds its current pull request before looking for older sync pull requests. It then closes an older pull request only when all of these conditions hold:

- the REST API author login is `github-actions[bot]`;
- the head branch belongs to this repository;
- the head branch matches `spec-sync/*`;
- the pull request is not the current run's pull request.

Creating the current pull request first is intentional. It prevents two near-simultaneous runs from both observing an empty pull-request list and leaving duplicate sync pull requests open.

Do not weaken the author, repository, branch-prefix, or current-PR exclusions in the cleanup step. They prevent the workflow from closing unrelated contributor pull requests.

Cleanup uses the paginated REST pulls endpoint in `scripts/spec-sync-prune.sh`.
Do not compare `gh pr list --json author` against the REST login: that GraphQL
command returns `app/github-actions` for the same bot. This mismatch previously
caused every bot PR to be skipped while the workflow still reported success.
Branch deletion runs as part of closing each superseded PR.

## Schema snapshot fea1d7f7d163

The reachable additions are read-state notifications and unread positions,
generated-image attachment references and local-input provenance, tool-output
retention, opaque MCP tool-list metadata, `sci` plan tiers and model-request
hook events. The nested `ThreadReadState` definitions in thread responses do not
add a read-state property to `Thread`; notifications expose that state separately.

| Contract | Owner and callers | Evidence | Decision |
| --- | --- | --- | --- |
| Read-state required fields, position branches and null/absent distinction | Typed notification decoder and both listener APIs | `TestSyncFea1ReadState`, `TestSyncFea1ConstructedUnions`, `TestReadStateNotificationLifecycle` | Validate before callbacks; preserve empty identifiers and revisions |
| Read-state delivery under pressure and per-thread ownership | Stdio protected notification queue | `TestStdioAllKnownNotificationsSurviveBestEffortPressure`, `TestStdioOrdersAllSchemaNotifications` | Protected delivery uses the exact `threadId` owner |
| Image reference/inline alternatives, input provenance and typed failure | `ThreadItemWrapper` in history and item notifications | `TestSyncFea1ImageGeneration`, `TestSyncFea1ConstructedUnions`, `TestGeneratedImageFailureAdmission` | Preserve file references even when legacy inline data is also present; reject missing alternatives and malformed input or failure records |
| New references remain isolated in runtime views | Shared deep-copy boundary in cache, conversation, events and results | `TestEveryItemSnapshotOwnsReferences`, `TestRunResultOwnsAllItemAndTurnReferences`, `TestConversationCacheAndSnapshotItemOwnership`, `TestStreamEventsCollectorAndResultOwnership` | Exercise populated, empty and nil references and mutations in both directions |
| Retention, catalog metadata and new enums survive typed codecs | Turn start, MCP status, account and hook carriers | `TestSyncFea1Fields`, `TestSyncedFieldsThroughServices`, schema field and enum coverage | Preserve false/zero/empty/null metadata and numeric tokens |

The Responses API additions remain inside the existing raw `Thread.InjectItems`
carrier. `RawResponseItemCompletedNotification` is absent from the server-method
union and remains excluded as documented in `TestSpecCoverage`. The changed
`ManagedHooksRequirements` and realtime-voice definitions, and the new
`ThreadReadStateOperation` definition, have no referencing wire entry points in
this snapshot; they do not establish new SDK service methods.
