### Regression coverage for malformed approval payloads already exists

**Location:** `dispatch_test.go:826`, `approval_test.go:747`

**Reason:** The repo already has focused tests for zero-value `DynamicToolCallResponse`,
zero-value `ToolRequestUserInputResponse`, nested empty answer payloads, and malformed dynamic
tool content items missing `type`, `text`, or `imageUrl`. The reported testing gap is stale in
this checkout.

### Duplicate approval dispatch tests across two files

**Location:** `approval_test.go`, `dispatch_test.go` — approval handler tests
**Date:** 2026-03-01

**Reason:** The audit claims these tests are "identical" and "redundant." They test different
aspects. `TestApprovalHandlerDispatch` (approval_test.go) registers all 7 handlers and verifies
each handler **was called** (checking invocation). `TestKnownApprovalHandlerDispatch` (dispatch_test.go)
is table-driven, registers one handler per case, and verifies the **response has no error** (checking
dispatch correctness). `TestMissingApprovalHandler` tests with zero handlers set;
`TestMissingApprovalHandlerReturnsMethodNotFound` tests with a specific handler missing while others
are registered. These are complementary, not duplicative.

### Approval handler dispatch claimed to be identically tested in two files

**Location:** `approval_test.go:243`, `dispatch_test.go:208` — approval handler dispatch tests
**Date:** 2026-03-01

**Reason:** The audit claims these tests are identical and one should be removed. They test different
aspects. `TestApprovalHandlerDispatch` (approval_test.go) registers all 7 handlers simultaneously
and verifies each handler **was called** via boolean flags (integration-style test of the full
handler set). `TestKnownApprovalHandlerDispatch` (dispatch_test.go) is table-driven, registers one
handler per test case in isolation, and verifies the **response has no error** (unit-style test of
individual dispatch correctness). These are complementary: one tests all handlers working together,
the other tests each handler in isolation.
