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
