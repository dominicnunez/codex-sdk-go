# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Transport starvation coverage for blocked streaming handlers already exists

**Location:** `stdio_internal_test.go:1196` — `TestStdioStreamingBackpressureDoesNotStarveUnrelatedResponses`

**Reason:** The current tree already has the integration regression the finding
claims is missing. The test blocks `item/agentMessage/delta` handlers, floods
streaming notifications past the worker queue, writes an unrelated response, and
asserts that the pending `Send` completes while the streaming handlers remain
blocked. The missing-coverage report is stale against the checked-in test suite.


# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### The malformed-request test is not exercising dead code

**Location:** `stdio_internal_test.go:17` — invalid request object handling

**Reason:** The report names a different test/helper than what exists in the file, and it misstates
production reachability. The actual tests at `stdio_internal_test.go:17` and `stdio_internal_test.go:55`
exercise `handleInvalidRequestObject`, which is called by the real transport path in
`stdio.go:628-629` and `stdio.go:662`. This is not dead code and not an unreachable-only test.
