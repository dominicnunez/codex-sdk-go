package exec

import (
	"context"
	"errors"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/sdk"
)

const jsonrpcVersion = "2.0"

const (
	ErrCodeParseError     = sdk.ErrCodeParseError
	ErrCodeInvalidRequest = sdk.ErrCodeInvalidRequest
	ErrCodeMethodNotFound = sdk.ErrCodeMethodNotFound
	ErrCodeInvalidParams  = sdk.ErrCodeInvalidParams
	ErrCodeInternalError  = sdk.ErrCodeInternalError
)

var (
	ErrNilContext           = sdk.ErrNilContext
	ErrInvalidParams        = sdk.ErrInvalidParams
	NewTransportError       = sdk.NewTransportError
	ApprovalPolicyNever     = sdk.ApprovalPolicyNever
	ApprovalPolicyOnFailure = sdk.ApprovalPolicyOnFailure
	ApprovalPolicyOnRequest = sdk.ApprovalPolicyOnRequest
	ApprovalPolicyUntrusted = sdk.ApprovalPolicyUntrusted
)

var errInvalidParams = ErrInvalidParams

type (
	Client                 = sdk.Client
	ClientInfo             = sdk.ClientInfo
	ClientOption           = sdk.ClientOption
	Error                  = sdk.Error
	InitializeCapabilities = sdk.InitializeCapabilities
	InitializeParams       = sdk.InitializeParams
	Notification           = sdk.Notification
	NotificationHandler    = sdk.NotificationHandler
	Request                = sdk.Request
	RequestHandler         = sdk.RequestHandler
	RequestID              = sdk.RequestID
	Response               = sdk.Response
	Transport              = sdk.Transport
	TransportError         = sdk.TransportError

	AgentMessageDeltaNotification           = sdk.AgentMessageDeltaNotification
	AgentMessageThreadItem                  = sdk.AgentMessageThreadItem
	AddPatchChangeKind                      = sdk.AddPatchChangeKind
	CommandExecutionOutputDeltaNotification = sdk.CommandExecutionOutputDeltaNotification
	ErrorNotification                       = sdk.ErrorNotification
	FileChangeOutputDeltaNotification       = sdk.FileChangeOutputDeltaNotification
	FileChangePatchUpdatedNotification      = sdk.FileChangePatchUpdatedNotification
	ItemCompletedNotification               = sdk.ItemCompletedNotification
	ItemStartedNotification                 = sdk.ItemStartedNotification
	PlanDeltaNotification                   = sdk.PlanDeltaNotification
	ReasoningSummaryTextDeltaNotification   = sdk.ReasoningSummaryTextDeltaNotification
	ReasoningTextDeltaNotification          = sdk.ReasoningTextDeltaNotification
	ThreadRealtimeErrorNotification         = sdk.ThreadRealtimeErrorNotification
	ThreadTokenUsageUpdatedNotification     = sdk.ThreadTokenUsageUpdatedNotification
	TurnCompletedNotification               = sdk.TurnCompletedNotification
	TurnStartedNotification                 = sdk.TurnStartedNotification

	AskForApproval                             = sdk.AskForApproval
	ApplyPatchApprovalParams                   = sdk.ApplyPatchApprovalParams
	ApplyPatchApprovalResponse                 = sdk.ApplyPatchApprovalResponse
	ApprovalHandlers                           = sdk.ApprovalHandlers
	CollabAgentState                           = sdk.CollabAgentState
	CollabAgentStatus                          = sdk.CollabAgentStatus
	CollabAgentTool                            = sdk.CollabAgentTool
	CollabAgentToolCallStatus                  = sdk.CollabAgentToolCallStatus
	CollabAgentToolCallThreadItem              = sdk.CollabAgentToolCallThreadItem
	CollaborationMode                          = sdk.CollaborationMode
	CollaborationModeSettings                  = sdk.CollaborationModeSettings
	CommandActionWrapper                       = sdk.CommandActionWrapper
	CommandExecutionApprovalDecisionWrapper    = sdk.CommandExecutionApprovalDecisionWrapper
	CommandExecutionRequestApprovalParams      = sdk.CommandExecutionRequestApprovalParams
	CommandExecutionRequestApprovalResponse    = sdk.CommandExecutionRequestApprovalResponse
	CommandExecutionStatus                     = sdk.CommandExecutionStatus
	CommandExecutionThreadItem                 = sdk.CommandExecutionThreadItem
	ContextCompactionThreadItem                = sdk.ContextCompactionThreadItem
	DeletePatchChangeKind                      = sdk.DeletePatchChangeKind
	DynamicToolCallOutputContentItemWrapper    = sdk.DynamicToolCallOutputContentItemWrapper
	DynamicToolCallThreadItem                  = sdk.DynamicToolCallThreadItem
	FileChangeThreadItem                       = sdk.FileChangeThreadItem
	FileChangeRequestApprovalParams            = sdk.FileChangeRequestApprovalParams
	FileChangeRequestApprovalResponse          = sdk.FileChangeRequestApprovalResponse
	FileUpdateChange                           = sdk.FileUpdateChange
	GitInfo                                    = sdk.GitInfo
	ImageUserInput                             = sdk.ImageUserInput
	ImageViewThreadItem                        = sdk.ImageViewThreadItem
	InputImageDynamicToolCallOutputContentItem = sdk.InputImageDynamicToolCallOutputContentItem
	InputTextDynamicToolCallOutputContentItem  = sdk.InputTextDynamicToolCallOutputContentItem
	ListFilesCommandAction                     = sdk.ListFilesCommandAction
	LocalImageUserInput                        = sdk.LocalImageUserInput
	MentionUserInput                           = sdk.MentionUserInput
	McpToolCallError                           = sdk.McpToolCallError
	McpToolCallResult                          = sdk.McpToolCallResult
	McpToolCallStatus                          = sdk.McpToolCallStatus
	McpToolCallThreadItem                      = sdk.McpToolCallThreadItem
	MessagePhase                               = sdk.MessagePhase
	ModeKind                                   = sdk.ModeKind
	PatchApplyStatus                           = sdk.PatchApplyStatus
	PatchChangeKindWrapper                     = sdk.PatchChangeKindWrapper
	Personality                                = sdk.Personality
	PlanThreadItem                             = sdk.PlanThreadItem
	ReasoningEffort                            = sdk.ReasoningEffort
	ReviewDecisionWrapper                      = sdk.ReviewDecisionWrapper
	ReasoningThreadItem                        = sdk.ReasoningThreadItem
	ReadCommandAction                          = sdk.ReadCommandAction
	SandboxMode                                = sdk.SandboxMode
	SearchCommandAction                        = sdk.SearchCommandAction
	SearchWebSearchAction                      = sdk.SearchWebSearchAction
	SkillUserInput                             = sdk.SkillUserInput
	SessionSource                              = sdk.SessionSource
	SessionSourceSubAgent                      = sdk.SessionSourceSubAgent
	SessionSourceWrapper                       = sdk.SessionSourceWrapper
	SubAgentSource                             = sdk.SubAgentSource
	SubAgentSourceOther                        = sdk.SubAgentSourceOther
	SubAgentSourceThreadSpawn                  = sdk.SubAgentSourceThreadSpawn
	TextElement                                = sdk.TextElement
	TextUserInput                              = sdk.TextUserInput
	Thread                                     = sdk.Thread
	ThreadActiveFlag                           = sdk.ThreadActiveFlag
	ThreadItem                                 = sdk.ThreadItem
	ThreadItemWrapper                          = sdk.ThreadItemWrapper
	ThreadReadParams                           = sdk.ThreadReadParams
	ThreadStartParams                          = sdk.ThreadStartParams
	ThreadStatusActive                         = sdk.ThreadStatusActive
	ThreadStatusIdle                           = sdk.ThreadStatusIdle
	ThreadStatusNotLoaded                      = sdk.ThreadStatusNotLoaded
	ThreadStatusSystemError                    = sdk.ThreadStatusSystemError
	ThreadStatusWrapper                        = sdk.ThreadStatusWrapper
	ThreadTokenUsage                           = sdk.ThreadTokenUsage
	Turn                                       = sdk.Turn
	TurnError                                  = sdk.TurnError
	TurnStartParams                            = sdk.TurnStartParams
	TurnStatus                                 = sdk.TurnStatus
	UnknownCommandAction                       = sdk.UnknownCommandAction
	UnknownDynamicToolCallOutputContentItem    = sdk.UnknownDynamicToolCallOutputContentItem
	UnknownPatchChangeKind                     = sdk.UnknownPatchChangeKind
	UnknownSessionSource                       = sdk.UnknownSessionSource
	UnknownSubAgentSource                      = sdk.UnknownSubAgentSource
	UnknownThreadItem                          = sdk.UnknownThreadItem
	UnknownThreadStatus                        = sdk.UnknownThreadStatus
	UnknownUserInput                           = sdk.UnknownUserInput
	UpdatePatchChangeKind                      = sdk.UpdatePatchChangeKind
	UserInput                                  = sdk.UserInput
	UserMessageThreadItem                      = sdk.UserMessageThreadItem
	EnteredReviewModeThreadItem                = sdk.EnteredReviewModeThreadItem
	ExitedReviewModeThreadItem                 = sdk.ExitedReviewModeThreadItem
	OpenPageWebSearchAction                    = sdk.OpenPageWebSearchAction
	FindInPageWebSearchAction                  = sdk.FindInPageWebSearchAction
	OtherWebSearchAction                       = sdk.OtherWebSearchAction
	UnknownWebSearchAction                     = sdk.UnknownWebSearchAction
	WebSearchActionWrapper                     = sdk.WebSearchActionWrapper
	WebSearchThreadItem                        = sdk.WebSearchThreadItem
)

const (
	CollabAgentStatusCompleted          = sdk.CollabAgentStatusCompleted
	CollabAgentStatusErrored            = sdk.CollabAgentStatusErrored
	CollabAgentStatusInterrupted        = sdk.CollabAgentStatusInterrupted
	CollabAgentStatusNotFound           = sdk.CollabAgentStatusNotFound
	CollabAgentStatusPendingInit        = sdk.CollabAgentStatusPendingInit
	CollabAgentStatusRunning            = sdk.CollabAgentStatusRunning
	CollabAgentStatusShutdown           = sdk.CollabAgentStatusShutdown
	CollabAgentToolCallStatusCompleted  = sdk.CollabAgentToolCallStatusCompleted
	CollabAgentToolCallStatusFailed     = sdk.CollabAgentToolCallStatusFailed
	CollabAgentToolCallStatusInProgress = sdk.CollabAgentToolCallStatusInProgress
	CollabAgentToolCloseAgent           = sdk.CollabAgentToolCloseAgent
	CollabAgentToolResumeAgent          = sdk.CollabAgentToolResumeAgent
	CollabAgentToolSendInput            = sdk.CollabAgentToolSendInput
	CollabAgentToolSpawnAgent           = sdk.CollabAgentToolSpawnAgent
	CollabAgentToolWait                 = sdk.CollabAgentToolWait
	CommandExecutionStatusCompleted     = sdk.CommandExecutionStatusCompleted
	CommandExecutionStatusFailed        = sdk.CommandExecutionStatusFailed
	CommandExecutionStatusInProgress    = sdk.CommandExecutionStatusInProgress
	McpToolCallStatusCompleted          = sdk.McpToolCallStatusCompleted
	McpToolCallStatusFailed             = sdk.McpToolCallStatusFailed
	McpToolCallStatusInProgress         = sdk.McpToolCallStatusInProgress
	ModeKindPlan                        = sdk.ModeKindPlan
	PatchApplyStatusCompleted           = sdk.PatchApplyStatusCompleted
	PatchApplyStatusFailed              = sdk.PatchApplyStatusFailed
	PatchApplyStatusInProgress          = sdk.PatchApplyStatusInProgress
	PersonalityFriendly                 = sdk.PersonalityFriendly
	ReasoningEffortHigh                 = sdk.ReasoningEffortHigh
	SandboxModeDangerFullAccess         = sdk.SandboxModeDangerFullAccess
	SandboxModeReadOnly                 = sdk.SandboxModeReadOnly
	SandboxModeWorkspaceWrite           = sdk.SandboxModeWorkspaceWrite
	ThreadActiveFlagWaitingOnApproval   = sdk.ThreadActiveFlagWaitingOnApproval
	ThreadActiveFlagWaitingOnUserInput  = sdk.ThreadActiveFlagWaitingOnUserInput
	TurnStatusCompleted                 = sdk.TurnStatusCompleted
	TurnStatusFailed                    = sdk.TurnStatusFailed
	TurnStatusInterrupted               = sdk.TurnStatusInterrupted
	UnmarshalErrorItemType              = sdk.UnmarshalErrorItemType
)

func NewClient(transport Transport, opts ...ClientOption) *Client {
	return sdk.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return sdk.WithRequestTimeout(timeout)
}

func WithHandlerErrorCallback(cb func(method string, err error)) ClientOption {
	return sdk.WithHandlerErrorCallback(cb)
}

func Ptr[T any](v T) *T {
	return sdk.Ptr(v)
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}

func cloneInitializeParams(params InitializeParams) InitializeParams {
	cp := params
	cp.ClientInfo = params.ClientInfo
	cp.ClientInfo.Title = cloneStringPtr(params.ClientInfo.Title)
	if params.Capabilities != nil {
		capabilities := *params.Capabilities
		capabilities.OptOutNotificationMethods = append([]string(nil), params.Capabilities.OptOutNotificationMethods...)
		cp.Capabilities = &capabilities
	}
	return cp
}

func isTransportError(err error) bool {
	var transportErr *TransportError
	return errors.As(err, &transportErr)
}
