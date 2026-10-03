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

## Inputs and thread items

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

Unknown discriminated-union variants are preserved where the public type exposes an `Unknown...` form. Outbound responses are validated more strictly because the SDK must not send a shape rejected by the current server schema.
