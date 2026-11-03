# Protocol guide

`appserver/protocol` maps the checked-in Codex app-server schemas to Go. The schemas remain the source of truth; this page is a discovery guide for public APIs that are easy to miss in the package index.

## JSON decoding and errors

The SDK uses the default `encoding/json` backend. Direct decoding, including
inside application envelopes, can partially update the destination before
returning an error and can visit later fields after a custom decoder fails.
Discard failed decode results. SDK service methods reject failed admission and
return zero responses; invalid metadata is not a successful response.

Native error details follow the supported Go runtime. Prefer `errors.As`,
`errors.Is` and structured fields over matching complete error strings.
Application codec errors retain their identity and chains. The [threat
model](threat-model.md#json-rpc-framing-dispatch-and-resource-exhaustion)
describes SDK diagnostic bounds and their limits.

`ServiceTier` and `ThreadSource` are open protocol strings. Their named constants
are conveniences rather than exhaustive lists; custom and empty strings retain
their spelling through requests, responses and thread state. A nil optional
pointer remains absent or null according to its carrier, while a present empty
string remains explicit. Standalone values require JSON strings. The distinct
`ThreadSourceKind` filter and other schema enums retain their closed validation.
The server interprets service tiers; accepting a name does not establish its
availability for a model or provider.

Service-tier overrides in thread start/fork/resume and turn start, goal token
budgets, and the `SHA`, `Branch` and `OriginURL` fields of Git metadata updates
use `OptionalNullable[T]` to preserve absent/null/value states. Its zero value
omits the field; `Present: true` with a nil `Value` emits explicit null, and a
nonnil `Value` supplies a replacement, including a zero budget or empty tier.
Omitted Git metadata leaves stored values unchanged, while null requests a
clear. A null goal budget requests removing the budget; omission keeps it.
The SDK preserves these requests without performing a server-side update locally.

`Config.Tools.WebSearch` contains a `WebSearchToolConfig` object with optional
allowed domains, a `low`/`medium`/`high` context size, and location metadata.
Empty objects and nullable members are supported; a nonnil empty domains slice
remains an empty array during serialization. Config admission rejects unknown
properties in the tool and location objects and null domain elements, including
overwritten occurrences. The separate `Config.WebSearch` mode and the legacy
`ToolsV2.ViewImage` flag retain their own meanings. The public tool and location
records keep ordinary JSON behavior when decoded outside Config.

## Transport termination

Custom transports can implement the optional `TransportLifecycle` capability.
Its stable `Done()` channel closes after notification admission ends and all
accepted notifications have finished dispatch or been discarded according to the
transport's shutdown contract. `Err()` supplies the terminal cause. Reader exit
alone is insufficient when accepted notifications still need delivery.

`Client.Done()` and `Client.Err()` expose that boundary to runtime helpers. A
matching accepted turn completion wins over transport termination. For transports
without the capability, `Client.Close()` rejects new client notification entries
and signals completion after already-entered dispatches retire; their autonomous
failure and internal queues cannot be observed through the required transport
interface. EOF does not disable ordinary stream backpressure or discard accepted
runtime replay. A stalled consumer must still drain, detach or cancel.

## Runtime turn outcomes

Run and Conversation helpers report terminal `failed` status as a Go error even
when the schema-valid completion omits `error` or sets it to null. They return no
result on failure; streams deliver the terminal `TurnCompleted` event before the
error and leave `Stream.Result()` nil. Detailed errors retain their existing
wrapping and can be inspected with `errors.As` into `*TurnError`. Missing details
produce an SDK error without inventing a protocol `TurnError`. The terminal turn
and collected items are still recorded in the matching cache incarnation and
Conversation history before the error is returned.

## Service map

The main service fields on `Client` include `Thread`, `Turn`, `Account`, `Apps`, `Skills`, and `ExternalAgent`. Recent protocol methods include:

| Service | Methods |
| --- | --- |
| `Apps` | `Read`, `Installed` |
| `Account` | `ConsumeRateLimitResetCredit`, `GetTokenUsage`, `GetWorkspaceMessages` |
| `Thread` | `Delete`, `GoalGet`, `GoalSet`, `GoalClear`, and the `SectionCreate`, `SectionDelete`, `SectionList`, `SectionUpdate`, and `SectionMove` methods |
| `Skills` | `SetExtraRoots` |
| `ExternalAgent` | `ImportHistories`, `RecordImportHistory` |

Parameters and responses are separate schema-shaped types. Optional fields use pointers when the wire protocol distinguishes omission from a zero value.

`OnThreadReadStateChanged` and `AddThreadReadStateChangedListener` receive durable
local read-state changes. `ThreadReadState.Revision` is opaque; `FirstUnread`
distinguishes omission, explicit null (read), thread start and a completed turn.
There is no read-state editing request in the current wire-method inventory.

Generated images use `ImageGenerationThreadItem`: `FileID` identifies a durable
attachment, while `Result` carries legacy inline data. Prefer `FileID` when both
are present. `InputImages` preserves absent versus present empty provenance lists;
paths and SHA-256 strings describe the peer's reported inputs and are not local
file verification. `TurnToolOutput.Retain` preserves an explicit false value.
MCP catalog `ToolListMeta` retains arbitrary JSON, including null and numeric tokens.

## Server path resolution

These schema-string path values pass unchanged to the server, which resolves
them in its own execution or configuration context:

| API | Path values |
| --- | --- |
| `Thread.Start`, `Thread.Resume`, `Thread.Fork`, `Thread.List`, `Turn.Start`, `Command.Exec` | `Cwd` |
| `Config.Read`, `PermissionProfile.List` | `Cwd` |
| `Config.Write`, `Config.BatchWrite` | `FilePath` |
| `Skills.List`, `Hooks.List` | `Cwds` |
| `FuzzyFileSearch.Search` | `Roots` |
| `ExternalAgent.ConfigDetect`, `ExternalAgent.ConfigImport` | `Cwds`, migration-item `Cwd` |

The SDK does not resolve these values using the application's working directory.
Returned skill grouping and migration-item cwd strings, `SkillErrorInfo.Path`, and
`Thread.Path` retain the peer's text, including relative values. Server permissions
and operation-specific restrictions still determine acceptance: resuming a running
thread can reject an override that
conflicts with its active configuration, and config writes remain restricted to
the server's user config file.

Fields declared as `AbsolutePathBuf`, such as Windows sandbox setup's cwd,
retain their existing SDK absolute-path checks. Legacy fields absent from the
current schemas retain their existing public representations and validation.

Initialization also checks the selected `codexHome` as an absolute, normalized
path before returning or caching the handshake. Invalid spellings fail without
latching a session, so a later valid response can be retried. The SDK preserves
accepted path text without checking existence or resolving symlinks.

Local skill icons (`SkillInterface.IconSmall` and `IconLarge`) are nullable
absolute, normalized paths. Skill listings and plugin responses, including
onboarding skills, validate them before successful publication. Their URL
siblings remain opaque strings; the SDK neither fetches icons nor resolves
local paths. `SkillInterface` retains ordinary JSON behavior when decoded
outside its containing skill records.

Service methods receive typed Go parameters. Plain parameter structs retain
standard `encoding/json` decoding behavior: for example, decoding a null array
element into a fresh `[]string` produces an empty string before the SDK receives it.
The service cannot recover that distinction, and these path schemas permit empty
strings. Applications accepting raw JSON must validate its shape before that
conversion; typed request preparation is not a raw JSON schema validator.

## Plugin availability

`PluginSummary.Availability` is a `PluginAvailability` enum value. An omitted
JSON member decodes as `PluginAvailabilityAvailable`; explicit values retain
`AVAILABLE` or `DISABLED_BY_ADMIN`. Null is rejected, including an earlier null
followed by a valid duplicate. This applies to `Plugin.List`,
`Plugin.Installed`, `Plugin.Read` and `Plugin.ShareList`. A zero Go field is
omitted when encoding, selecting the schema default on the next decode.

## Thread-list filters

`Thread.List` preserves its existing parameter fields. A nil `ModelProviders`
slice omits the provider filter; a nonnil empty slice sends `[]`, which asks for
all providers. Its scalar `Cwd` is sent unchanged. The server resolves relative
cwd filters using its own working directory, rather than the client's.

Use `Thread.ListJSON` with `ThreadListParamsJSON` for the complete checked-in
filter schema. It returns the same typed `ThreadListResponse` and uses the same
response validation and thread-state cache as `List`. Its `Cwd` raw JSON can be
absent, null, a string or an array of strings. The SDK checks that shape before
sending and preserves the path strings for server resolution.

Nullable fields use `OptionalNullable[T]`: `Present: false` omits a member;
`Present: true` with a nil `Value` sends null; a nonnil `Value` sends that value,
including empty arrays, empty strings, false and zero. In particular, omitted
`SectionID` includes all sections, null selects unsectioned threads, and a string
selects that section. Collection filters with a nonnil `Value` pointing to a nil
slice select an empty array; request preparation preserves that branch without
changing the caller's slice. `UseStateDbOnly` also uses `OptionalNullable[bool]`
to retain decoded presence, but `ListJSON` rejects an explicit null because the
schema permits only omission, false or true.

`ThreadListParamsJSON` also retains standard struct decoding behavior. Decode
each replacement request into a fresh zero-value DTO. Reusing a populated DTO
merges newly supplied members; omitted members keep their previous values.
For example, decoding `{}` into a populated DTO does not clear its filters.
The DTO has no JSON methods of its own, so it can be embedded in application
request envelopes without a promoted codec consuming sibling fields.

For example, select unsectioned threads across all providers and two cwd filters:

```go
providers := []string{}
page, err := client.Thread.ListJSON(ctx, protocol.ThreadListParamsJSON{
    Cwd: json.RawMessage(`["repo-a","repo-b"]`),
    ModelProviders: protocol.OptionalNullable[[]string]{
        Present: true,
        Value:   &providers,
    },
    SectionID: protocol.OptionalNullable[string]{Present: true},
})
```

Import `encoding/json` and `appserver/protocol` for those parameter values.
Keep the same filters when following `NextCursor` or `BackwardsCursor`.
The original `ThreadListParams` field types and JSON method set remain unchanged;
it cannot express cwd arrays or explicit null for `SectionID`.

## Inputs and thread items

`ThreadItemWrapper`, `PatchChangeKindWrapper`, and `WebSearchActionWrapper` accept
SDK variants as values or pointers. Both forms use the variant's wire codec,
including its discriminator, validation, and unknown-variant raw payload.
Use the wrapper when serializing a value variant; its concrete type may expose
`MarshalJSON` only through a pointer receiver.

`ThreadStatusWrapper`, `SessionSourceWrapper`, and `ReasoningSummaryWrapper` also
accept built-in values or pointers, including a pointer to a literal enum value.
Known nil pointers encode as null, matching a missing wrapper value. Unsupported
application-defined variants remain errors in these closed wire unions.

`UserInput` supports text, remote and local images, remote and local audio, skills, and mentions. For example:

```go
clientID := "client-message-1"

_, err := client.Turn.Start(ctx, codex.TurnStartParams{
	ThreadID:            threadID,
	ClientUserMessageID: &clientID,
	Input: []codex.UserInput{
		&codex.TextUserInput{Text: "Transcribe and summarize this recording."},
		&codex.LocalAudioUserInput{Path: "/absolute/path/to/recording.wav"},
	},
})
```

Dynamic-tool responses can return audio with `InputAudioDynamicToolCallOutputContentItem`. Thread responses and item notifications decode sub-agent activity and sleep items as `SubAgentActivityThreadItem` and `SleepThreadItem` rather than unknown items.

## Notifications

Typed `On...` methods set the handler for a notification. Typed `Add...Listener` methods append a listener and return an unsubscribe function:

```go
client.OnEnvironmentConnected(func(n codex.EnvironmentConnectionNotification) {
	fmt.Printf("thread %s connected to %s\n", n.ThreadID, n.EnvironmentID)
})

unsubscribe := client.AddThreadDeletedListener(func(n codex.ThreadDeletedNotification) {
	fmt.Println("deleted:", n.ThreadID)
})
defer unsubscribe()
```

The typed surface also covers thread reverts and queue changes, project updates, strict-review requirements, external-agent import progress, moderation metadata, and model safety-buffering changes. Use raw notification handlers only when intentionally handling a future method that the SDK does not yet type.

`TurnModerationMetadataJSONNotification.Metadata` is an owned `json.RawMessage`,
matching the schema's arbitrary JSON value. It preserves objects, arrays,
scalars, explicit null and numeric tokens without converting them to float64.
Metadata must be present when decoding. Consecutive duplicate metadata objects
merge their top-level members; null or another JSON kind resets the accumulated
object. Successful decoding replaces a reused receiver, and failed admission
leaves it unchanged. Each typed recipient receives its own metadata bytes.

Use `OnTurnModerationMetadataJSON` or `AddTurnModerationMetadataJSONListener`
for this complete JSON representation. The original
`TurnModerationMetadataNotification.Metadata` remains a `map[string]interface{}`
with its existing object/null decoding and float64 number behavior. Existing
constructors, map indexing and registration methods remain unchanged. Those
legacy methods can report errors for valid nonobject metadata or numbers outside
float64 range; applications needing the full schema should use the JSON methods.
Both replacement methods set the same handler for `turn/moderationMetadata`, so
the last registration replaces the previous one and passing nil clears it.
Appended listeners from either API coexist; a legacy decoding failure does not
prevent JSON listeners from receiving the notification.

To construct object metadata in the JSON type, marshal a map into its field.
To inspect it, decode the field into the application type appropriate to that value. Use
`json.Decoder.UseNumber` when decoding into an interface if numeric precision
must be retained. For example:

```go
var object map[string]json.RawMessage
if err := json.Unmarshal(n.Metadata, &object); err != nil {
	// This notification's metadata may be an array or scalar instead.
	return
}
```

Both notification types retain plain struct marshalling, so anonymous application
wrappers keep their additional fields. Their `UnmarshalJSON` methods
follow Go method promotion; use a named notification field when decoding an
outer application envelope. Constructed JSON metadata must contain valid JSON;
nil encodes as null, while a nonnil empty or malformed raw value fails encoding.

`AddThreadStateListener` receives independent cached snapshots and thread closure
updates, including an immediate replay when state is already cached. Each listener
panic is recovered separately and reported through `WithHandlerErrorCallback`;
later listeners still receive the committed update. The error context is the
originating RPC or notification method, or `CacheThreadState` for a direct cache
update and `AddThreadStateListener` for initial replay. A panic during replay
still returns the unsubscribe function. Callbacks run outside the cache lock;
unsubscribing affects future deliveries, while an already selected callback may
still run. Each recipient runs serially in cache commit order. Idle delivery runs
on the publishing call; concurrent or reentrant writes to a busy recipient can
return before delivery. Pending updates coalesce to the latest snapshot. Closure
supersedes older pending updates and precedes a later reopened snapshot; repeated
close/reopen transitions while busy may coalesce to one closure and the latest
update. Callbacks must return promptly, since an earlier blocked callback can
delay later recipients selected by the same publication.

`ThreadStateGeneration` checks the current
open incarnation, returning zero when absent or closed. `CompleteThreadTurn`
atomically appends to that incarnation's latest metadata, returning an independent
historical snapshot; closure, eviction or recreation causes publication to be
skipped. Each successful call appends once, without turn-ID deduplication. These
helpers support runtime snapshot ownership; generations are local best-effort
cache state, not remote revisions, durable identity or authorization.

`Thread.StartWithStateListener` starts observing before sending the request and
atomically transfers that observation to its returned generation and listener.
Notifications admitted while the request is pending take precedence over the
initial response in the cache. The returned response remains an independent
initial snapshot. Initial update diagnostics use `thread/start`; replayed closure
diagnostics use `thread/closed`. Both nil callbacks create no subscription.
Conversations use this helper so a close during startup remains terminal for the
returned handle even if the cache is explicitly reopened afterward.

Pending starts share a client-wide budget of 64 temporarily pinned thread
identities; each request retains at most 64 candidate records. A returned
identity outside the available budget, or ambiguous
same-ID start/close history without a correlating start, produces
`ErrThreadStartObservationLost`. Both `Thread.Start` and
`Thread.StartWithStateListener` then return the valid, owned initial response with
the error; the latter returns generation zero and a no-op unsubscribe function.
No listener or response-derived cache publication is created. The remote start
may have succeeded, so callers must not interpret this error as remote failure or
automatically retry. Runtime run, stream and Conversation helpers propagate the
error without dispatching a turn. Request, decoding and schema-admission failures
still return zero responses and leave no startup subscription or temporary pins.

## Login variants

`Account.Login` accepts these typed parameter variants:

- `ApiKeyLoginAccountParams`
- `ChatgptLoginAccountParams`, including app-brand, streamlined-login, and hosted-success-page options
- `ChatgptDeviceCodeLoginAccountParams`
- `ChatgptAuthTokensLoginAccountParams`
- `AmazonBedrockLoginAccountParams` for the experimental managed Bedrock flow

Credential-bearing types redact secrets from their formatted and debug JSON representations. Raw JSON-RPC frames still contain wire credentials and must not be logged.

## App metadata

App listings and app-list notifications default omitted `isEnabled` to true and
`isAccessible` to false. Explicit false/true values are preserved; recognized null
flags, including native folded aliases, reject the listing or typed notification
instead of selecting a default.

The same non-null rule applies to `supportsPersonality` in model metadata,
workspace-write configuration flags, granular approval's `request_permissions`
and `skill_approval`, and `workspaceWrite` sandbox flags. Omission retains each
field's default representation. Validation follows the owning decoder's field
matching and runs after its existing native, metadata and path checks.
`SandboxWorkspaceWrite` remains a plain representation; `Config` owns its
admission checks and retains its documented partial updates on decoding errors.

`SandboxPolicyReadOnly.NetworkAccess` carries the schema's optional, non-null
boolean. Nil preserves omission and the server's false default; a nonnil pointer
preserves explicit true or false through thread responses, settings callbacks
and outgoing turn/command policies. The legacy `Access` member stays optional.
The record keeps ordinary Go JSON behavior; `SandboxPolicyWrapper` owns strict
admission, including rejection of null and wrong-type recognized occurrences.

## Permission profiles

Permission profile listings require both `id` and a non-null `allowed` boolean
for each entry. An explicit false value is preserved; missing or null flags
reject the response rather than becoming an implicit false value.

## Approvals

`ApprovalPolicyOnFailure` retains its exported name and value, but encoding that
known removed policy returns an error before typed thread/turn request dispatch.
Current literals and granular policies remain supported. Approval-policy string
decoding still preserves unknown, empty and legacy literals; re-encoding the
specific removed `on-failure` literal fails. Explicit raw unknown variants keep
their existing forwarding behavior.

Register server-request handlers with `Client.SetApprovalHandlers`. A protocol-valid denial for legacy apply-patch and exec-command review decisions is an object containing a rejection reason:

```go
decision := codex.ReviewDecisionWrapper{
	Value: codex.DeniedReviewDecision{
		Rejection: "The command writes outside the approved workspace.",
	},
}
```

Use `ReviewDecisionApprovedMCPPolicyAmendment` for the MCP policy-amendment string decision. `item/tool/requestUserInput` requests require `isBlocking`; malformed requests are rejected before invoking the handler.

User-input questions default omitted `isSecret` and `isOther` flags to false.
Declared flag values must be non-null booleans; an invalid question rejects the
whole request before invoking the approval handler. Options retain their
nullable array representation.

The deprecated `AutoResolutionMs` field is a nullable unsigned 64-bit value.
Omission or null means no value; a present zero remains explicit, and the full
`uint64` range is retained without a floating-point conversion. Negative or
out-of-range recognized values reject the request before the handler, including
invalid earlier duplicates. Use `IsBlocking` to decide whether a request blocks.

For `mcpServer/elicitation/request`, standard `form` mode validates the typed
`requestedSchema`, including its property definitions, before invoking the
handler. The `openaiForm` and `openai/form` modes retain arbitrary JSON in
`OpenAIRequestedSchema`. Schema admission does not validate answers or decide
whether the application should collect the requested information.

Legacy additional-filesystem permission paths and approval cwd, command-action,
parsed-command and grantRoot strings are preserved as supplied. They may be
relative, empty or contain dot segments. The SDK does not resolve them against
cwd or substitute a normalized path before invoking an approval handler. Parsed
command paths are best-effort metadata; applications may derive a separate path
for display or policy without replacing the original request evidence.

Fields defined as AbsolutePathBuf, such as typed thread cwd and filesystem watch
paths, retain their individual absolute-path validation. Approval handlers own
authorization and any interpretation of opaque strings; schema-valid metadata is
not an approval or proof that the server resolves a path in a particular way.

`attestation/generate` requires an object before the application callback runs.
Empty objects and open extension metadata remain valid; null parameters reject
with `ErrInvalidParams` and the transport's invalid-params reply under the original
request ID. Rejection produces no token result and leaves the connection usable.
The public empty parameters record remains method-free for ordinary application
JSON. Existing handler errors and non-empty-token response validation still apply.

Command approval input arrays reject null string entries before application
handlers run, including earlier entries hidden by duplicate properties. This
covers the current `proposedExecpolicyAmendment` and legacy `command` arrays.
The current optional amendment may be absent or null; the legacy command array
remains required and non-null. Empty arrays, empty arguments and opaque strings
retain their values. Field matching follows each existing approval owner and
validation does not resolve or normalize approval evidence.

## Compatibility

Inbound stdio error records require non-null `code` and `message` members.
Explicit code zero and an empty message remain valid. Missing members or a null
recognized occurrence, including one hidden by a later duplicate, use the existing
correlated parse-error path (`-32700`, `failed to parse server response`) with no
peer error data. Field matching retains native aliases and escaped names; valid
duplicates and unconstrained data keep their selected values. The enclosing
frame's final error-field selection remains unchanged. The public `Error` record
retains ordinary JSON behavior, including scalar null-no-clear semantics.

Collaboration metadata requires non-null `mode`, `settings` and `settings.model`
before `Thread.Resume` publishes a response or thread-settings callbacks run.
An empty model string remains valid, as do nullable developer instructions and
reasoning effort. Resume allows an absent or null collaboration mode; the required
thread-settings member does not. Partial duplicate objects retain native merging,
and a nullable resume reset starts fresh presence without erasing forbidden nested
null occurrences. Matching follows each existing owner: native aliases inside the
plain records and at the resume root, exact matching at the thread-settings root.
The collaboration records remain method-free for ordinary application JSON and
constructed turn requests. Existing lifecycle response errors keep their priority.

Saved `disabledPluginIds` metadata defaults to an empty list when omitted.
Supplied arrays and their string elements must be non-null, including earlier
duplicate occurrences; empty-string identities remain valid. Validation uses the
settings owner's exact property matching before callbacks or cache updates. These
saved IDs are descriptive metadata and do not enforce plugin capabilities.

`Config.ReadRequirements` validates the six structured raw fields
`allowedPermissionProfiles`, `autoReview`, `browserUse`, `feedback`, `models` and
`modelProviders` before returning success. Known nested constraints include
non-null boolean map values, origin-policy enums, complete request headers and
non-empty reasoning-effort strings. Nullable values and open properties remain
valid; provider-map values are unconstrained. Admission checks every recognized
occurrence without rewriting retained raw JSON. Outer field matching follows the
existing Go records, while property names inside raw objects match the schema
exactly. Public requirements records retain ordinary JSON behavior. These checks
validate reported metadata; they do not enforce managed policy locally.

`skills/changed` and `account/updated` require object payloads on both typed
listener APIs, even though every declared member is optional. Empty objects and
unconstrained extra metadata remain valid, as do null account field values. A
null payload rejects with method-specific diagnostics before either typed callback.
Raw subscriptions keep their existing payload behavior. The empty skills record
remains method-free for ordinary application JSON and embedding.

Typed app label/icon maps, configuration feature requirements, configured browser
origin policies and feature-enablement results reject null map entries before
successful response or typed notification publication. Earlier null entries remain
invalid after duplicate entries, maps or nullable resets. Nullable whole maps stay
supported where declared; feature-enablement results require a non-null map.
Empty maps, empty strings, explicit false and empty origin-policy objects retain
their native merge/selection behavior. Dynamic keys remain opaque. Public nested
configuration records stay method-free; direct `Config` decoding retains its
established partial-update behavior on errors, while service calls return zero
results on failure.

Nullable enum metadata is checked before publication: login-completion
`onboardingEntrypoint` and MCP startup `failureReason` at their notification
owners, and `cliAuthCredentialsStore`, `allowedLoginMethods` and
`allowedWindowsSandboxImplementations` at `Config.ReadRequirements`. Unsupported
values cannot hide behind later valid duplicates or nulls. Requirements arrays
may be absent, null or empty, but their elements must be declared non-null enum
strings; an empty login-method list still means no method is permitted. Public
string/raw representations and existing matching rules remain unchanged.

Config metadata exposes typed `PackagedDefaultsConfigLayerSource` and
`EnterpriseManagedConfigLayerSource` values. Packaged defaults require an absolute,
normalized file path; enterprise IDs and names require non-null strings and may be
empty. These checks cover config origins, included layers and write override
metadata. Only exact schema property names, including escaped spellings, supply
these source fields; differently cased extras cannot replace them. Returned paths
are checked lexically without resolution or filesystem
access. Rejection of metadata from a completed write does not roll back that write.
Future source types retain owned raw JSON, and other branches keep unrelated
extras unconstrained.

`HookPromptThreadItem` exposes hook-generated prompt fragments through
`HookPromptFragment` records and retains optional nullable lifecycle times.
The item union validates required IDs, fragment arrays and fragment strings before
responses, typed callbacks or caches publish them. Empty strings and arrays remain
valid. Repeated fragment arrays follow native record reuse, including shrink/regrow
and empty-array resets, without hiding forbidden null occurrences. Fragment records
remain method-free; future unknown item types retain owned raw JSON.

Required non-null thread, plugin and sharing metadata rejects every canonical
null occurrence, including escaped property names, even when a later duplicate
has a valid value. The same checks cover required fields inside repeated granular
approval and thread-spawn objects. Earlier partial objects may still omit members;
valid duplicate selection, nullable fields and open metadata retain their existing
behavior. Initialize validates required response occurrences before caching a
successful handshake, so a rejected response can be retried. Its public response
record remains method-free; handshake admission owns these checks.

`Plugin.ShareUpdateTargets` supports `LISTED`, `UNLISTED` and `PRIVATE` through
`PluginShareUpdateDiscoverability`, including `PluginShareUpdateDiscoverabilityListed`. Target
identities, principal types and target roles are checked before dispatch. The
server decides whether the caller may change sharing and applies visibility.

Async agent-message questions allow only `title` and `options`. Undeclared
question properties, including differently cased names, fail at the shared
question decoder before response, notification or cache publication. Existing
title/option validation, nullable options, empty strings and valid duplicate
selection remain unchanged. Surrounding agent messages and notifications retain
their allowed extension metadata.

Known object alternatives for session sources (`custom`, `subAgent`), sub-agent
sources (`other`, `thread_spawn`) and approval policies (`granular`) have closed
outer shapes. Extra properties or competing branch keys fail before successful
response or typed notification publication. Nested `thread_spawn` and `granular`
records remain open to additional metadata. Future root source objects still use
`UnknownSessionSource` with owned raw JSON. `SessionSourceCustom` stays a plain
record; its union wrapper owns admission rather than a promoted JSON method.

Shared account metadata validates every recognized plan, authentication-mode,
and rate-limit-reason enum occurrence at its record owner. Valid nullable resets
remain supported; a later value or null cannot erase an earlier invalid enum.
This covers account reads, rate-limit snapshots and buckets, and account update
notifications. Unknown account branches retain their raw metadata.

ChatGPT account `email` is required and nullable. `ChatgptAccount.Email` uses a
`*string`: nil encodes as an explicit null, and a nonnil pointer preserves an
empty or populated string. Account reads reject a missing email key.

Reset-credit outcomes, plugin-install auth policies, and the status fields of
thread-unsubscribe, login-cancel, add-credits-email and configuration-write
responses validate every recognized enum occurrence.
A later valid duplicate cannot repair an earlier invalid value. Existing error
precedence and each record's field-name matching remain unchanged.

The `process/outputDelta` and `command/exec/outputDelta` decoders validate every
recognized `stream` occurrence, including escaped field names. A later valid
duplicate cannot hide an invalid earlier stream. Existing field and base64
errors retain their precedence, and valid duplicates retain their final value.

`PluginListMarketplaceKindVertical` selects the schema-supported `vertical`
marketplace kind. `Plugin.List` forwards it unchanged, including in mixed
marketplace-kind filters.

`PluginListParams.MarketplaceKinds` distinguishes a nil slice (omit the filter
and use server defaults) from a nonnil empty slice (send `[]` and select no
marketplace kinds). This applies to direct JSON encoding and `Plugin.List`.

`RemoteControlStatusChangedNotification` accepts only `disabled`, `connecting`,
`connected`, and `errored` states before typed callback delivery. The status
remains required and non-null. Validation belongs to the notification record;
`RemoteControlConnectionStatus` retains ordinary string decoding outside it.

Known thread items validate agent-message `delivery`, command-execution `source`,
and sub-agent activity `kind` against the checked-in enums before publication.
Agent-message delivery remains optional and nullable. An omitted command source
decodes as `agent`, matching the pinned server default; explicit null is rejected.
Code that previously tested `CommandExecutionThreadItem.Source == nil` to detect
older records should now treat the decoded default as an agent command. These
checks belong to `ThreadItemWrapper`; public leaf and scalar types keep their
existing JSON method sets for application composition.

Guardian review records validate `userAuthorization` as `unknown`, `low`,
`medium`, or `high`; it may also be absent or null. Completed review records
require `decisionSource` to be `agent`. These checks run at the notification
and review record decoders before typed callback delivery. The public scalar
types retain ordinary string decoding when used outside those records.

`ItemGuardianApprovalReviewStartedNotification.TargetItemID` and
`ItemGuardianApprovalReviewCompletedNotification.TargetItemID` now use `*string`
to match the optional nullable schema field. Check for nil before reading an
item ID, and use `protocol.Ptr("item-id")` when constructing a notification.
Nil represents no target (including network policy reviews) and serializes as
null; `protocol.Ptr("")` preserves an explicitly empty string. This is a breaking
Go API correction in the `appserver/protocol` notification types.

Custom session sources decode as `SessionSourceCustom{Custom: name}` rather than
`UnknownSessionSource`. The `custom` member must be a non-null string; an empty
string is valid. Construct the value directly or place it (by value or pointer)
in `SessionSourceWrapper.Value`. Code that inspected custom source names through
`UnknownSessionSource.Raw` should switch to the typed branch. Genuinely unknown
source objects continue to use `UnknownSessionSource`. The wrapper owns this
validation; `SessionSourceCustom` remains a plain struct so applications can
embed it in their own JSON envelopes.

### Thread-spawn source metadata migration

`SubAgentSourceThreadSpawn.ThreadSpawn` now matches the checked-in `SubAgentSource`
schema. `AgentPath` is available as `*string`; `AgentNickname` and `AgentRole`
change from `string` to `*string`, and `Depth` changes from `uint32` to `int32`.
These are breaking Go API corrections; the server's JSON shape is unchanged.
Missing or null metadata decodes to nil and encodes as null, matching the pinned
server's defaults. A pointer to an empty string retains an explicit empty value.
Agent paths are opaque protocol strings, not local filesystem paths.

Before:

```go
source.ThreadSpawn.AgentNickname = "worker"
source.ThreadSpawn.AgentRole = "researcher"
```

After:

```go
var source protocol.SubAgentSourceThreadSpawn
source.ThreadSpawn.AgentNickname = protocol.Ptr("worker")
source.ThreadSpawn.AgentRole = protocol.Ptr("researcher")
source.ThreadSpawn.AgentPath = protocol.Ptr("worker/alpha")
source.ThreadSpawn.Depth = 1
source.ThreadSpawn.ParentThreadID = "parent"
```

Check pointers before reading optional metadata; nil and `protocol.Ptr("")` have
different meanings. Existing code assigning an entire anonymous `ThreadSpawn`
record must include the new field and updated types, or switch to field-by-field
assignment as above. If converting a `uint32` depth variable, check that it is at
most `math.MaxInt32` before converting to `int32`. The schema's signed range also
permits negative values. These field changes apply through the public `appserver`
type alias too.

Unknown discriminated-union variants are preserved where the public type exposes an `Unknown...` form. Outbound responses are validated more strictly because the SDK must not send a shape rejected by the current server schema.

## Synced metadata and feedback archives

Thread items carry optional `StartedAtMs` and `CompletedAtMs` lifecycle times
independently of item-notification envelope times. These fields survive history,
typed notifications and runtime snapshots. Image-generation items also expose
`GenerationID`. Unknown item branches continue to retain their raw payloads.

`Hooks.List` accepts an optional `ThreadID` to inspect a loaded thread's hooks;
omit `Cwds` in this mode. Hook metadata exposes the owning `EnvironmentID`.
Hook execution summaries preserve `SourcePath` as an opaque server string.
Configured hook metadata retains its absolute-path contract.

New nullable observation metadata uses the usual SDK pointer representation:
omitted and null both mean unavailable; explicit false, zero and empty strings
remain distinguishable from that unavailable state.

`ProcessExitedNotification.OutputDrainTimedOut` reports incomplete output
collection after process exit. Nil means unavailable; false is a reported
negative result. Account metadata exposes the selected ChatGPT `WorkspaceName`
and typed `GcpAccount`. `AmazonBedrockAccount.UsesCodexManagedCredentials`
reports the server's Bedrock credential-management mode, defaulting to false when
omitted. Present values must be booleans; null or a wrong JSON kind fails before
`Account.Get` returns success. Unknown future account types retain owned raw JSON,
and the Bedrock flag remains an unconstrained extra on other account branches.
Account thread usage exposes `NativeUsageUSDMicros`;
nil means unknown and a pointer to zero means known zero.

`Config.ReadRequirements` exposes managed `OtelRequirements`. `OtelExporter`
selects one of `Kind` (`none` or `statsig`), `HTTP` or `GRPC`; TLS strings remain
opaque server metadata. These records do not activate local telemetry.

`Feedback.Save` sends `FeedbackSaveParams` to `feedback/save` and returns the
server's saved path, thread ID and optional prompt hash. The caller owns the ZIP
on the server host, including after an interrupted RPC. The SDK preserves server
path strings without local resolution.

Reviews run inline. The retained `ReviewDeliveryDetached` constant is rejected
by encoding and decoding because it is absent from the current schema.
