### Thread and config union tests should reject unknown approval-policy string literals

**Location:** `97`

**Reason:** `AskForApprovalWrapper.UnmarshalJSON` intentionally accepts any JSON string by storing
it as `approvalPolicyLiteral` (`thread.go:541-546`) instead of validating against the current enum
set. That matches the SDK's forward-compatibility handling for union string variants, so adding
tests that expect unknown approval-policy strings to be rejected would assert behavior the current
design does not implement. The real missing regression in this area is the `subAgent.other:null`
decode path.

### Thread and config union tests should reject unknown approval-policy string literals

**Location:** `97`

**Reason:** `AskForApprovalWrapper.UnmarshalJSON` intentionally accepts any JSON string by storing
it as `approvalPolicyLiteral` (`thread.go:541-546`) instead of validating against the current enum
set. That matches the SDK's forward-compatibility handling for union string variants, so adding
tests that expect unknown approval-policy strings to be rejected would assert behavior the current
design does not implement. The real missing regression in this area is the `subAgent.other:null`
decode path.
