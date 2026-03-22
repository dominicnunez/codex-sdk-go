### The realtime started notification tests already include the required version field

**Location:** `18`

**Reason:** The current test fixtures already include `"version"` in both direct
unmarshal cases and in the listener-dispatch payload. The production type also
requires `threadId` and `version` in `ThreadRealtimeStartedNotification.UnmarshalJSON`
at `realtime.go:25-33`, so a missing-version fixture would fail immediately, but
that stale fixture is not present in this tree. A focused run of `go test -count=1
-run '^TestThreadRealtimeStartedNotification$' ./...` passes.
