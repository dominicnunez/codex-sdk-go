package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestApplyPatchApprovalRoundTrip tests ApplyPatchApproval params/response JSON round-trip
func TestApplyPatchApprovalRoundTrip(t *testing.T) {
	// Test params serialization
	params := codex.ApplyPatchApprovalParams{
		CallID:         "call-123",
		ConversationID: "thread-456",
		FileChanges: map[string]codex.FileChangeWrapper{
			"file1.go": {Value: &codex.AddFileChange{Content: "new content"}},
			"file2.go": {Value: &codex.UpdateFileChange{UnifiedDiff: "diff content", MovePath: ptr("new/path.go")}},
			"file3.go": {Value: &codex.DeleteFileChange{Content: "old content"}},
		},
		GrantRoot: ptr("/home/user"),
		Reason:    ptr("security patch"),
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal params: %v", err)
	}

	var decoded codex.ApplyPatchApprovalParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}

	if decoded.CallID != "call-123" || decoded.ConversationID != "thread-456" {
		t.Errorf("params round-trip failed")
	}

	// Test response deserialization with various decision types
	testCases := []struct {
		name         string
		responseJSON string
		checkFunc    func(codex.ApplyPatchApprovalResponse) bool
	}{
		{
			name:         "approved",
			responseJSON: `{"decision":"approved"}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				return r.Decision.Value == "approved"
			},
		},
		{
			name:         "approved_for_session",
			responseJSON: `{"decision":"approved_for_session"}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				return r.Decision.Value == "approved_for_session"
			},
		},
		{
			name:         "denied",
			responseJSON: `{"decision":"denied"}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				return r.Decision.Value == "denied"
			},
		},
		{
			name:         "abort",
			responseJSON: `{"decision":"abort"}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				return r.Decision.Value == "abort"
			},
		},
		{
			name:         "approved_execpolicy_amendment",
			responseJSON: `{"decision":{"approved_execpolicy_amendment":{"proposed_execpolicy_amendment":["rule1","rule2"]}}}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				obj, ok := r.Decision.Value.(codex.ApprovedExecpolicyAmendmentDecision)
				return ok && len(obj.ProposedExecpolicyAmendment) == 2
			},
		},
		{
			name:         "network_policy_amendment",
			responseJSON: `{"decision":{"network_policy_amendment":{"network_policy_amendment":{"action":"allow","host":"example.com"}}}}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				obj, ok := r.Decision.Value.(codex.NetworkPolicyAmendmentDecision)
				return ok && obj.NetworkPolicyAmendment.Action == "allow"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp codex.ApplyPatchApprovalResponse
			if err := json.Unmarshal([]byte(tc.responseJSON), &resp); err != nil {
				t.Fatalf("Unmarshal response: %v", err)
			}
			if !tc.checkFunc(resp) {
				t.Errorf("decision check failed")
			}
		})
	}
}

// TestCommandExecutionRequestApprovalRoundTrip tests CommandExecutionRequestApproval params/response
func TestCommandExecutionRequestApprovalRoundTrip(t *testing.T) {
	// Test params serialization
	params := codex.CommandExecutionRequestApprovalParams{
		ItemID:   "item-123",
		ThreadID: "thread-456",
		TurnID:   "turn-789",
		Command:  ptr("git status"),
		Cwd:      ptr("/home/user/project"),
		CommandActions: &[]codex.CommandActionWrapper{
			{Value: &codex.ReadCommandAction{Command: "cat", Name: "file.txt", Path: "/path/to/file.txt"}},
			{Value: &codex.UnknownCommandAction{Command: "git status"}},
		},
		Reason: ptr("checking git status"),
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal params: %v", err)
	}

	var decoded codex.CommandExecutionRequestApprovalParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal params: %v", err)
	}

	if decoded.ItemID != "item-123" || *decoded.Command != "git status" {
		t.Errorf("params round-trip failed")
	}

	// Test response deserialization
	testCases := []struct {
		name         string
		responseJSON string
		checkFunc    func(codex.CommandExecutionRequestApprovalResponse) bool
	}{
		{
			name:         "accept",
			responseJSON: `{"decision":"accept"}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				return r.Decision.Value == "accept"
			},
		},
		{
			name:         "acceptForSession",
			responseJSON: `{"decision":"acceptForSession"}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				return r.Decision.Value == "acceptForSession"
			},
		},
		{
			name:         "decline",
			responseJSON: `{"decision":"decline"}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				return r.Decision.Value == "decline"
			},
		},
		{
			name:         "cancel",
			responseJSON: `{"decision":"cancel"}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				return r.Decision.Value == "cancel"
			},
		},
		{
			name:         "acceptWithExecpolicyAmendment",
			responseJSON: `{"decision":{"acceptWithExecpolicyAmendment":{"execpolicy_amendment":["rule1"]}}}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				obj, ok := r.Decision.Value.(codex.AcceptWithExecpolicyAmendmentDecision)
				return ok && len(obj.ExecpolicyAmendment) == 1
			},
		},
		{
			name:         "applyNetworkPolicyAmendment",
			responseJSON: `{"decision":{"applyNetworkPolicyAmendment":{"network_policy_amendment":{"action":"deny","host":"blocked.com"}}}}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				obj, ok := r.Decision.Value.(codex.ApplyNetworkPolicyAmendmentDecision)
				return ok && obj.NetworkPolicyAmendment.Action == "deny"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp codex.CommandExecutionRequestApprovalResponse
			if err := json.Unmarshal([]byte(tc.responseJSON), &resp); err != nil {
				t.Fatalf("Unmarshal response: %v", err)
			}
			if !tc.checkFunc(resp) {
				t.Errorf("decision check failed")
			}
		})
	}
}

// TestApprovalHandlerDispatch tests that approval handlers are called when server sends requests
func TestApprovalHandlerDispatch(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Track handler calls
	var applyPatchCalled bool
	var commandExecCalled bool
	var execCommandCalled bool
	var fileChangeCalled bool
	var skillCalled bool
	var toolCallCalled bool
	var userInputCalled bool
	var authRefreshCalled bool

	handlers := codex.ApprovalHandlers{
		OnApplyPatchApproval: func(ctx context.Context, params codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
			applyPatchCalled = true
			return codex.ApplyPatchApprovalResponse{
				Decision: codex.ReviewDecisionWrapper{Value: "approved"},
			}, nil
		},
		OnCommandExecutionRequestApproval: func(ctx context.Context, params codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
			commandExecCalled = true
			return codex.CommandExecutionRequestApprovalResponse{
				Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "accept"},
			}, nil
		},
		OnExecCommandApproval: func(ctx context.Context, params codex.ExecCommandApprovalParams) (codex.ExecCommandApprovalResponse, error) {
			execCommandCalled = true
			return codex.ExecCommandApprovalResponse{
				Decision: codex.ReviewDecisionWrapper{Value: "approved"},
			}, nil
		},
		OnFileChangeRequestApproval: func(ctx context.Context, params codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
			fileChangeCalled = true
			return codex.FileChangeRequestApprovalResponse{
				Decision: "accept",
			}, nil
		},
		OnSkillRequestApproval: func(ctx context.Context, params codex.SkillRequestApprovalParams) (codex.SkillRequestApprovalResponse, error) {
			skillCalled = true
			return codex.SkillRequestApprovalResponse{
				Decision: "approve",
			}, nil
		},
		OnDynamicToolCall: func(ctx context.Context, params codex.DynamicToolCallParams) (codex.DynamicToolCallResponse, error) {
			toolCallCalled = true
			return codex.DynamicToolCallResponse{
				Success: true,
				ContentItems: []codex.DynamicToolCallOutputContentItemWrapper{
					{Value: &codex.InputTextDynamicToolCallOutputContentItem{Text: "result"}},
				},
			}, nil
		},
		OnToolRequestUserInput: func(ctx context.Context, params codex.ToolRequestUserInputParams) (codex.ToolRequestUserInputResponse, error) {
			userInputCalled = true
			return codex.ToolRequestUserInputResponse{
				Answers: map[string]codex.ToolRequestUserInputAnswer{
					"q1": {Answers: []string{"answer"}},
				},
			}, nil
		},
		OnChatgptAuthTokensRefresh: func(ctx context.Context, params codex.ChatgptAuthTokensRefreshParams) (codex.ChatgptAuthTokensRefreshResponse, error) {
			authRefreshCalled = true
			return codex.ChatgptAuthTokensRefreshResponse{
				AccessToken:       "new-token",
				ChatgptAccountID:  "account-123",
				ChatgptPlanType:   ptr("plus"),
			}, nil
		},
	}

	client.SetApprovalHandlers(handlers)

	// Inject server→client requests for each approval type
	ctx := context.Background()

	// 1. ApplyPatchApproval
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 1},
		Method:  "applyPatchApproval",
		Params:  json.RawMessage(`{"callId":"c1","conversationId":"t1","fileChanges":{}}`),
	})

	// 2. CommandExecutionRequestApproval
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 2},
		Method:  "item/commandExecution/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i1","threadId":"t1","turnId":"tu1"}`),
	})

	// 3. ExecCommandApproval (legacy)
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 3},
		Method:  "execCommandApproval",
		Params:  json.RawMessage(`{"callId":"c1","conversationId":"t1","command":["ls"],"cwd":"/","parsedCmd":[]}`),
	})

	// 4. FileChangeRequestApproval
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 4},
		Method:  "item/fileChange/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i1","threadId":"t1","turnId":"tu1"}`),
	})

	// 5. SkillRequestApproval
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 5},
		Method:  "skill/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i1","skillName":"test-skill"}`),
	})

	// 6. DynamicToolCall
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 6},
		Method:  "item/tool/call",
		Params:  json.RawMessage(`{"tool":"test","arguments":{},"callId":"c1","threadId":"t1","turnId":"tu1"}`),
	})

	// 7. ToolRequestUserInput
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 7},
		Method:  "item/tool/requestUserInput",
		Params:  json.RawMessage(`{"itemId":"i1","threadId":"t1","turnId":"tu1","questions":[{"id":"q1","header":"H","question":"Q"}]}`),
	})

	// 8. ChatgptAuthTokensRefresh
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 8},
		Method:  "account/chatgptAuthTokens/refresh",
		Params:  json.RawMessage(`{"reason":"unauthorized"}`),
	})

	// Verify all handlers were called
	if !applyPatchCalled {
		t.Error("ApplyPatchApproval handler not called")
	}
	if !commandExecCalled {
		t.Error("CommandExecutionRequestApproval handler not called")
	}
	if !execCommandCalled {
		t.Error("ExecCommandApproval handler not called")
	}
	if !fileChangeCalled {
		t.Error("FileChangeRequestApproval handler not called")
	}
	if !skillCalled {
		t.Error("SkillRequestApproval handler not called")
	}
	if !toolCallCalled {
		t.Error("DynamicToolCall handler not called")
	}
	if !userInputCalled {
		t.Error("ToolRequestUserInput handler not called")
	}
	if !authRefreshCalled {
		t.Error("ChatgptAuthTokensRefresh handler not called")
	}
}

// TestMissingApprovalHandler tests that missing handlers return method-not-found error
func TestMissingApprovalHandler(t *testing.T) {
	mock := NewMockTransport()
	_ = codex.NewClient(mock)

	// Don't set any handlers - all should return method-not-found

	ctx := context.Background()

	// Inject a server→client request
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 1},
		Method:  "applyPatchApproval",
		Params:  json.RawMessage(`{"callId":"c1","conversationId":"t1","fileChanges":{}}`),
	})

	// Verify response is method-not-found error
	if len(mock.sentResponses) == 0 {
		t.Fatal("No response sent")
	}

	resp := mock.sentResponses[0]
	if resp.Error == nil {
		t.Fatal("Expected error response")
	}
	if resp.Error.Code != codex.ErrCodeMethodNotFound {
		t.Errorf("Expected ErrCodeMethodNotFound error, got code %d", resp.Error.Code)
	}
}

// TestApprovalEndToEnd tests full approval flow from server request to client response
func TestApprovalEndToEnd(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	handlers := codex.ApprovalHandlers{
		OnCommandExecutionRequestApproval: func(ctx context.Context, params codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
			// Verify params
			if params.ItemID != "item-abc" {
				t.Errorf("Expected itemId=item-abc, got %s", params.ItemID)
			}
			if params.Command == nil || *params.Command != "ls -la" {
				t.Errorf("Expected command=ls -la")
			}

			// Return approval
			return codex.CommandExecutionRequestApprovalResponse{
				Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "accept"},
			}, nil
		},
	}

	client.SetApprovalHandlers(handlers)

	ctx := context.Background()

	// Server sends approval request
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 99},
		Method:  "item/commandExecution/requestApproval",
		Params:  json.RawMessage(`{"itemId":"item-abc","threadId":"t1","turnId":"tu1","command":"ls -la"}`),
	})

	// Verify response was sent
	if len(mock.sentResponses) == 0 {
		t.Fatal("No response sent")
	}

	resp := mock.sentResponses[0]
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	// Verify response ID matches request ID
	if resp.ID.Value != 99 {
		t.Errorf("Response ID mismatch: expected 99, got %v", resp.ID.Value)
	}

	// Verify response result contains decision
	var result codex.CommandExecutionRequestApprovalResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result.Decision.Value != "accept" {
		t.Errorf("Expected decision=accept, got %v", result.Decision.Value)
	}
}

// TestApprovalHandlersCompleteness verifies all 8 server→client request handler fields exist
func TestApprovalHandlersCompleteness(t *testing.T) {
	// This test ensures ApprovalHandlers struct has all 8 handler fields
	// as specified in ServerRequest.json
	handlers := codex.ApprovalHandlers{
		OnApplyPatchApproval:              func(context.Context, codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) { return codex.ApplyPatchApprovalResponse{}, nil },
		OnCommandExecutionRequestApproval: func(context.Context, codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) { return codex.CommandExecutionRequestApprovalResponse{}, nil },
		OnExecCommandApproval:             func(context.Context, codex.ExecCommandApprovalParams) (codex.ExecCommandApprovalResponse, error) { return codex.ExecCommandApprovalResponse{}, nil },
		OnFileChangeRequestApproval:       func(context.Context, codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) { return codex.FileChangeRequestApprovalResponse{}, nil },
		OnSkillRequestApproval:            func(context.Context, codex.SkillRequestApprovalParams) (codex.SkillRequestApprovalResponse, error) { return codex.SkillRequestApprovalResponse{}, nil },
		OnDynamicToolCall:                 func(context.Context, codex.DynamicToolCallParams) (codex.DynamicToolCallResponse, error) { return codex.DynamicToolCallResponse{}, nil },
		OnToolRequestUserInput:            func(context.Context, codex.ToolRequestUserInputParams) (codex.ToolRequestUserInputResponse, error) { return codex.ToolRequestUserInputResponse{}, nil },
		OnChatgptAuthTokensRefresh:        func(context.Context, codex.ChatgptAuthTokensRefreshParams) (codex.ChatgptAuthTokensRefreshResponse, error) { return codex.ChatgptAuthTokensRefreshResponse{}, nil },
	}

	// Verify we can set handlers on client
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	client.SetApprovalHandlers(handlers)

	// Count: 8 server→client request types
	// 1. ApplyPatchApproval
	// 2. CommandExecutionRequestApproval
	// 3. ExecCommandApproval
	// 4. FileChangeRequestApproval
	// 5. SkillRequestApproval
	// 6. DynamicToolCall
	// 7. ToolRequestUserInput
	// 8. ChatgptAuthTokensRefresh

	// This test will fail to compile if any handler field is missing or has wrong signature
}
