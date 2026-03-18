### [Bug] Thread response methods accept schema-required fields as zero values
- **Severity**: Medium
- **File**: thread.go:806
- **Details**: `Thread.Read`, `Thread.Resume`, `Thread.MetadataUpdate`, and `Thread.Unarchive` only call `sendRequest` and never validate required response fields. The v2 schemas for `ThreadReadResponse`, `ThreadResumeResponse`, `ThreadMetadataUpdateResponse`, and `ThreadUnarchiveResponse` require a `thread` object, and the embedded `Thread` schema requires `id` and other fields. A server response like `{"thread":{}}` deserializes successfully and returns an unusable zero-value thread instead of a protocol error.
- **Suggested fix**: Add per-response validation after `sendRequest`, following the existing `Initialize` and `Thread.Start` pattern, and reject missing required `Thread` fields before returning success.

### [Bug] Plugin response methods do not enforce required result fields
- **Severity**: Medium
- **File**: plugin.go:163
- **Details**: `Plugin.Read` and `Plugin.Install` trust JSON unmarshaling alone. The schemas require `plugin` for `PluginReadResponse`, and `appsNeedingAuth` plus `authPolicy` for `PluginInstallResponse`. Missing fields currently decode to zero values such as an empty `PluginDetail` or empty auth policy, which lets protocol violations look successful and pushes failures downstream into caller logic.
- **Suggested fix**: Validate required fields after unmarshaling, including nested required plugin fields, and return a descriptive protocol error when the server omits them.

### [Bug] Oversized response recovery depends on JSON field order
- **Severity**: Medium
- **File**: stdio.go:1072
- **Details**: `handleOversizedFrame` only inspects the retained 64 KB prefix from an oversized frame. If a valid oversized JSON-RPC response places `"id"` after a huge `"result"` field, the prefix has no recoverable request ID, so the pending `Send` is left waiting until context timeout even though the response was already discarded. JSON object field order is not guaranteed, so this is a real correctness gap and an avoidable timeout vector.
- **Suggested fix**: Preserve enough routing metadata to recover late top-level `id` fields, or conservatively fail pending sends once an oversized frame is identified as a response but cannot be matched safely.

### [Testing] Required-field validation is only covered for `thread/start`
- **Severity**: Medium
- **File**: thread_test.go:153
- **Details**: The suite checks missing `thread.id` for `Thread.Start`, but `Thread.Read`, `Thread.Resume`, `Thread.Unarchive`, `Thread.MetadataUpdate`, `Plugin.Read`, and `Plugin.Install` only have happy-path, empty-result, or malformed-type tests. There are no regression tests for responses that are structurally valid JSON but omit schema-required fields, which is why the zero-value success paths above were not caught.
- **Suggested fix**: Add table-driven tests for missing required fields on every response type that relies on manual validation rather than type mismatches.

### [Testing] Oversized-frame tests miss the late-`id` response ordering case
- **Severity**: Medium
- **File**: stdio_test.go:1148
- **Details**: The oversized transport tests cover early `id` fields and completely ambiguous oversized frames, but they do not cover a valid response whose large `result` appears before `id`. That missing case hides the field-order bug in `handleOversizedFrame`, because the current tests only exercise the routable and clearly-unroutable extremes.
- **Suggested fix**: Add an oversized response test where `result` comes before `id` and assert that `Send` still resolves deterministically instead of waiting for deadline expiry.

### [Code Quality] Comment claims fallback cloning preserves unknown variants when it actually drops them
- **Severity**: Low
- **File**: conversation.go:185
- **Details**: The comment says unknown future variants “must preserve data” and that the JSON fallback “keeps parity,” but the fallback helpers return nil or zero values when cloning fails. `conversation_internal_test.go` explicitly asserts that these fallbacks drop uncloneable values. The current comment describes behavior the implementation does not provide.
- **Suggested fix**: Rewrite the comment to describe the real best-effort semantics, or change the fallback path so it actually preserves opaque data.

### [Configuration] One audit exception entry is stale and contradicts the current transport API
- **Severity**: Low
- **File**: audit/exceptions/risks.md:12
- **Details**: The first risk entry says fixing reader shutdown would require changing `StdioTransport` from `io.Reader` to `io.ReadCloser`, but `NewStdioTransport` already requires an `io.ReadCloser` and `closeWithFailure` closes `readerCloser`. This stale exception can mislead future audits and suppress already-fixed behavior.
- **Suggested fix**: Remove or update the stale exception entry so the exception set matches the current implementation.
