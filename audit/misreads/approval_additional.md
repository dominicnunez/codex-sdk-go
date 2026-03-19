### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `approval.go:199`, `approval.go:583`, `approval.go:947`, `approval.go:978`, `approval.go:1284`, `approval_additional.go:62`, `approval_additional.go:157`, `client.go:584`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.
