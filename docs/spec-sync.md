# Spec-sync maintenance

The `spec-sync` workflow checks the upstream Codex app-server schemas on its schedule and through manual dispatch. When it detects changes, it commits the schema snapshot to a bot-owned `spec-sync/*` branch and opens a pull request.

## Download admission

The workflow downloads root, v1 and v2 files into a temporary candidate directory.
Transfers must complete with a final HTTP 2xx status, including after redirects.
Python's standard-library JSON parser then checks every candidate `.json` file
before the workflow compares or replaces the tracked snapshot, updates the
baseline, publishes a branch or changes pull requests. Empty, malformed,
multiple-document and nonstandard non-finite JSON bodies fail this gate.

Download, inventory or JSON-validation failure exits before publication and
removes the temporary candidate. Existing tracked schemas, the baseline and
earlier sync proposals remain intact. This checks JSON syntax, rather than schema
semantics or SDK compatibility; those still need review and protocol checks.

`TestSpecSyncRejectsFailedDownloads` executes the workflow's run block with real
curl against a local HTTP server, while branch and pull-request operations use
inert fixtures. It covers failures in each downloaded directory, valid bodies,
redirects and preservation of existing state.

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
| Image reference/inline alternatives, input provenance and typed failure | `ThreadItemWrapper` in history and item notifications | `TestSyncFea1ImageGeneration`, `TestSyncFea1ConstructedUnions`, `TestGeneratedImageFailureAdmission`, `TestImageGenerationAlternatives`, `TestImageGenerationAlternativesThroughCarriers`, `TestImageGenerationAlternativeHistory` | Preserve file references even when legacy inline data is also present; accept either valid alternative regardless of the other property’s shape; reject missing alternatives and malformed shared input or failure records |
| New references remain isolated in runtime views | Shared deep-copy boundary in cache, conversation, events and results | `TestEveryItemSnapshotOwnsReferences`, `TestRunResultOwnsAllItemAndTurnReferences`, `TestConversationCacheAndSnapshotItemOwnership`, `TestStreamEventsCollectorAndResultOwnership` | Exercise populated, empty and nil references and mutations in both directions |
| Retention, catalog metadata and new enums survive typed codecs | Turn start, MCP status, account and hook carriers | `TestSyncFea1Fields`, `TestSyncedFieldsThroughServices`, schema field and enum coverage | Preserve false/zero/empty/null metadata and numeric tokens |
| Local lint uses the same pinned version as CI | Hook resolver, PATH, GOBIN, first GOPATH entry and optional Nix delegation | `TestHookRunnerRequiresPinnedVersion` | Reject wrong versions before execution, verify installations, and retain the host-environment fallback without Nix |

The Responses API additions remain inside the existing raw `Thread.InjectItems`
carrier. `RawResponseItemCompletedNotification` is absent from the server-method
union and remains excluded as documented in `TestSpecCoverage`. The changed
`ManagedHooksRequirements` and realtime-voice definitions, and the new
`ThreadReadStateOperation` definition, have no referencing wire entry points in
this snapshot; they do not establish new SDK service methods.

## Schema snapshot 26ad281fd1ca

This snapshot adds feedback saving, item lifecycle times, image generation IDs,
process output-drain status, workspace account names, GCP account selection,
native usage cost, thread-scoped hooks and hook environment IDs. Hook run paths
are now opaque legacy strings, and review delivery permits only inline.
Managed OpenTelemetry requirements have typed exporter, protocol, TLS and
logging-limit representations.

| Contract | Owner and callers | Evidence | Decision |
| --- | --- | --- | --- |
| New nullable metadata and zero/false preservation | Account, usage, hook and process codecs | `TestSyncMetadataFields`, `TestSyncAccountAndUsageServices`, `TestSyncProcessDrainCallbacks` | Preserve supplied values; omitted nullable metadata remains unavailable |
| Flat item lifecycle times across variants | Item union and each concrete encoder, history/read and notifications | `TestSyncEveryItemLifecycle`, `TestSyncLifecyclePublicRoutes`, existing snapshot ownership tests | Preserve both common fields through value/pointer union forms and deep copies |
| Opaque hook execution paths | Run decoder and both hook listener APIs | `TestSyncLegacyHookPath`, `TestSyncOpaqueHookCallbacks` | Preserve spelling; configured hook metadata keeps its distinct absolute-path schema |
| Removed review delivery | Enum codec before request dispatch | `TestSyncRemovedReviewDelivery`, review and enum coverage tests | Retain the constant but reject its obsolete wire value |
| Feedback archive operation | Feedback service and required response admission | `TestSyncFeedbackSaveService`, request/type coverage | Send server path strings; leave server file lifecycle with the caller |
| Managed telemetry metadata | Requirements response and exporter union | `TestSyncOtelRetention`, `TestOtelExporterValidation`, `FuzzOtelExporterContract`, `TestSyncConfigOtelService` | Validate shapes before public success; perform no telemetry connections |

`AccountSession` and workspace/session definitions, application storage
requirements, network review domains and additional-context names have no
referencing wire entry points in this snapshot. Their presence in the combined
schema does not add SDK methods. New error-info strings already survive the
existing opaque `CodexErrorInfo` carrier. Unrecognized future item variants retain
lifecycle fields in their raw payloads. The known `hookPrompt` branch uses typed
fragments and lifecycle fields, with admission checks at the item union.
