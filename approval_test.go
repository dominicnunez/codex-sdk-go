package codex_test

import (
	"context"
	"encoding/json"
	"strings"
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
		{
			name:         "unknown_object_variant",
			responseJSON: `{"decision":{"future_amendment_type":{"key":"value"}}}`,
			checkFunc: func(r codex.ApplyPatchApprovalResponse) bool {
				_, ok := r.Decision.Value.(codex.UnknownReviewDecision)
				return ok
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

			// Verify round-trip
			marshaled, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("Marshal response: %v", err)
			}
			var roundtrip codex.ApplyPatchApprovalResponse
			if err := json.Unmarshal(marshaled, &roundtrip); err != nil {
				t.Fatalf("Unmarshal roundtrip: %v", err)
			}
			if !tc.checkFunc(roundtrip) {
				t.Errorf("roundtrip decision check failed")
			}
		})
	}
}

func TestNetworkApprovalContextRejectsInvalidProtocol(t *testing.T) {
	var ctx codex.NetworkApprovalContext
	err := json.Unmarshal([]byte(`{"host":"example.com","protocol":"ftp"}`), &ctx)
	if err == nil {
		t.Fatal("expected invalid protocol error")
	}
	if !strings.Contains(err.Error(), `invalid protocol "ftp"`) {
		t.Fatalf("error = %v; want invalid protocol", err)
	}
}

func TestChatgptAuthTokensRefreshParamsRejectInvalidReason(t *testing.T) {
	var params codex.ChatgptAuthTokensRefreshParams
	err := json.Unmarshal([]byte(`{"reason":"expired"}`), &params)
	if err == nil {
		t.Fatal("expected invalid reason error")
	}
	if !strings.Contains(err.Error(), `invalid reason "expired"`) {
		t.Fatalf("error = %v; want invalid reason", err)
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
		{
			name:         "unknown_object_variant",
			responseJSON: `{"decision":{"future_decision_type":{"data":123}}}`,
			checkFunc: func(r codex.CommandExecutionRequestApprovalResponse) bool {
				_, ok := r.Decision.Value.(codex.UnknownCommandExecutionApprovalDecision)
				return ok
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

			// Verify round-trip
			marshaled, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("Marshal response: %v", err)
			}
			var roundtrip codex.CommandExecutionRequestApprovalResponse
			if err := json.Unmarshal(marshaled, &roundtrip); err != nil {
				t.Fatalf("Unmarshal roundtrip: %v", err)
			}
			if !tc.checkFunc(roundtrip) {
				t.Errorf("roundtrip decision check failed")
			}
		})
	}
}

func TestMcpServerElicitationRequestParamsRejectInvalidVariants(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantText string
	}{
		{
			name:     "form missing requestedSchema",
			payload:  `{"serverName":"demo-server","threadId":"thread-1","message":"Enter credentials","mode":"form"}`,
			wantText: `requestedSchema`,
		},
		{
			name:     "url missing elicitationId",
			payload:  `{"serverName":"demo-server","threadId":"thread-1","message":"Open browser","mode":"url","url":"https://example.com"}`,
			wantText: `elicitationId`,
		},
		{
			name:     "unknown mode",
			payload:  `{"serverName":"demo-server","threadId":"thread-1","message":"Unknown","mode":"modal"}`,
			wantText: `unsupported elicitation mode`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params codex.McpServerElicitationRequestParams
			err := json.Unmarshal([]byte(tt.payload), &params)
			if err == nil {
				t.Fatal("expected unmarshal error")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("error = %v; want %q", err, tt.wantText)
			}
		})
	}
}

func TestApprovalDecisionWrappersMarshalPointerBackedVariants(t *testing.T) {
	t.Run("review decision wrapper", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
			want  string
		}{
			{
				name: "approved execpolicy amendment pointer",
				value: &codex.ApprovedExecpolicyAmendmentDecision{
					ProposedExecpolicyAmendment: []string{"allow read"},
				},
				want: `{"approved_execpolicy_amendment":{"proposed_execpolicy_amendment":["allow read"]}}`,
			},
			{
				name: "network policy amendment pointer",
				value: &codex.NetworkPolicyAmendmentDecision{
					NetworkPolicyAmendment: codex.NetworkPolicyAmendment{
						Action: codex.NetworkPolicyRuleActionAllow,
						Host:   "example.com",
					},
				},
				want: `{"network_policy_amendment":{"network_policy_amendment":{"action":"allow","host":"example.com"}}}`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := json.Marshal(codex.ReviewDecisionWrapper{Value: tt.value})
				if err != nil {
					t.Fatalf("MarshalJSON() error = %v", err)
				}
				if got := string(data); got != tt.want {
					t.Fatalf("MarshalJSON() = %s; want %s", got, tt.want)
				}
			})
		}
	})

	t.Run("command execution approval decision wrapper", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
			want  string
		}{
			{
				name: "accept with execpolicy amendment pointer",
				value: &codex.AcceptWithExecpolicyAmendmentDecision{
					ExecpolicyAmendment: []string{"allow git status"},
				},
				want: `{"acceptWithExecpolicyAmendment":{"execpolicy_amendment":["allow git status"]}}`,
			},
			{
				name: "apply network policy amendment pointer",
				value: &codex.ApplyNetworkPolicyAmendmentDecision{
					NetworkPolicyAmendment: codex.NetworkPolicyAmendment{
						Action: codex.NetworkPolicyRuleActionDeny,
						Host:   "blocked.example.com",
					},
				},
				want: `{"applyNetworkPolicyAmendment":{"network_policy_amendment":{"action":"deny","host":"blocked.example.com"}}}`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := json.Marshal(codex.CommandExecutionApprovalDecisionWrapper{Value: tt.value})
				if err != nil {
					t.Fatalf("MarshalJSON() error = %v", err)
				}
				if got := string(data); got != tt.want {
					t.Fatalf("MarshalJSON() = %s; want %s", got, tt.want)
				}
			})
		}
	})
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
	var toolCallCalled bool
	var permissionsCalled bool
	var userInputCalled bool
	var authRefreshCalled bool
	var mcpElicitationCalled bool

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
		OnDynamicToolCall: func(ctx context.Context, params codex.DynamicToolCallParams) (codex.DynamicToolCallResponse, error) {
			toolCallCalled = true
			return codex.DynamicToolCallResponse{
				Success: true,
				ContentItems: []codex.DynamicToolCallOutputContentItemWrapper{
					{Value: &codex.InputTextDynamicToolCallOutputContentItem{Text: "result"}},
				},
			}, nil
		},
		OnPermissionsRequestApproval: func(ctx context.Context, params codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
			permissionsCalled = true
			scope := codex.PermissionGrantScopeSession
			return codex.PermissionsRequestApprovalResponse{
				Permissions: codex.GrantedPermissionProfile{
					Network: &codex.AdditionalNetworkPermissions{Enabled: ptr(true)},
				},
				Scope: &scope,
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
				AccessToken:      "new-token",
				ChatgptAccountID: "account-123",
				ChatgptPlanType:  ptr("plus"),
			}, nil
		},
		OnMcpServerElicitationRequest: func(ctx context.Context, params codex.McpServerElicitationRequestParams) (codex.McpServerElicitationRequestResponse, error) {
			mcpElicitationCalled = true
			return codex.McpServerElicitationRequestResponse{
				Action:  codex.McpServerElicitationActionAccept,
				Content: map[string]interface{}{"token": "abc123"},
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

	// 5. DynamicToolCall
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 5},
		Method:  "item/tool/call",
		Params:  json.RawMessage(`{"tool":"test","arguments":{},"callId":"c1","threadId":"t1","turnId":"tu1"}`),
	})

	// 6. ToolRequestUserInput
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 6},
		Method:  "item/tool/requestUserInput",
		Params:  json.RawMessage(`{"itemId":"i1","threadId":"t1","turnId":"tu1","questions":[{"id":"q1","header":"H","question":"Q"}]}`),
	})

	// 7. PermissionsRequestApproval
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 7},
		Method:  "item/permissions/requestApproval",
		Params:  json.RawMessage(`{"itemId":"i1","threadId":"t1","turnId":"tu1","permissions":{"network":{"enabled":true}}}`),
	})

	// 8. ChatgptAuthTokensRefresh
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 8},
		Method:  "account/chatgptAuthTokens/refresh",
		Params:  json.RawMessage(`{"reason":"unauthorized"}`),
	})

	// 9. McpServerElicitationRequest
	_, _ = mock.InjectServerRequest(ctx, codex.Request{
		JSONRPC: "2.0",
		ID:      codex.RequestID{Value: 9},
		Method:  "mcpServer/elicitation/request",
		Params:  json.RawMessage(`{"serverName":"demo","threadId":"t1","message":"Need input","mode":"url","elicitationId":"e1","url":"https://example.com"}`),
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
	if !toolCallCalled {
		t.Error("DynamicToolCall handler not called")
	}
	if !permissionsCalled {
		t.Error("PermissionsRequestApproval handler not called")
	}
	if !userInputCalled {
		t.Error("ToolRequestUserInput handler not called")
	}
	if !authRefreshCalled {
		t.Error("ChatgptAuthTokensRefresh handler not called")
	}
	if !mcpElicitationCalled {
		t.Error("McpServerElicitationRequest handler not called")
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

func TestAdditionalApprovalEndToEnd(t *testing.T) {
	t.Run("permissions request approval", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		client.SetApprovalHandlers(codex.ApprovalHandlers{
			OnPermissionsRequestApproval: func(ctx context.Context, params codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
				if params.ItemID != "item-abc" {
					t.Fatalf("Expected itemId=item-abc, got %s", params.ItemID)
				}
				if params.Permissions.Network == nil || params.Permissions.Network.Enabled == nil || !*params.Permissions.Network.Enabled {
					t.Fatalf("Expected requested network permission to be enabled, got %#v", params.Permissions.Network)
				}

				scope := codex.PermissionGrantScopeTurn
				return codex.PermissionsRequestApprovalResponse{
					Permissions: codex.GrantedPermissionProfile{
						FileSystem: &codex.AdditionalFileSystemPermissions{
							Read: []string{"/tmp/project"},
						},
					},
					Scope: &scope,
				}, nil
			},
		})

		resp, err := mock.InjectServerRequest(context.Background(), codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: 101},
			Method:  "item/permissions/requestApproval",
			Params:  json.RawMessage(`{"itemId":"item-abc","threadId":"t1","turnId":"tu1","permissions":{"network":{"enabled":true}}}`),
		})
		if err != nil {
			t.Fatalf("InjectServerRequest() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("Unexpected error response: %+v", resp.Error)
		}

		var result codex.PermissionsRequestApprovalResponse
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Unmarshal permissions response: %v", err)
		}
		if result.Scope == nil || *result.Scope != codex.PermissionGrantScopeTurn {
			t.Fatalf("Scope = %#v; want %q", result.Scope, codex.PermissionGrantScopeTurn)
		}
		if result.Permissions.FileSystem == nil || len(result.Permissions.FileSystem.Read) != 1 || result.Permissions.FileSystem.Read[0] != "/tmp/project" {
			t.Fatalf("Permissions = %#v; want granted read path", result.Permissions)
		}
	})

	t.Run("mcp server elicitation request", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)

		client.SetApprovalHandlers(codex.ApprovalHandlers{
			OnMcpServerElicitationRequest: func(ctx context.Context, params codex.McpServerElicitationRequestParams) (codex.McpServerElicitationRequestResponse, error) {
				if params.ServerName != "demo-server" {
					t.Fatalf("Expected serverName=demo-server, got %s", params.ServerName)
				}
				if params.Mode != codex.McpServerElicitationModeForm {
					t.Fatalf("Expected mode=form, got %q", params.Mode)
				}

				return codex.McpServerElicitationRequestResponse{
					Action: codex.McpServerElicitationActionAccept,
					Content: map[string]interface{}{
						"apiKey": "secret-token",
					},
				}, nil
			},
		})

		resp, err := mock.InjectServerRequest(context.Background(), codex.Request{
			JSONRPC: "2.0",
			ID:      codex.RequestID{Value: 102},
			Method:  "mcpServer/elicitation/request",
			Params: json.RawMessage(
				`{"serverName":"demo-server","threadId":"t1","message":"Enter credentials","mode":"form","requestedSchema":{"type":"object","properties":{"apiKey":{"type":"string"}},"required":["apiKey"]}}`,
			),
		})
		if err != nil {
			t.Fatalf("InjectServerRequest() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("Unexpected error response: %+v", resp.Error)
		}

		var result codex.McpServerElicitationRequestResponse
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Unmarshal MCP elicitation response: %v", err)
		}
		if result.Action != codex.McpServerElicitationActionAccept {
			t.Fatalf("Action = %q; want %q", result.Action, codex.McpServerElicitationActionAccept)
		}

		content, ok := result.Content.(map[string]interface{})
		if !ok {
			t.Fatalf("Content type = %T; want map[string]interface{}", result.Content)
		}
		if got := content["apiKey"]; got != "secret-token" {
			t.Fatalf("Content[apiKey] = %#v; want %q", got, "secret-token")
		}
	})
}

func TestDynamicToolCallOutputContentItemWrapperRejectsMalformedPayloads(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantErrPart string
	}{
		{
			name:        "missing type",
			payload:     `{}`,
			wantErrPart: `"type"`,
		},
		{
			name:        "input text missing text",
			payload:     `{"type":"inputText"}`,
			wantErrPart: `"text"`,
		},
		{
			name:        "input image missing imageUrl",
			payload:     `{"type":"inputImage"}`,
			wantErrPart: `"imageUrl"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item codex.DynamicToolCallOutputContentItemWrapper
			err := json.Unmarshal([]byte(tt.payload), &item)
			if err == nil {
				t.Fatal("expected unmarshal error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error = %v; want substring %q", err, tt.wantErrPart)
			}
		})
	}
}
