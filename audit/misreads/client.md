### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `client.go:608`, `approval.go:943`, `approval.go:1166`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Server responses no longer spoof local transport failures in Client.Send

**Location:** `client.go:336` — response error handling

**Reason:** The current `Client.Send` implementation no longer inspects
wire-level `error.data` transport metadata. When a response contains an error,
it always returns `NewRPCError(resp.Error)` at `client.go:337-338`, so a server
cannot forge `{"transport":"failed","origin":"client"}` and have the client
reclassify it as a local `TransportError`.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `approval.go:199`, `approval.go:583`, `approval.go:947`, `approval.go:978`, `approval.go:1284`, `approval_additional.go:62`, `approval_additional.go:157`, `client.go:584`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### handleApproval marshal error does not leak internal structure

**Location:** `client.go:254-256` — json.Marshal error in handleApproval
**Date:** 2026-02-27

**Reason:** The audit claims the raw `json.Marshal` error leaks type information across the trust
boundary and is "visible in any error-logging or debugging path before it reaches the transport."
This is incorrect. The error propagates directly to `handleRequest` in `stdio.go:314`, which
immediately replaces it with a hardcoded `"internal handler error"` message (stdio.go:327) before
sending the JSON-RPC response. The original error string is never logged, stored, or exposed to
any external party. There is no logging or debugging path in this code — the error goes from
`handleApproval` return → `handleRequest` goroutine → generic error response. The internal type
information never crosses any trust boundary.

### handleApproval marshalForWire pointer-to-result described as a potential bug but works correctly

**Location:** `client.go:324` — marshalForWire(&result) call
**Date:** 2026-02-28

**Reason:** The audit describes a theoretical scenario where `marshalForWire(&result)` could fail
to satisfy the `wireMarshaler` interface check for future types. The audit itself concludes
"The current code is correct for all existing types" and "No change needed for current types.
This is noted for awareness." A finding that describes correct behavior and requires no change
is not an actionable finding — it is a speculative concern about hypothetical future types.

### internalListenerSeq described as inconsistent but acknowledged as correct

**Location:** `client.go:56` — internalListenerSeq counter
**Date:** 2026-03-01

**Reason:** The audit's own conclusion states "No actual bug" and "No change needed — the mutex
protection is sufficient and the pattern is deliberate since the listener map also needs the lock."
A finding that explicitly states no bug exists and no change is needed is not actionable.
`internalListenerSeq` is always accessed under `listenersMu.Lock()` because the listener map
operations require the same lock — using a separate atomic would be unnecessary.

### handleApproval pointer-to-value wireMarshaler dispatch re-flagged as new finding

**Location:** `client.go:324` — marshalForWire(&result) call
**Date:** 2026-03-01

**Reason:** This is a duplicate of the existing exception "handleApproval marshalForWire pointer-to-result
described as a potential bug but works correctly." The audit itself concludes "No change needed for
current types" — all existing approval response types use pointer receivers, and the `&result` pattern
works correctly. A speculative concern about hypothetical future types is not an actionable finding.

### Concurrent notification listener subscribe/unsubscribe claimed to be untested

**Location:** `client.go:214-235` — addNotificationListener concurrent safety
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for subscribe/unsubscribe racing with
dispatch. This is factually wrong. `listener_test.go:33-63` contains `TestConcurrentInternalListeners`
which runs 10 goroutines each performing 50 iterations of subscribe, dispatch, and unsubscribe
concurrently — designed to be run with `-race`.

### FuzzyFileSearch claimed to be missing from approval handler dispatch

**Location:** `client.go:248-294` — handleRequest approval dispatch
**Date:** 2026-03-01

**Reason:** The audit claims `fuzzyFileSearch` should be routed through `handleRequest` as a
server→client approval request. This is incorrect. `fuzzyFileSearch` is a **client→server**
request — it appears in `specs/ClientRequest.json` and is implemented as
`FuzzyFileSearchService.Search()` which calls `sendRequest` (fuzzy_search.go:53). It is correctly
absent from the server→client approval dispatch in `handleRequest`. The `request_coverage_test.go`
comment at line 189 ("server→client request (approval flow)") is misleading, but the code is
correct — `fuzzyFileSearch` is tested in `fuzzy_search_test.go` as a normal client→server method.

### Concurrent notification listener subscribe/unsubscribe claimed to be untested

**Location:** `client.go:214-235` — addNotificationListener concurrent safety
**Date:** 2026-03-01

**Reason:** The audit claims there are no concurrent tests for subscribe/unsubscribe racing with
dispatch. This is factually wrong. `listener_test.go:33-63` contains `TestConcurrentInternalListeners`
which runs 10 goroutines each performing 50 iterations of subscribe, dispatch, and unsubscribe
concurrently — designed to be run with `-race`.

### handleApproval pointer-to-result wireMarshaler dispatch described as fragile but works correctly

**Location:** `client.go:397` — marshalForWire(&result) call
**Date:** 2026-03-01

**Reason:** This is a duplicate of the existing exception "handleApproval marshalForWire pointer-to-result
described as a potential bug but works correctly." The audit itself concludes the code works correctly
today and "No immediate change needed." All existing approval response types implement `marshalWire`
on value receivers, which are callable on pointer receivers in Go. The concern about a future change
to pointer receivers is speculative — not an actionable finding.

### handleApproval passing pointer to result for marshalForWire is standard Go behavior

**Location:** `client.go:397` — marshalForWire(&result) call
**Date:** 2026-03-01

**Reason:** The audit notes that `marshalForWire(&result)` checks `wireMarshaler` on `*R` instead of `R`,
then admits "the current code is correct for all existing types." The suggested fix is to add a comment
explaining how Go method sets work. This is standard Go behavior (a pointer to a value satisfies
interfaces with both value and pointer receivers), not a bug or code quality issue. The finding
describes no actual or potential incorrect behavior.
