### Hook and guardian review notifications already reject malformed required fields

**Location:** `66`

**Reason:** The current checkout already validates `HookOutputEntry`,
`HookRunSummary`, `HookStartedNotification`, `HookCompletedNotification`,
`GuardianApprovalReview`, `ItemGuardianApprovalReviewStartedNotification`, and
`ItemGuardianApprovalReviewCompletedNotification` with schema-aligned
`UnmarshalJSON` implementations. Nested required fields such as `run.id`,
`run.status`, and `review.status` are enforced before dispatch, and
`hook_notifications_test.go` exercises the malformed-payload error path.

### Thread, turn, and guardian payloads already reject unsupported enum values

**Location:** `167`

**Reason:** The current branch already validates these inbound enums during JSON decoding via
`unmarshalEnumString`. Unsupported `TurnStatus`, `ThreadActiveFlag`,
`GuardianApprovalReviewStatus`, and `GuardianRiskLevel` values fail unmarshaling before they can
be cached on a thread or dispatched to notification handlers. The report item was stale; the only
remaining gap was regression coverage, which is now covered by the test suite.

### Thread, turn, and guardian payloads already reject unsupported enum values

**Location:** `167`

**Reason:** The current branch already validates these inbound enums during JSON decoding via
`unmarshalEnumString`. Unsupported `TurnStatus`, `ThreadActiveFlag`,
`GuardianApprovalReviewStatus`, and `GuardianRiskLevel` values fail unmarshaling before they can
be cached on a thread or dispatched to notification handlers. The report item was stale; the only
remaining gap was regression coverage, which is now covered by the test suite.
