package codex

import "context"

// ApprovalHandlers contains optional callback functions for all server→client approval requests.
// If a handler is not set, the client will return a JSON-RPC method-not-found error when
// the server sends that request type.
type ApprovalHandlers struct {
	OnApplyPatchApproval              func(context.Context, ApplyPatchApprovalParams) (ApplyPatchApprovalResponse, error)
	OnCommandExecutionRequestApproval func(context.Context, CommandExecutionRequestApprovalParams) (CommandExecutionRequestApprovalResponse, error)
	OnExecCommandApproval             func(context.Context, ExecCommandApprovalParams) (ExecCommandApprovalResponse, error)
	OnFileChangeRequestApproval       func(context.Context, FileChangeRequestApprovalParams) (FileChangeRequestApprovalResponse, error)
	OnPermissionsRequestApproval      func(context.Context, PermissionsRequestApprovalParams) (PermissionsRequestApprovalResponse, error)
	OnDynamicToolCall                 func(context.Context, DynamicToolCallParams) (DynamicToolCallResponse, error)
	OnToolRequestUserInput            func(context.Context, ToolRequestUserInputParams) (ToolRequestUserInputResponse, error)
	OnChatgptAuthTokensRefresh        func(context.Context, ChatgptAuthTokensRefreshParams) (ChatgptAuthTokensRefreshResponse, error)
	OnMcpServerElicitationRequest     func(context.Context, McpServerElicitationRequestParams) (McpServerElicitationRequestResponse, error)
}

// SetApprovalHandlers registers approval handlers on the client for server→client requests.
func (c *Client) SetApprovalHandlers(handlers ApprovalHandlers) {
	c.approvalMu.Lock()
	defer c.approvalMu.Unlock()
	c.approvalHandlers = handlers
}
