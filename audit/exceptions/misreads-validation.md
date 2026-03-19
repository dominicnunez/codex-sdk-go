# Validation Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale response and notification validation findings.

### Thread unsubscribe responses already reject unknown status values

**Location:** `thread.go:1378` — `ThreadUnsubscribeResponse.UnmarshalJSON`

**Reason:** The current unmarshal path does not accept arbitrary status strings. It calls
`validateThreadUnsubscribeStatus`, which only allows `notLoaded`, `notSubscribed`, and
`unsubscribed`, and returns an error for anything else before the response reaches callers. The
regression test `TestThreadUnsubscribeRejectsInvalidStatus` in `thread_test.go` also covers the
invalid-enum path.

### Command exec output delta notifications already reject unknown stream values

**Location:** `command.go:104` — `CommandExecOutputDeltaNotification.UnmarshalJSON`

**Reason:** The current notification unmarshal path validates `stream` with
`validateCommandExecOutputStream`, which only accepts `stdout` and `stderr`. Invalid values are
rejected during unmarshaling and do not reach registered handlers. The regression test
`TestCommandExecOutputDeltaInvalidStreamReportsHandlerError` in `command_test.go` exercises that
failure path.

### Approval request enums already reject unsupported protocol and refresh reason values

**Location:** `approval.go:568` — `NetworkApprovalContext.UnmarshalJSON`; `approval.go:1263` — `ChatgptAuthTokensRefreshParams.UnmarshalJSON`

**Reason:** The current approval request decoding path already validates both constrained enums
before handlers see them. `NetworkApprovalContext.UnmarshalJSON` rejects unsupported
`NetworkApprovalProtocol` values, and `ChatgptAuthTokensRefreshParams.UnmarshalJSON` rejects
unsupported `ChatgptAuthTokensRefreshReason` values. The regression tests
`TestNetworkApprovalContextRejectsInvalidProtocol` and
`TestChatgptAuthTokensRefreshParamsRejectInvalidReason` cover both invalid-enum paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `approval.go:199`, `approval.go:583`, `approval.go:947`, `approval.go:978`, `approval.go:1284`, `approval_additional.go:62`, `approval_additional.go:157`, `client.go:584`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Config write responses already reject unknown status values

**Location:** `config.go:457` — `ConfigWriteResponse.UnmarshalJSON`

**Reason:** The current config write decode path validates `status` against the `WriteStatus`
enum before returning a successful response. Unsupported values are rejected during unmarshaling
for both `config/value/write` and `config/batchWrite`. The regression test
`TestConfigWriteRejectsInvalidStatus` covers both client methods.
