# Notification Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale notification-validation findings.

### Realtime notification payloads are already validated before handlers run

**Location:** `realtime.go:25` — realtime notification unmarshaling

**Reason:** The current tree already validates `thread/realtime/closed`,
`thread/realtime/error`, and `thread/realtime/itemAdded` with custom
`UnmarshalJSON` methods that call `unmarshalInboundObject`. Malformed payloads
do not reach application handlers. The regression coverage in
`realtime_test.go` also includes missing-required-field handler error tests for
these notifications.

### Hook and guardian review notifications already reject malformed required fields

**Location:** `hook_notifications.go:66` — hook and guardian notification unmarshaling

**Reason:** The current checkout already validates `HookOutputEntry`,
`HookRunSummary`, `HookStartedNotification`, `HookCompletedNotification`,
`GuardianApprovalReview`, `ItemGuardianApprovalReviewStartedNotification`, and
`ItemGuardianApprovalReviewCompletedNotification` with schema-aligned
`UnmarshalJSON` implementations. Nested required fields such as `run.id`,
`run.status`, and `review.status` are enforced before dispatch, and
`hook_notifications_test.go` exercises the malformed-payload error path.

### Config warning notifications already require a summary

**Location:** `config.go:497` — config warning unmarshaling

**Reason:** `ConfigWarningNotification` already implements `UnmarshalJSON` and
requires `summary` through `unmarshalInboundObject`. The same file also
validates nested `TextRange` and `TextPosition` fields. The existing tests in
`config_test.go` cover both direct unmarshaling failures and handler error
reporting for missing `summary`.
