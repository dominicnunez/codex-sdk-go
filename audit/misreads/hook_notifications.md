# Notification Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale notification-validation findings.

### Hook and guardian review notifications already reject malformed required fields

**Location:** `hook_notifications.go:66` — hook and guardian notification unmarshaling

**Reason:** The current checkout already validates `HookOutputEntry`,
`HookRunSummary`, `HookStartedNotification`, `HookCompletedNotification`,
`GuardianApprovalReview`, `ItemGuardianApprovalReviewStartedNotification`, and
`ItemGuardianApprovalReviewCompletedNotification` with schema-aligned
`UnmarshalJSON` implementations. Nested required fields such as `run.id`,
`run.status`, and `review.status` are enforced before dispatch, and
`hook_notifications_test.go` exercises the malformed-payload error path.


# Validation Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> This file covers stale response and notification validation findings.

### Thread, turn, and guardian payloads already reject unsupported enum values

**Location:** `enums.go:18`, `enums.go:209`, `hook_notifications.go:167` — custom `UnmarshalJSON` on `TurnStatus`, `ThreadActiveFlag`, `GuardianApprovalReviewStatus`, and `GuardianRiskLevel`

**Reason:** The current branch already validates these inbound enums during JSON decoding via
`unmarshalEnumString`. Unsupported `TurnStatus`, `ThreadActiveFlag`,
`GuardianApprovalReviewStatus`, and `GuardianRiskLevel` values fail unmarshaling before they can
be cached on a thread or dispatched to notification handlers. The report item was stale; the only
remaining gap was regression coverage, which is now covered by the test suite.
