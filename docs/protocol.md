# Protocol guide

`appserver/protocol` maps the checked-in Codex app-server schemas to Go. The schemas remain the source of truth; this page is a discovery guide for public APIs that are easy to miss in the package index.

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

Service methods receive typed Go parameters. Plain parameter structs retain
standard `encoding/json` decoding behavior: for example, decoding a null array
element into a fresh `[]string` produces an empty string before the SDK receives it.
The service cannot recover that distinction, and these path schemas permit empty
strings. Applications accepting raw JSON must validate its shape before that
conversion; typed request preparation is not a raw JSON schema validator.

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

`Thread.StartWithStateListener` couples a successful start response's cache
publication, generation and listener registration before eviction or callbacks.
It returns the response, that publication's generation and unsubscribe function;
initial callback diagnostics use `thread/start`. Request failures admit no new
subscription or publication, and both nil callbacks create no subscription.
Ordinary `Thread.Start` retains its existing signature and behavior. Runtime
Conversations use the coupled helper so cache pressure cannot discard the newly
started thread before observation, while a close during startup remains terminal
for the returned handle even if the cache is explicitly reopened afterward.

## Login variants

`Account.Login` accepts these typed parameter variants:

- `ApiKeyLoginAccountParams`
- `ChatgptLoginAccountParams`, including app-brand, streamlined-login, and hosted-success-page options
- `ChatgptDeviceCodeLoginAccountParams`
- `ChatgptAuthTokensLoginAccountParams`
- `AmazonBedrockLoginAccountParams` for the experimental managed Bedrock flow

Credential-bearing types redact secrets from their formatted and debug JSON representations. Raw JSON-RPC frames still contain wire credentials and must not be logged.

## Approvals

Register server-request handlers with `Client.SetApprovalHandlers`. A protocol-valid denial for legacy apply-patch and exec-command review decisions is an object containing a rejection reason:

```go
decision := codex.ReviewDecisionWrapper{
	Value: codex.DeniedReviewDecision{
		Rejection: "The command writes outside the approved workspace.",
	},
}
```

Use `ReviewDecisionApprovedMCPPolicyAmendment` for the MCP policy-amendment string decision. `item/tool/requestUserInput` requests require `isBlocking`; malformed requests are rejected before invoking the handler.

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

## Compatibility

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
