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
