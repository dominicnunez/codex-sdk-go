### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `442`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `450`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `470`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### ExecArgs safety does not depend on duplicate-flag parser precedence

**Location:** `41`

**Reason:** The current implementation rejects the `--` end-of-options marker and all typed safety
flags from `ExecArgs` before the CLI is ever spawned. That means `ExecArgs` cannot supply a second
`--model`, `--sandbox`, `--full-auto`, or `--config` flag for the parser to resolve, so the safety
boundary does not depend on a last-wins CLI contract. The remaining ordering test only verifies the
argv shape emitted after validation, and the related comments were tightened to reflect that design.

### ExecArgs safety does not depend on duplicate-flag parser precedence

**Location:** `181`

**Reason:** The current implementation rejects the `--` end-of-options marker and all typed safety
flags from `ExecArgs` before the CLI is ever spawned. That means `ExecArgs` cannot supply a second
`--model`, `--sandbox`, `--full-auto`, or `--config` flag for the parser to resolve, so the safety
boundary does not depend on a last-wins CLI contract. The remaining ordering test only verifies the
argv shape emitted after validation, and the related comments were tightened to reflect that design.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `442`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `450`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### Process shutdown already classifies SDK-initiated interrupt exits before surfacing wait errors

**Location:** `470`

**Reason:** The current process shutdown path records whether `Close()` sent an
interrupt or escalated to a kill, then classifies `p.waitErr` with
`isExpectedShutdownWaitError` before returning it from `processExitError`.
SDK-initiated interrupt exits are therefore not treated the same as unrelated
signal or nonzero exits, which is the distinction the report says is missing.

### The default minimal child environment already preserves Windows profile and app-data variables

**Location:** `110`

**Reason:** The current process startup code already uses OS-specific allowlists.
`defaultChildEnvKeysForGOOS("windows")` appends `APPDATA`, `LOCALAPPDATA`,
`USERPROFILE`, `SYSTEMROOT`, and related Windows variables to the shared minimal
environment, and `minimalChildEnv()` resolves through that helper at
`process.go:336-357`. The single cross-platform allowlist described in the report
is no longer present.

### The default minimal child environment already preserves Windows profile and app-data variables

**Location:** `351`

**Reason:** The current process startup code already uses OS-specific allowlists.
`defaultChildEnvKeysForGOOS("windows")` appends `APPDATA`, `LOCALAPPDATA`,
`USERPROFILE`, `SYSTEMROOT`, and related Windows variables to the shared minimal
environment, and `minimalChildEnv()` resolves through that helper at
`process.go:336-357`. The single cross-platform allowlist described in the report
is no longer present.

### Process.Wait claimed to have zero test coverage

**Location:** `191-198`

**Reason:** The audit claims "Process.Wait() has zero test coverage. No test calls Wait()."
This is factually wrong. `process_test.go` calls `proc.Wait()` at lines 97, 153, 245, and 333.
The Wait+Close race is untested, but the method itself is exercised in multiple tests.

### Config values passed to CLI args without sanitization described as security risk

**Location:** `89`

**Reason:** The audit claims config values concatenated into CLI args could allow shell metacharacter
injection or flag misinterpretation. This is incorrect. `exec.Command` does not invoke a shell — each
argument is passed as a discrete `argv` element, so shell metacharacters have no effect. The `--config`
flag and `k=v` value are passed as two separate arguments (not one), so the value cannot be
misinterpreted as a flag. The `=` ambiguity concern is already covered by the known exception
"Config flag values containing '=' are ambiguous on the CLI." The security framing is misleading
because `exec.Command` eliminates the actual attack vector.

### Config key=value concatenation allows parsing ambiguity

**Location:** `94`

**Reason:** Duplicate of existing exception "Config flag values containing '=' are ambiguous on the
CLI" at `process.go:87`. Both describe the same issue: `--config k=v` concatenation is ambiguous
when keys or values contain `=`. The existing exception already documents why this is a CLI-side
parsing concern and not an SDK defect. The additional suggestion to validate keys is a feature
request, not a bug.

### ExecArgs values described as needing shell metacharacter validation

**Location:** `84-113`

**Reason:** The finding itself acknowledges "`exec.Command` does not use a shell" and "This is safe."
The concern about the Codex CLI interpreting `--config "key=$(cmd)"` is speculative — `exec.Command`
passes each argument as a discrete `argv` element, so `$(cmd)` is a literal string, not a shell expansion.
The CLI's parsing of its own arguments is outside the SDK's responsibility. The finding concludes with
"The current `exec.Command` usage is safe against shell injection" — confirming no vulnerability exists.

### ensureInit holds mutex during blocking Initialize RPC described as new finding

**Location:** `232-246`

**Reason:** This is a duplicate of the known exception "ensureInit holds mutex across RPC
round-trip, serializing concurrent callers" which documents that replacing the mutex with a
`sync.Once`-like done channel requires non-trivial concurrency redesign for a one-time startup
path.

### ExecArgs flag bypass via space-separated value form described as a gap but no actual bypass exists

**Location:** `93-105`

**Reason:** The audit describes the "real gap" as future CLI aliases or short flags (e.g. `-m` for
`--model`) bypassing the check. This is speculation about future CLI changes, not an existing bug.
The current code correctly rejects all current flag forms. The finding's own suggested fix is
"Add a comment documenting this limitation" — a documentation suggestion, not a code defect.
Typed safety flags are always appended after ExecArgs with last-wins semantics, so even a missed
flag form would be overridden.

### Process.Close grace period and SIGKILL escalation claimed to be untested

**Location:** `197-228`

**Reason:** The audit claims "the test suite doesn't spawn a real subprocess" and "the isSignalError
helper is also untested." Both claims are factually wrong. `TestProcessCloseForceKill`
(process_test.go:472-512) spawns a real subprocess that traps SIGINT, calls `Close()`, and verifies
it completes within 10 seconds — exercising the SIGINT→grace period→SIGKILL path. `isSignalError`
has dedicated tests in `process_internal_test.go` covering nil error, non-ExitError, signal-killed
process, and normal exit cases.

### StartProcess does not forward child stderr to the parent by default

**Location:** `258-262`

**Reason:** The report says `StartProcess` replaces a nil `ProcessOptions.Stderr` with
`os.Stderr`. In the current code, `StartProcess` assigns `cmd.Stderr = opts.Stderr` directly and
the `ProcessOptions` comment states that nil discards child stderr output. The regression test
`TestStartProcessNilStderrDoesNotForwardToParent` in `process_test.go` already verifies that the
parent process does not receive child stderr unless the caller opts in.
