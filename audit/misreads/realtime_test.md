# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### The realtime started notification tests already include the required version field

**Location:** `realtime_test.go:18` — `TestThreadRealtimeStartedNotification`

**Reason:** The current test fixtures already include `"version"` in both direct
unmarshal cases and in the listener-dispatch payload. The production type also
requires `threadId` and `version` in `ThreadRealtimeStartedNotification.UnmarshalJSON`
at `realtime.go:25-33`, so a missing-version fixture would fail immediately, but
that stale fixture is not present in this tree. A focused run of `go test -count=1
-run '^TestThreadRealtimeStartedNotification$' ./...` passes.
