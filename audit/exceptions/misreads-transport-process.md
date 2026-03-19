# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Reason:** Explanation (can be multiple lines)

### Server responses no longer spoof local transport failures in Client.Send

**Location:** `client.go:336` — response error handling

**Reason:** The current `Client.Send` implementation no longer inspects
wire-level `error.data` transport metadata. When a response contains an error,
it always returns `NewRPCError(resp.Error)` at `client.go:337-338`, so a server
cannot forge `{"transport":"failed","origin":"client"}` and have the client
reclassify it as a local `TransportError`.

### The client test suite no longer locks in forged transport metadata as a TransportError

**Location:** `client_test.go:257` — forged transport metadata regression test

**Reason:** The checked-in test now verifies the opposite behavior. The test at
`client_test.go:257-291` is `TestClientSendForgedTransportFailureResponseReturnsRPCError`,
and it asserts that a forged wire payload remains an `RPCError` with the original
error code, message, and data intact.

### The default minimal child environment already preserves Windows profile and app-data variables

**Location:** `process.go:110` and `process.go:351` — OS-specific child env allowlists

**Reason:** The current process startup code already uses OS-specific allowlists.
`defaultChildEnvKeysForGOOS("windows")` appends `APPDATA`, `LOCALAPPDATA`,
`USERPROFILE`, `SYSTEMROOT`, and related Windows variables to the shared minimal
environment, and `minimalChildEnv()` resolves through that helper at
`process.go:336-357`. The single cross-platform allowlist described in the report
is no longer present.

### Minimal-environment coverage already verifies required baseline variables and Windows-specific allowlists

**Location:** `process_test.go:70`, `process_test.go:561`, and `process_internal_test.go:100`

**Reason:** The current tests already cover both the required baseline variables
and the Windows-specific allowlist behavior. `requiredMinimalEnvForRuntime` in
`process_test.go:70-86` defines the expected runtime baseline, and
`TestStartProcessMinimalEnvByDefault` in `process_test.go:561-589` asserts that
those variables survive process startup while unrelated secrets do not. The
platform-specific env builder is also covered directly by
`TestDefaultChildEnvKeysForGOOSUsesPlatformSpecificAllowlist` and
`TestMinimalChildEnvForGOOSUsesPlatformAllowlist` in
`process_internal_test.go:100-205`.
