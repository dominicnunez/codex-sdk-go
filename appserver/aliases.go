// Package appserver starts and manages a Codex app-server process and provides
// app-server-backed run helpers.
package appserver

import (
	"time"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const sdkVersion = "0.3.0"

const (
	ErrCodeParseError     = protocol.ErrCodeParseError
	ErrCodeInvalidRequest = protocol.ErrCodeInvalidRequest
	ErrCodeMethodNotFound = protocol.ErrCodeMethodNotFound
	ErrCodeInvalidParams  = protocol.ErrCodeInvalidParams
	ErrCodeInternalError  = protocol.ErrCodeInternalError
)

var (
	ErrNilContext           = protocol.ErrNilContext
	ErrInvalidParams        = protocol.ErrInvalidParams
	NewTransportError       = protocol.NewTransportError
	ApprovalPolicyNever     = protocol.ApprovalPolicyNever
	ApprovalPolicyOnFailure = protocol.ApprovalPolicyOnFailure
	ApprovalPolicyOnRequest = protocol.ApprovalPolicyOnRequest
	ApprovalPolicyUntrusted = protocol.ApprovalPolicyUntrusted
)

type (
	Client                                     = protocol.Client
	ClientInfo                                 = protocol.ClientInfo
	ClientOption                               = protocol.ClientOption
	Error                                      = protocol.Error
	InitializeCapabilities                     = protocol.InitializeCapabilities
	InitializeParams                           = protocol.InitializeParams
	InitializeResponse                         = protocol.InitializeResponse
	Notification                               = protocol.Notification
	NotificationHandler                        = protocol.NotificationHandler
	Request                                    = protocol.Request
	RequestHandler                             = protocol.RequestHandler
	RequestID                                  = protocol.RequestID
	Response                                   = protocol.Response
	Transport                                  = protocol.Transport
	TransportError                             = protocol.TransportError
	AgentMessageDeltaNotification              = protocol.AgentMessageDeltaNotification
	AgentMessageThreadItem                     = protocol.AgentMessageThreadItem
	AddPatchChangeKind                         = protocol.AddPatchChangeKind
	CommandExecutionOutputDeltaNotification    = protocol.CommandExecutionOutputDeltaNotification
	ErrorNotification                          = protocol.ErrorNotification
	FileChangeOutputDeltaNotification          = protocol.FileChangeOutputDeltaNotification
	FileChangePatchUpdatedNotification         = protocol.FileChangePatchUpdatedNotification
	ItemCompletedNotification                  = protocol.ItemCompletedNotification
	ItemStartedNotification                    = protocol.ItemStartedNotification
	PlanDeltaNotification                      = protocol.PlanDeltaNotification
	ReasoningSummaryTextDeltaNotification      = protocol.ReasoningSummaryTextDeltaNotification
	ReasoningTextDeltaNotification             = protocol.ReasoningTextDeltaNotification
	ThreadRealtimeErrorNotification            = protocol.ThreadRealtimeErrorNotification
	ThreadTokenUsageUpdatedNotification        = protocol.ThreadTokenUsageUpdatedNotification
	TurnCompletedNotification                  = protocol.TurnCompletedNotification
	TurnStartedNotification                    = protocol.TurnStartedNotification
	AskForApproval                             = protocol.AskForApproval
	ApprovalHandlers                           = protocol.ApprovalHandlers
	CollabAgentState                           = protocol.CollabAgentState
	CollabAgentStatus                          = protocol.CollabAgentStatus
	CollabAgentTool                            = protocol.CollabAgentTool
	CollabAgentToolCallStatus                  = protocol.CollabAgentToolCallStatus
	CollabAgentToolCallThreadItem              = protocol.CollabAgentToolCallThreadItem
	CollaborationMode                          = protocol.CollaborationMode
	CollaborationModeSettings                  = protocol.CollaborationModeSettings
	CommandActionWrapper                       = protocol.CommandActionWrapper
	CommandExecutionApprovalDecisionWrapper    = protocol.CommandExecutionApprovalDecisionWrapper
	CommandExecutionRequestApprovalParams      = protocol.CommandExecutionRequestApprovalParams
	CommandExecutionRequestApprovalResponse    = protocol.CommandExecutionRequestApprovalResponse
	CommandExecutionStatus                     = protocol.CommandExecutionStatus
	CommandExecutionThreadItem                 = protocol.CommandExecutionThreadItem
	ContextCompactionThreadItem                = protocol.ContextCompactionThreadItem
	DeletePatchChangeKind                      = protocol.DeletePatchChangeKind
	DynamicToolCallOutputContentItemWrapper    = protocol.DynamicToolCallOutputContentItemWrapper
	DynamicToolCallThreadItem                  = protocol.DynamicToolCallThreadItem
	FileChangeThreadItem                       = protocol.FileChangeThreadItem
	FileChangeRequestApprovalParams            = protocol.FileChangeRequestApprovalParams
	FileChangeRequestApprovalResponse          = protocol.FileChangeRequestApprovalResponse
	FileUpdateChange                           = protocol.FileUpdateChange
	GitInfo                                    = protocol.GitInfo
	ImageUserInput                             = protocol.ImageUserInput
	ImageViewThreadItem                        = protocol.ImageViewThreadItem
	InputImageDynamicToolCallOutputContentItem = protocol.InputImageDynamicToolCallOutputContentItem
	InputTextDynamicToolCallOutputContentItem  = protocol.InputTextDynamicToolCallOutputContentItem
	ListFilesCommandAction                     = protocol.ListFilesCommandAction
	LocalImageUserInput                        = protocol.LocalImageUserInput
	MentionUserInput                           = protocol.MentionUserInput
	McpToolCallError                           = protocol.McpToolCallError
	McpToolCallResult                          = protocol.McpToolCallResult
	McpToolCallStatus                          = protocol.McpToolCallStatus
	McpToolCallThreadItem                      = protocol.McpToolCallThreadItem
	MessagePhase                               = protocol.MessagePhase
	ModeKind                                   = protocol.ModeKind
	PatchApplyStatus                           = protocol.PatchApplyStatus
	PatchChangeKindWrapper                     = protocol.PatchChangeKindWrapper
	Personality                                = protocol.Personality
	PlanThreadItem                             = protocol.PlanThreadItem
	ReasoningEffort                            = protocol.ReasoningEffort
	ReasoningThreadItem                        = protocol.ReasoningThreadItem
	ReadCommandAction                          = protocol.ReadCommandAction
	ReviewDecisionWrapper                      = protocol.ReviewDecisionWrapper
	SandboxMode                                = protocol.SandboxMode
	SandboxPolicy                              = protocol.SandboxPolicy
	SandboxPolicyDangerFullAccess              = protocol.SandboxPolicyDangerFullAccess
	SandboxPolicyExternalSandbox               = protocol.SandboxPolicyExternalSandbox
	SandboxPolicyReadOnly                      = protocol.SandboxPolicyReadOnly
	SandboxPolicyWorkspaceWrite                = protocol.SandboxPolicyWorkspaceWrite
	ReadOnlyAccess                             = protocol.ReadOnlyAccess
	ReadOnlyAccessFullAccess                   = protocol.ReadOnlyAccessFullAccess
	ReadOnlyAccessRestricted                   = protocol.ReadOnlyAccessRestricted
	ReadOnlyAccessWrapper                      = protocol.ReadOnlyAccessWrapper
	NetworkAccess                              = protocol.NetworkAccess
	SearchCommandAction                        = protocol.SearchCommandAction
	SearchWebSearchAction                      = protocol.SearchWebSearchAction
	SkillUserInput                             = protocol.SkillUserInput
	SessionSource                              = protocol.SessionSource
	SessionSourceSubAgent                      = protocol.SessionSourceSubAgent
	SessionSourceWrapper                       = protocol.SessionSourceWrapper
	SubAgentSource                             = protocol.SubAgentSource
	SubAgentSourceOther                        = protocol.SubAgentSourceOther
	SubAgentSourceThreadSpawn                  = protocol.SubAgentSourceThreadSpawn
	TextElement                                = protocol.TextElement
	TextUserInput                              = protocol.TextUserInput
	Thread                                     = protocol.Thread
	ThreadActiveFlag                           = protocol.ThreadActiveFlag
	ThreadItem                                 = protocol.ThreadItem
	ThreadItemWrapper                          = protocol.ThreadItemWrapper
	ThreadReadParams                           = protocol.ThreadReadParams
	ThreadStartParams                          = protocol.ThreadStartParams
	ThreadStatusActive                         = protocol.ThreadStatusActive
	ThreadStatusIdle                           = protocol.ThreadStatusIdle
	ThreadStatusNotLoaded                      = protocol.ThreadStatusNotLoaded
	ThreadStatusSystemError                    = protocol.ThreadStatusSystemError
	ThreadStatusWrapper                        = protocol.ThreadStatusWrapper
	ThreadTokenUsage                           = protocol.ThreadTokenUsage
	Turn                                       = protocol.Turn
	TurnError                                  = protocol.TurnError
	TurnStartParams                            = protocol.TurnStartParams
	TurnStatus                                 = protocol.TurnStatus
	UnknownCommandAction                       = protocol.UnknownCommandAction
	UnknownDynamicToolCallOutputContentItem    = protocol.UnknownDynamicToolCallOutputContentItem
	UnknownPatchChangeKind                     = protocol.UnknownPatchChangeKind
	UnknownSessionSource                       = protocol.UnknownSessionSource
	UnknownSubAgentSource                      = protocol.UnknownSubAgentSource
	UnknownThreadItem                          = protocol.UnknownThreadItem
	UnknownThreadStatus                        = protocol.UnknownThreadStatus
	UnknownUserInput                           = protocol.UnknownUserInput
	UpdatePatchChangeKind                      = protocol.UpdatePatchChangeKind
	UserInput                                  = protocol.UserInput
	UserMessageThreadItem                      = protocol.UserMessageThreadItem
	EnteredReviewModeThreadItem                = protocol.EnteredReviewModeThreadItem
	ExitedReviewModeThreadItem                 = protocol.ExitedReviewModeThreadItem
	OpenPageWebSearchAction                    = protocol.OpenPageWebSearchAction
	FindInPageWebSearchAction                  = protocol.FindInPageWebSearchAction
	OtherWebSearchAction                       = protocol.OtherWebSearchAction
	UnknownWebSearchAction                     = protocol.UnknownWebSearchAction
	WebSearchActionWrapper                     = protocol.WebSearchActionWrapper
	WebSearchThreadItem                        = protocol.WebSearchThreadItem
)

const (
	CollabAgentStatusCompleted          = protocol.CollabAgentStatusCompleted
	CollabAgentStatusErrored            = protocol.CollabAgentStatusErrored
	CollabAgentStatusInterrupted        = protocol.CollabAgentStatusInterrupted
	CollabAgentStatusNotFound           = protocol.CollabAgentStatusNotFound
	CollabAgentStatusPendingInit        = protocol.CollabAgentStatusPendingInit
	CollabAgentStatusRunning            = protocol.CollabAgentStatusRunning
	CollabAgentStatusShutdown           = protocol.CollabAgentStatusShutdown
	CollabAgentToolCallStatusCompleted  = protocol.CollabAgentToolCallStatusCompleted
	CollabAgentToolCallStatusFailed     = protocol.CollabAgentToolCallStatusFailed
	CollabAgentToolCallStatusInProgress = protocol.CollabAgentToolCallStatusInProgress
	CollabAgentToolCloseAgent           = protocol.CollabAgentToolCloseAgent
	CollabAgentToolResumeAgent          = protocol.CollabAgentToolResumeAgent
	CollabAgentToolSendInput            = protocol.CollabAgentToolSendInput
	CollabAgentToolSpawnAgent           = protocol.CollabAgentToolSpawnAgent
	CollabAgentToolWait                 = protocol.CollabAgentToolWait
	CommandExecutionStatusCompleted     = protocol.CommandExecutionStatusCompleted
	CommandExecutionStatusFailed        = protocol.CommandExecutionStatusFailed
	CommandExecutionStatusInProgress    = protocol.CommandExecutionStatusInProgress
	McpToolCallStatusCompleted          = protocol.McpToolCallStatusCompleted
	McpToolCallStatusFailed             = protocol.McpToolCallStatusFailed
	McpToolCallStatusInProgress         = protocol.McpToolCallStatusInProgress
	ModeKindPlan                        = protocol.ModeKindPlan
	PatchApplyStatusCompleted           = protocol.PatchApplyStatusCompleted
	PatchApplyStatusFailed              = protocol.PatchApplyStatusFailed
	PatchApplyStatusInProgress          = protocol.PatchApplyStatusInProgress
	PersonalityFriendly                 = protocol.PersonalityFriendly
	ReasoningEffortHigh                 = protocol.ReasoningEffortHigh
	NetworkAccessEnabled                = protocol.NetworkAccessEnabled
	NetworkAccessRestricted             = protocol.NetworkAccessRestricted
	SandboxModeDangerFullAccess         = protocol.SandboxModeDangerFullAccess
	SandboxModeReadOnly                 = protocol.SandboxModeReadOnly
	SandboxModeWorkspaceWrite           = protocol.SandboxModeWorkspaceWrite
	ThreadActiveFlagWaitingOnApproval   = protocol.ThreadActiveFlagWaitingOnApproval
	ThreadActiveFlagWaitingOnUserInput  = protocol.ThreadActiveFlagWaitingOnUserInput
	TurnStatusCompleted                 = protocol.TurnStatusCompleted
	TurnStatusFailed                    = protocol.TurnStatusFailed
	TurnStatusInterrupted               = protocol.TurnStatusInterrupted
	UnmarshalErrorItemType              = protocol.UnmarshalErrorItemType
)

func NewClient(transport Transport, opts ...ClientOption) *Client {
	return protocol.NewClient(transport, opts...)
}

func WithRequestTimeout(timeout time.Duration) ClientOption {
	return protocol.WithRequestTimeout(timeout)
}

func WithHandlerErrorCallback(cb func(method string, err error)) ClientOption {
	return protocol.WithHandlerErrorCallback(cb)
}

func Ptr[T any](v T) *T {
	return protocol.Ptr(v)
}
