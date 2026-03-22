### Config warning notifications already require a summary

**Location:** `497`

**Reason:** `ConfigWarningNotification` already implements `UnmarshalJSON` and
requires `summary` through `unmarshalInboundObject`. The same file also
validates nested `TextRange` and `TextPosition` fields. The existing tests in
`config_test.go` cover both direct unmarshaling failures and handler error
reporting for missing `summary`.

### Config write responses already reject unknown status values

**Location:** `457`

**Reason:** The current config write decode path validates `status` against the `WriteStatus`
enum before returning a successful response. Unsupported values are rejected during unmarshaling
for both `config/value/write` and `config/batchWrite`. The regression test
`TestConfigWriteRejectsInvalidStatus` covers both client methods.

### Config warning notifications do not use AbsolutePathBuf for the optional path field

**Location:** `493`

**Reason:** The audit grouped `ConfigWarningNotification.UnmarshalJSON` with inbound path decoders
that are backed by `AbsolutePathBuf`. That specific claim is wrong in the current specs.
`specs/v2/ConfigWarningNotification.json` defines `path` as `string | null`, not `AbsolutePathBuf`,
so there is no missing absolute-path validation requirement for this notification field.
