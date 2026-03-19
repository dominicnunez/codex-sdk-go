# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Reason:** Explanation (can be multiple lines)

### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `client.go:608`, `approval.go:943`, `approval.go:1166`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Dynamic tool content items do not accept payloads missing required fields

**Location:** `approval.go:997`, `approval.go:1030`, `approval.go:1047`

**Reason:** The recognized `inputText` and `inputImage` variants already use
`unmarshalInboundObject` to require both the discriminator and the variant-specific field, and the
wrapper decoder rejects a missing or empty `type` before dispatching. The regression coverage in
`approval_test.go:747` already checks `{}`, `{"type":"inputText"}`, and `{"type":"inputImage"}`
and expects all three to fail decoding.

### Regression coverage for malformed approval payloads already exists

**Location:** `dispatch_test.go:826`, `approval_test.go:747`

**Reason:** The repo already has focused tests for zero-value `DynamicToolCallResponse`,
zero-value `ToolRequestUserInputResponse`, nested empty answer payloads, and malformed dynamic
tool content items missing `type`, `text`, or `imageUrl`. The reported testing gap is stale in
this checkout.
