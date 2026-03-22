### Claimed missing EOF test coverage is based on a non-existent deadlock path

**Location:** `2020`

**Reason:** The reported test gap is tied to the assertion that `enqueueWrite(..., watchReaderStop=false)`
has a deadlock path after `readerStopped` closes. That deadlock path does not exist because
`enqueueWrite` returns immediately on `readerStopped` when `watchReaderStop` is false. The finding
therefore misclassifies coverage as missing for behavior that does not occur.

### Oversized-frame tests already cover the late-id response ordering case

**Location:** `1327`

**Reason:** The current transport tests already send an oversized JSON-RPC
response with a huge `result` field before the top-level `id` and assert that
the pending `Send` resolves with a parse error instead of timing out. The
claimed test gap no longer exists in this checkout.
