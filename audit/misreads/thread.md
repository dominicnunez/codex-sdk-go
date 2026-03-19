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


# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### ThreadStartParams.ApprovalPolicy bare interface marshaling flagged as new finding but already covered

**Location:** `thread.go:586` — ThreadStartParams.ApprovalPolicy field type
**Date:** 2026-02-27

**Reason:** The audit re-flagged the bare interface typing of `ApprovalPolicy` on params structs
as a new Medium-severity bug. The audit itself acknowledges "This finding is already covered by
the exception and is noted here for completeness — no new action required." The existing exception
"Params structs use bare interface instead of wrapper type" at `thread.go:538` et al. already
covers this exact issue. This is a duplicate, not a new finding.

### SessionSourceSubAgent round-trip serialization described as losing type discriminator

**Location:** `thread.go:263-277` — SessionSourceWrapper.MarshalJSON SubAgent case
**Date:** 2026-03-01

**Reason:** The finding claims `json.Marshal(v)` for `SessionSourceSubAgent` "may not produce the
correct wire format with type discriminators." This is incorrect. `SessionSourceSubAgent` has a
`json:"subAgent"` struct tag (thread.go:66), and `SubAgentSourceThreadSpawn` has a `json:"thread_spawn"`
struct tag (thread.go:89-95). Default `json.Marshal` produces `{"subAgent":{"thread_spawn":{...}}}`,
which matches the format expected by `UnmarshalJSON` (line 213 checks for key `"subAgent"`, line 243
checks for key `"thread_spawn"`). The round-trip is correct. This is also a duplicate of the known
exception "SessionSourceSubAgent relies on implicit marshaling for SubAgentSource variants."

### ThreadStartParams.ApprovalPolicy uses interface type described as code quality issue

**Location:** `thread.go:659` — ThreadStartParams.ApprovalPolicy field type
**Date:** 2026-03-01

**Reason:** Same duplicate as above. The known exception "Params structs use bare interface instead
of wrapper type" at `thread.go:538` et al. covers this field. `ThreadStartParams.ApprovalPolicy`
is explicitly listed in the exception's location set.

### Thread response methods return zero-value threads when required fields are missing

**Location:** `thread.go:40` — `Thread.UnmarshalJSON` and thread response validation

**Reason:** This behavior does not occur in the current worktree. `Thread.UnmarshalJSON`
rejects missing required thread fields such as `id`, `cliVersion`, `cwd`, `status`, and
`ephemeral`, and the thread response methods validate their decoded responses before returning
success. A payload like `{"thread":{}}` does not deserialize into a successful zero-value thread.
