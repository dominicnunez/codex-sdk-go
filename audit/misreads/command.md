### Command exec output delta notifications already reject unknown stream values

**Location:** `104`

**Reason:** The current notification unmarshal path validates `stream` with
`validateCommandExecOutputStream`, which only accepts `stdout` and `stderr`. Invalid values are
rejected during unmarshaling and do not reach registered handlers. The regression test
`TestCommandExecOutputDeltaInvalidStreamReportsHandlerError` in `command_test.go` exercises that
failure path.
