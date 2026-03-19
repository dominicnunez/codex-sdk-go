### Process.Close discards signal and kill errors during graceful shutdown

**Location:** `process.go:153-164` — Signal and Kill error handling in Close
**Date:** 2026-02-28

**Reason:** `Close()` discards errors from `Signal(os.Interrupt)` and `Process.Kill()` with `_ =`.
The transport close error takes priority (returned to caller). Signal errors are typically
`os.ErrProcessDone` (process already exited) which is not an error condition. Kill errors after
a grace period timeout are rare and non-actionable — the process is being force-terminated.
Surfacing these would require a structured multi-error return or a callback, which is
disproportionate to the severity. The caller cares whether cleanup succeeded (transport closed),
not about the intermediate signaling steps.

### Config flag values containing "=" are ambiguous on the CLI

**Location:** `process.go:87` — buildArgs config flag construction
**Date:** 2026-02-28

**Reason:** Config flags are constructed as `--config k=v`. If the value itself contains `=`,
the CLI may split on the first `=` and misinterpret the value. This is a CLI-side parsing
concern — the SDK faithfully passes what the caller provides. The CLI's `--config` flag parser
determines how `key=a=b` is interpreted, and the standard convention (split on first `=`) handles
this correctly. Adding quoting or escaping in the SDK without knowing the CLI's parser would be
speculative and could introduce new ambiguity.

### ensureInit holds mutex across RPC round-trip, serializing concurrent callers

**Location:** `process.go:192-206` — ensureInit
**Date:** 2026-03-01

**Reason:** `ensureInit` holds `initMu` across the `Initialize` RPC call. Concurrent
`Run`/`RunStreamed` callers serialize behind this lock, and if the first caller's context
expires mid-init, subsequent callers must retry. Replacing the mutex with a `sync.Once`-like
done channel requires careful error-retry semantics (the current design deliberately retries
on failure by keeping `initDone` false). The serialization only affects the very first call
on a fresh Process — after `initDone` is latched, the lock is held for a single boolean
check. The risk is low and the fix requires non-trivial concurrency redesign for a one-time
startup path.

### Process.Close transport error propagation lacks a unit test

**Location:** `process.go:156-187` — Close transport.Close error path
**Date:** 2026-03-01

**Reason:** Testing that `transport.Close()` errors propagate through `Process.Close()`
requires injecting a mock transport into the unexported `transport` field. The field is
unexported by design (callers should not interact with the transport directly).
Exporting it or adding a test-only constructor solely for this test adds public API surface
for a Low-severity testing gap. The code path is trivial (one `if err != nil` assignment)
and is exercised indirectly by integration tests with real processes.

### ExecArgs flag rejection does not cover abbreviated or prefix flags

**Location:** `process.go:96-103` — buildArgs flag rejection loop
**Date:** 2026-03-01

**Reason:** The rejection logic already blocks `--model`, `-model`, and `=` variants. Prefix-matching
(e.g. rejecting any arg starting with `--mod`) would create false positives for legitimate flags
that share a prefix. The primary mitigation is last-wins ordering: typed safety flags are always
appended after ExecArgs, so even if an abbreviated form slips through, the typed value takes
precedence. This holds as long as the downstream CLI parser uses last-wins semantics, which is
the standard convention and is documented as an assumption.
