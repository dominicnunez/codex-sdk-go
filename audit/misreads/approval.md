### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `943`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `1166`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `199`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `583`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `947`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `978`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `1284`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `199`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `583`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `947`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `978`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `1284`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `943`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Approval handlers already reject malformed dynamic tool and user-input results

**Location:** `1166`

**Reason:** The common approval path already validates handler return values before marshaling
them onto the wire. `DynamicToolCallResponse.validate` rejects a nil `contentItems` slice, and
`ToolRequestUserInputResponse.validate` rejects a nil `answers` map plus nested answers with a nil
`answers` slice. The failure path is covered by `dispatch_test.go:826`.

### Dynamic tool content items do not accept payloads missing required fields

**Location:** `997`

**Reason:** The recognized `inputText` and `inputImage` variants already use
`unmarshalInboundObject` to require both the discriminator and the variant-specific field, and the
wrapper decoder rejects a missing or empty `type` before dispatching. The regression coverage in
`approval_test.go:747` already checks `{}`, `{"type":"inputText"}`, and `{"type":"inputImage"}`
and expects all three to fail decoding.

### Dynamic tool content items do not accept payloads missing required fields

**Location:** `1030`

**Reason:** The recognized `inputText` and `inputImage` variants already use
`unmarshalInboundObject` to require both the discriminator and the variant-specific field, and the
wrapper decoder rejects a missing or empty `type` before dispatching. The regression coverage in
`approval_test.go:747` already checks `{}`, `{"type":"inputText"}`, and `{"type":"inputImage"}`
and expects all three to fail decoding.

### Dynamic tool content items do not accept payloads missing required fields

**Location:** `1047`

**Reason:** The recognized `inputText` and `inputImage` variants already use
`unmarshalInboundObject` to require both the discriminator and the variant-specific field, and the
wrapper decoder rejects a missing or empty `type` before dispatching. The regression coverage in
`approval_test.go:747` already checks `{}`, `{"type":"inputText"}`, and `{"type":"inputImage"}`
and expects all three to fail decoding.

### Approval request enums already reject unsupported protocol and refresh reason values

**Location:** `568`

**Reason:** The current approval request decoding path already validates both constrained enums
before handlers see them. `NetworkApprovalContext.UnmarshalJSON` rejects unsupported
`NetworkApprovalProtocol` values, and `ChatgptAuthTokensRefreshParams.UnmarshalJSON` rejects
unsupported `ChatgptAuthTokensRefreshReason` values. The regression tests
`TestNetworkApprovalContextRejectsInvalidProtocol` and
`TestChatgptAuthTokensRefreshParamsRejectInvalidReason` cover both invalid-enum paths.

### Approval request enums already reject unsupported protocol and refresh reason values

**Location:** `1263`

**Reason:** The current approval request decoding path already validates both constrained enums
before handlers see them. `NetworkApprovalContext.UnmarshalJSON` rejects unsupported
`NetworkApprovalProtocol` values, and `ChatgptAuthTokensRefreshParams.UnmarshalJSON` rejects
unsupported `ChatgptAuthTokensRefreshReason` values. The regression tests
`TestNetworkApprovalContextRejectsInvalidProtocol` and
`TestChatgptAuthTokensRefreshParamsRejectInvalidReason` cover both invalid-enum paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `199`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `583`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `947`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `978`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Approval handler responses already fail locally when decision, scope, action, or token fields are invalid

**Location:** `1284`

**Reason:** The generic approval path does not serialize these malformed results onto the wire in
the current code. `handleApproval` calls `validateDecodedResponse` before marshaling, and the
approval response types now implement `validate()` methods for the constrained decision, scope,
action, and credential fields the report described. The regression test
`TestApprovalHandlerRejectsInvalidResponsePayloads` exercises those rejection paths.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `148-150`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `409-411`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `672-674`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `802-804`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### ReviewDecisionWrapper unmarshal correctly handles double-nested network_policy_amendment

**Location:** `198-204`

**Reason:** The audit claims that unmarshaling `raw` (the value at key `"network_policy_amendment"`)
into `NetworkPolicyAmendmentDecision` loses data because the struct expects a `"network_policy_amendment"`
JSON field that isn't present. This is incorrect. The spec (`ExecCommandApprovalResponse.json:71-82`,
`ApplyPatchApprovalResponse.json:71-82`) defines double nesting: the outer object has key
`"network_policy_amendment"` whose value is another object with an inner `"network_policy_amendment"`
key containing the actual `NetworkPolicyAmendment` data. So `raw` is
`{"network_policy_amendment":{"action":"...","host":"..."}}`, which correctly deserializes into
`NetworkPolicyAmendmentDecision{NetworkPolicyAmendment: ...}`. The roundtrip is not broken.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper field names differ because the specs differ

**Location:** `189-195`

**Reason:** The audit claims the naming difference between `ApprovedExecpolicyAmendmentDecision.ProposedExecpolicyAmendment`
(in `ReviewDecisionWrapper`) and `AcceptWithExecpolicyAmendmentDecision.ExecpolicyAmendment`
(in `CommandExecutionApprovalDecisionWrapper`) is a code-level inconsistency. This is incorrect.
These are two different spec schemas with different JSON field names:
`ApplyPatchApprovalResponse.json` / `ExecCommandApprovalResponse.json` define outer key
`"approved_execpolicy_amendment"` with inner field `"proposed_execpolicy_amendment"`, while
`CommandExecutionRequestApprovalResponse.json` defines outer key `"acceptWithExecpolicyAmendment"`
with inner field `"execpolicy_amendment"`. The Go types faithfully mirror the specs. The naming
difference originates in the protocol definition, not in the Go code.

### DynamicToolCallParams.Arguments uses untyped interface{} described as a code quality issue

**Location:** `754`

**Reason:** This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which explains
the deliberate caller-convenience tradeoff. Duplicate of existing exception.

### DynamicToolCallParams.Arguments untyped interface{} described as a new finding but covered by existing exception

**Location:** `753`

**Reason:** The audit describes `DynamicToolCallParams.Arguments` being `interface{}` as a code
quality issue. This is the exact same issue as the known exception "OutputSchema and
DynamicToolCallParams.Arguments use bare interface{} instead of json.RawMessage" which explains
the deliberate caller-convenience tradeoff. Duplicate of existing exception.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper use interface{} instead of sealed interface

**Location:** `179`

**Reason:** This is already covered by the known exception "ReviewDecisionWrapper and
CommandExecutionApprovalDecisionWrapper use untyped interface{} for Value" which explains that
changing these to sealed interfaces would alter the public API surface, violating the spec
compliance rules. The custom UnmarshalJSON/MarshalJSON methods already enforce valid values
at runtime. Duplicate of existing exception.

### ReviewDecisionWrapper and CommandExecutionApprovalDecisionWrapper use interface{} instead of sealed interface

**Location:** `463`

**Reason:** This is already covered by the known exception "ReviewDecisionWrapper and
CommandExecutionApprovalDecisionWrapper use untyped interface{} for Value" which explains that
changing these to sealed interfaces would alter the public API surface, violating the spec
compliance rules. The custom UnmarshalJSON/MarshalJSON methods already enforce valid values
at runtime. Duplicate of existing exception.

### Approval handler error path claimed to only be tested for ChatGPT token refresh

**Location:** `386`

**Reason:** Factually wrong. `stdio_test.go:729` (`TestStdioApprovalInvalidParamsReturnsErrorCode`)
tests the unmarshal-error path for `applyPatchApproval` through a real `StdioTransport`, sending
invalid JSON params and verifying the `-32602` error code. `stdio_test.go:776`
(`TestStdioApprovalHandlerErrorReturnsErrorCode`) tests the handler-error path for the same type.
Since `handleApproval` is a generic function, testing one approval type exercises the generic
unmarshal-error and handler-error code paths for all types.

### Approval handler wire-format fidelity for redacted types claimed to be untested

**Location:** `913-930`

**Reason:** The audit claims "no test verifies that marshalForWire is actually used in the approval
response path." This is factually wrong. `credential_redact_test.go:62-98`
(`TestCredentialTypesRedactWithAllFormatVerbs/ChatgptAuthTokensRefresh/wireProtocol`) registers an
`OnChatgptAuthTokensRefresh` handler returning a real token, simulates a server request through the
transport via `InjectServerRequest`, and verifies the response JSON contains the unredacted token
and does not contain `[REDACTED]`. This exercises the full `handleApproval` → `marshalForWire` path.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `148-150`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `409-411`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `672-674`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `802-804`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `148-150`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `409-411`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `672-674`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.

### Wrapper MarshalJSON methods do not panic on nil interface Value

**Location:** `802-804`

**Reason:** The audit claims these wrappers "panic on nil Value" because they call `json.Marshal(w.Value)`
without a nil guard. This is incorrect. All `Value` fields are Go interface types (`FileChange`,
`CommandAction`, `ParsedCommand`, `DynamicToolCallOutputContentItem`, `ReviewTarget`, `PatchChangeKind`,
`WebSearchAction`). Calling `json.Marshal` on a nil interface value does NOT panic — it returns
`[]byte("null"), nil`. The behavior is identical to the explicit `[]byte("null"), nil` pattern used
by other wrappers. Furthermore, these `Value` fields are always populated by their corresponding
`UnmarshalJSON` methods, which return errors on unknown types rather than leaving `Value` nil.
There is no panic and no data corruption.
