# Spec-sync maintenance

The `spec-sync` workflow checks the upstream Codex app-server schemas on its schedule and through manual dispatch. When it detects changes, it commits the schema snapshot to a bot-owned `spec-sync/*` branch and opens a pull request.

## Snapshot identity

The latest upstream commit affecting the schema path selects one snapshot for a
run. Its full commit SHA is checked before inventory requests; empty values,
branch names and abbreviated or malformed IDs stop the run. Root, v1 and v2
inventories all use that SHA as the Contents API `ref`. GitHub returns download
URLs containing the selected commit, so later upstream changes cannot switch
the file list or individual downloads to another revision.

The baseline records that full SHA, and branch names and pull-request metadata
use its matching short prefix. `TestSpecSyncPinsSnapshot` exercises upstream
advances after selection, after inventory selection and between file downloads,
including files added or removed between revisions and common files whose
contents change. The test verifies inventory requests, downloaded bytes and
publication metadata against the selected snapshot.

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

## Schema snapshot 16ddd538bdf2

This snapshot adds model-provider listing/configuration, file-backed audio input,
managed human checkpoints, thread-section changes and nullable metadata.

| Contract | Owner and callers | Evidence | Decision |
| --- | --- | --- | --- |
| Provider settings/auth/patch unions, exact field selection and published summaries | ModelProvider.List/ConfigWrite and their private admission carriers | `TestModelProviderListAdmission`, `TestModelProviderConfigWrite`, `TestModelProviderExactNamesAndMerges`, provider union/patch tests and `FuzzModelProviderOpaquePatch` | Validate the representation actually published; preserve patch omission/null and leave credential paths opaque |
| Audio URL/file-ID alternatives | UserInput codec, Turn.Start/Steer, history and item callbacks | `TestAudioInputAlternatives`, `TestAudioInputPublicRoutes`, `TestConstructedAudioInputAlternatives`, `FuzzAudioInputAlternatives` | Admit either independent string alternative, including empty identifiers and malformed inactive extras |
| Human capability and one-time decisions | Initialize normalization/equality and command approval response carrier | `TestHumanApprovalHandshakeIdentity`, `TestHumanCommandOneTimeDecisions` | Retain negotiated identity and reject broad decisions for humanCommand before response publication |
| Required nullable section destination and protected delivery | Typed listeners and stdio notification owner/reservations | `TestThreadSectionNotificationLifecycle`, `TestThreadSectionNotificationRoundTrip`, `TestStdioSectionNotificationRetention`, schema-wide ordering/pressure tests | Preserve null removal and exact thread identity; retain existing complete byte lifecycle |
| Completion tokens, account filtering, search provider and SSE logging | Item-completion callbacks, both list DTOs and config services | `TestCompletionContentTokens`, `TestCurrentAccountFilterRoutes`, `TestSyncedSearchAndLoggingMetadata`, `TestSearchProviderRestrictionOrder` | Preserve explicit zero/false and uint64 precision; reject invalid values before successful publication and preserve existing checks |

`ContentItem.input_audio` uses the raw Responses API `file_id` alternative through
`Thread.InjectItems`; `TestFileBackedRawResponseAudio` covers forwarding it.
Its nested definitions in `ThreadResumeParams` do not add a typed input property.
The separate dynamic-tool and function-call-output audio schemas are unchanged.
`QueuedSubmission` has no referencing wire entry point. `ModelProviderError`
describes optional structured JSON-RPC error data; the generic raw error and
unknown fallback remain available. Section notifications supply no destination
name or entry timestamp, so they do not synthesize cached thread metadata.


## Schema snapshot 9018f3a3b021

This snapshot adds provider-selectable gateway device authorization, account
security notices, agent-message display instructions and file-backed tool audio.

| Contract | Owner and callers | Evidence | Decision |
| --- | --- | --- | --- |
| Nullable gateway parameters, prompt and device capability | Account gateway read/login, both changed listeners | `TestGatewayDeviceFlowAndSecuritySetup`, `TestNestedSchemaRecordMerges` | Preserve existing methods and add parameter-taking methods; URLs and codes remain data |
| Optional account-bound notice and account user ID | SecuritySetupRead and Get responses | `TestGatewayDeviceFlowAndSecuritySetup`, `TestNestedSchemaRecordMerges` | Validate required notice/action members; preserve absent/null notices and empty strings |
| Agent moderation and display instructions | Private item admission, history, both item listener APIs, shared copies | `TestAgentModerationAdmission`, `TestAgentModerationPublicRoutesAndOwnership`, `TestNestedSchemaRecordMerges`, `FuzzAgentModerationMerges`, existing complete item/run/conversation/stream ownership tests | Preserve unknown actions and false hints; follow duplicate merges and null resets without adding methods to nested public records |
| Provider selection, metadata and output deferral | Model.List, Mcp.ResourceRead and Thread.ItemsList request codecs | `TestNewSchemaRequests`, `TestMcpRequestMetadataRejectsNonObjects` | Retain raw object values and explicit null; deferral object has no declared selectors |
| URL/file-ID function-output audio | FunctionCallOutputContentItemWrapper, Turn.Start and typed history | `TestFunctionAudioAlternatives`, `TestFunctionAudioPublicRoutes`, `FuzzFunctionAudioAlternatives`, complete reference-copy tests | Evaluate alternatives independently, including empty strings and malformed inactive properties |

`QueuedSubmission.responsesapiClientMetadata` has no referencing wire consumer.
The added `ThreadEnvironment` definition is likewise unreferenced by the actual
thread-settings notification. Neither creates a new SDK operation. The plugin
prompt change updates a prose limit from 128 to 512 characters; the schema adds
no length constraint, and the SDK had no old limit to remove.
`inferenceHookDenied` remains available through opaque `TurnError.CodexErrorInfo`
(`TestOpaqueInferenceHookError`). Raw Responses API audio already supports file
IDs through `Thread.InjectItems`; its nested definitions in resume parameters do
not add a new typed request property. `RawResponseItemCompletedNotification`
remains absent from the server-method union.
