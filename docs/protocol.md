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

Legacy additional-filesystem permission paths may be relative and are preserved as supplied. Fields defined by the schema as absolute paths, including request working directories, continue to require normalized absolute values.

## Compatibility

Unknown discriminated-union variants are preserved where the public type exposes an `Unknown...` form. Outbound responses are validated more strictly because the SDK must not send a shape rejected by the current server schema.
