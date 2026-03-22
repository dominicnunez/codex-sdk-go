### BinaryPath relies on design-by-contract without path validation

**Location:** `149-153`

**Reason:** `ProcessOptions.BinaryPath` is documented as "must be a trusted value — passed directly
to exec.CommandContext." The SDK is a library where callers control all inputs. Adding path
sanitization (no null bytes, no shell metacharacters) would be speculative defense against caller
misuse that exec.CommandContext already rejects with clear errors. The design-by-contract boundary
is at the public API: callers who set BinaryPath from untrusted input have a bug in their code,
not in the SDK. This matches Go standard library conventions where exec.Command accepts any string
and fails at execution time.
