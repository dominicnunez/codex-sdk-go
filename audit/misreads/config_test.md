# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Thread and config union tests should reject unknown approval-policy string literals

**Location:** `thread_test.go:375`, `config_test.go:97` — approval-policy union regression coverage

**Reason:** `AskForApprovalWrapper.UnmarshalJSON` intentionally accepts any JSON string by storing
it as `approvalPolicyLiteral` (`thread.go:541-546`) instead of validating against the current enum
set. That matches the SDK's forward-compatibility handling for union string variants, so adding
tests that expect unknown approval-policy strings to be rejected would assert behavior the current
design does not implement. The real missing regression in this area is the `subAgent.other:null`
decode path.
