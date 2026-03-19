### Thread, turn, and guardian payloads already reject unsupported enum values

**Location:** `enums.go:18`, `enums.go:209`, `hook_notifications.go:167` — custom `UnmarshalJSON` on `TurnStatus`, `ThreadActiveFlag`, `GuardianApprovalReviewStatus`, and `GuardianRiskLevel`

**Reason:** The current branch already validates these inbound enums during JSON decoding via
`unmarshalEnumString`. Unsupported `TurnStatus`, `ThreadActiveFlag`,
`GuardianApprovalReviewStatus`, and `GuardianRiskLevel` values fail unmarshaling before they can
be cached on a thread or dispatched to notification handlers. The report item was stale; the only
remaining gap was regression coverage, which is now covered by the test suite.
