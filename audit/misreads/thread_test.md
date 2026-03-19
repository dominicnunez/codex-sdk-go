### Thread service tests for empty-response methods claimed to discard meaningful responses

**Location:** `thread_test.go:451,494,599` — TestThreadSetName, TestThreadArchive, TestThreadCompactStart
**Date:** 2026-03-01

**Reason:** The audit claims these tests "set up mock responses with thread data, call the service
method, then discard the response with `_ = response`" and that "the mock response data is set up
but never validated." This misreads the response types. `ThreadSetNameResponse`, `ThreadArchiveResponse`,
and `ThreadCompactStartResponse` are all empty structs (per spec), and their service methods pass `nil`
as the deserialization target to `sendRequest`. There is nothing to validate on the response — `_ = response`
is correct. The mock response data setup is superfluous boilerplate, but discarding an empty struct
is not a testing gap. (Note: `TestThreadUnsubscribe` is a separate case — `ThreadUnsubscribeResponse`
has a `Status` field that the test genuinely does not validate.)

### Required-field validation is not limited to thread/start

**Location:** `thread_test.go:197`, `plugin_test.go:290` — response validation regression coverage

**Reason:** The current test suite already exercises missing required fields beyond
`Thread.Start`. `TestThreadResponsesRejectMissingRequiredThreadFields` covers
`Thread.Read`, `Thread.Resume`, `Thread.MetadataUpdate`, and `Thread.Unarchive`,
and `TestThreadResponseRequiredFieldValidation` checks missing required nested
thread fields across those responses. `TestPluginRequiredFieldValidation` does
the same for `Plugin.Read` and `Plugin.Install`.

### Thread and config union tests should reject unknown approval-policy string literals

**Location:** `thread_test.go:375`, `config_test.go:97` — approval-policy union regression coverage

**Reason:** `AskForApprovalWrapper.UnmarshalJSON` intentionally accepts any JSON string by storing
it as `approvalPolicyLiteral` (`thread.go:541-546`) instead of validating against the current enum
set. That matches the SDK's forward-compatibility handling for union string variants, so adding
tests that expect unknown approval-policy strings to be rejected would assert behavior the current
design does not implement. The real missing regression in this area is the `subAgent.other:null`
decode path.
