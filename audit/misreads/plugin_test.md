# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Plugin validation tests are stale and currently break the default test suite

**Location:** `plugin_test.go:290` — `TestPluginRequiredFieldValidation`

**Reason:** The audit is stale against the current test file. The checked-in assertions at
`plugin_test.go:327-354` already expect `required field "appsNeedingAuth"` and
`required field "authPolicy"`, not the legacy `missing appsNeedingAuth` / `missing authPolicy`
strings described in the report. The described red-suite behavior does not occur in this checkout:
`go test -run TestPluginRequiredFieldValidation ./...` and `go test ./...` both pass.

### Required-field validation is not limited to thread/start

**Location:** `thread_test.go:197`, `plugin_test.go:290` — response validation regression coverage

**Reason:** The current test suite already exercises missing required fields beyond
`Thread.Start`. `TestThreadResponsesRejectMissingRequiredThreadFields` covers
`Thread.Read`, `Thread.Resume`, `Thread.MetadataUpdate`, and `Thread.Unarchive`,
and `TestThreadResponseRequiredFieldValidation` checks missing required nested
thread fields across those responses. `TestPluginRequiredFieldValidation` does
the same for `Plugin.Read` and `Plugin.Install`.
