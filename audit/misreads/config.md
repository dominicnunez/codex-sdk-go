# Notification Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale notification-validation findings.

### Config warning notifications already require a summary

**Location:** `config.go:497` — config warning unmarshaling

**Reason:** `ConfigWarningNotification` already implements `UnmarshalJSON` and
requires `summary` through `unmarshalInboundObject`. The same file also
validates nested `TextRange` and `TextPosition` fields. The existing tests in
`config_test.go` cover both direct unmarshaling failures and handler error
reporting for missing `summary`.


# Validation Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale response and notification validation findings.

### Config write responses already reject unknown status values

**Location:** `config.go:457` — `ConfigWriteResponse.UnmarshalJSON`

**Reason:** The current config write decode path validates `status` against the `WriteStatus`
enum before returning a successful response. Unsupported values are rejected during unmarshaling
for both `config/value/write` and `config/batchWrite`. The regression test
`TestConfigWriteRejectsInvalidStatus` covers both client methods.


# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Config warning notifications do not use AbsolutePathBuf for the optional path field

**Location:** `config.go:493` — `ConfigWarningNotification.UnmarshalJSON`

**Reason:** The audit grouped `ConfigWarningNotification.UnmarshalJSON` with inbound path decoders
that are backed by `AbsolutePathBuf`. That specific claim is wrong in the current specs.
`specs/v2/ConfigWarningNotification.json` defines `path` as `string | null`, not `AbsolutePathBuf`,
so there is no missing absolute-path validation requirement for this notification field.
