### The subprocess suite already covers the SIGINT-to-exit-130 shutdown path

**Location:** `process_test.go:871` — `TestProcessCloseTreatsInterruptExitCode130AsExpected`

**Reason:** The current process integration tests already exercise the common
shell-handler path where `SIGINT` triggers cleanup and exits with status `130`.
The test initializes a fake child, traps `INT`, exits `130`, and asserts that
`Process.Close()` succeeds after the final notification is drained. The reported
testing gap is stale against the current suite.

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

### The StartProcess integration test does not use the obsolete initialize payload described in the audit

**Location:** `process_test.go:32` — `TestStartProcess` fake process response

**Reason:** The audited payload is no longer present. The checked-in fake process now returns
`platformFamily`, `platformOs`, and `userAgent` in the initialize result at `process_test.go:32`,
which satisfies the current validation logic. A fresh `go test ./...` passes in this checkout, so
the reported suite-breaking failure is stale and does not reflect the current code.
