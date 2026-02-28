package codex

// This file contains typed string enums defined in the protocol spec that are
// referenced by fields across multiple domain files.

// TurnStatus represents the status of a turn.
type TurnStatus string

const (
	TurnStatusCompleted   TurnStatus = "completed"
	TurnStatusInterrupted TurnStatus = "interrupted"
	TurnStatusFailed      TurnStatus = "failed"
	TurnStatusInProgress  TurnStatus = "inProgress"
)

// TurnAbortReason represents the reason a turn was aborted.
type TurnAbortReason string

const (
	TurnAbortReasonInterrupted  TurnAbortReason = "interrupted"
	TurnAbortReasonReplaced     TurnAbortReason = "replaced"
	TurnAbortReasonReviewEnded  TurnAbortReason = "review_ended"
)

// Personality represents the assistant personality style.
type Personality string

const (
	PersonalityNone     Personality = "none"
	PersonalityFriendly Personality = "friendly"
	PersonalityPragmatic Personality = "pragmatic"
)

// ModeKind represents the initial collaboration mode.
type ModeKind string

const (
	ModeKindPlan    ModeKind = "plan"
	ModeKindDefault ModeKind = "default"
)

// MergeStrategy represents the merge strategy for config writes.
type MergeStrategy string

const (
	MergeStrategyReplace MergeStrategy = "replace"
	MergeStrategyUpsert  MergeStrategy = "upsert"
)

// Verbosity controls output length/detail on models via the Responses API.
type Verbosity string

const (
	VerbosityLow    Verbosity = "low"
	VerbosityMedium Verbosity = "medium"
	VerbosityHigh   Verbosity = "high"
)

// SandboxMode represents the sandbox access mode.
type SandboxMode string

const (
	SandboxModeReadOnly          SandboxMode = "read-only"
	SandboxModeWorkspaceWrite    SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess  SandboxMode = "danger-full-access"
)

// WebSearchMode represents the web search behavior mode.
type WebSearchMode string

const (
	WebSearchModeDisabled WebSearchMode = "disabled"
	WebSearchModeCached   WebSearchMode = "cached"
	WebSearchModeLive     WebSearchMode = "live"
)

// WriteStatus represents the result of a config write operation.
type WriteStatus string

const (
	WriteStatusOK           WriteStatus = "ok"
	WriteStatusOKOverridden WriteStatus = "okOverridden"
)

// NetworkApprovalProtocol represents the protocol in a network approval context.
type NetworkApprovalProtocol string

const (
	NetworkApprovalProtocolHTTP      NetworkApprovalProtocol = "http"
	NetworkApprovalProtocolHTTPS     NetworkApprovalProtocol = "https"
	NetworkApprovalProtocolSocks5TCP NetworkApprovalProtocol = "socks5Tcp"
	NetworkApprovalProtocolSocks5UDP NetworkApprovalProtocol = "socks5Udp"
)

// NetworkPolicyRuleAction represents the action for a network policy rule.
type NetworkPolicyRuleAction string

const (
	NetworkPolicyRuleActionAllow NetworkPolicyRuleAction = "allow"
	NetworkPolicyRuleActionDeny  NetworkPolicyRuleAction = "deny"
)

// ExecCommandSource represents where a command execution originated.
type ExecCommandSource string

const (
	ExecCommandSourceAgent                    ExecCommandSource = "agent"
	ExecCommandSourceUserShell                ExecCommandSource = "user_shell"
	ExecCommandSourceUnifiedExecStartup       ExecCommandSource = "unified_exec_startup"
	ExecCommandSourceUnifiedExecInteraction   ExecCommandSource = "unified_exec_interaction"
)

// ExecCommandStatus represents the status of a legacy exec command.
type ExecCommandStatus string

const (
	ExecCommandStatusCompleted ExecCommandStatus = "completed"
	ExecCommandStatusFailed    ExecCommandStatus = "failed"
	ExecCommandStatusDeclined  ExecCommandStatus = "declined"
)

// ExecOutputStream represents the output stream of a command.
type ExecOutputStream string

const (
	ExecOutputStreamStdout ExecOutputStream = "stdout"
	ExecOutputStreamStderr ExecOutputStream = "stderr"
)

// LocalShellStatus represents the status of a local shell execution.
type LocalShellStatus string

const (
	LocalShellStatusCompleted  LocalShellStatus = "completed"
	LocalShellStatusInProgress LocalShellStatus = "in_progress"
	LocalShellStatusIncomplete LocalShellStatus = "incomplete"
)

// ThreadActiveFlag represents the active status flag of a thread.
type ThreadActiveFlag string

const (
	ThreadActiveFlagWaitingOnApproval  ThreadActiveFlag = "waitingOnApproval"
	ThreadActiveFlagWaitingOnUserInput ThreadActiveFlag = "waitingOnUserInput"
)

// ThreadUnsubscribeStatus represents the result of unsubscribing from a thread.
type ThreadUnsubscribeStatus string

const (
	ThreadUnsubscribeStatusNotLoaded     ThreadUnsubscribeStatus = "notLoaded"
	ThreadUnsubscribeStatusNotSubscribed ThreadUnsubscribeStatus = "notSubscribed"
	ThreadUnsubscribeStatusUnsubscribed  ThreadUnsubscribeStatus = "unsubscribed"
)

// ThreadSortKey represents the sort key for thread listing.
type ThreadSortKey string

const (
	ThreadSortKeyCreatedAt ThreadSortKey = "created_at"
	ThreadSortKeyUpdatedAt ThreadSortKey = "updated_at"
)

// ThreadSourceKind represents the origin of a thread.
type ThreadSourceKind string

const (
	ThreadSourceKindCLI                 ThreadSourceKind = "cli"
	ThreadSourceKindVSCode              ThreadSourceKind = "vscode"
	ThreadSourceKindExec                ThreadSourceKind = "exec"
	ThreadSourceKindAppServer           ThreadSourceKind = "appServer"
	ThreadSourceKindSubAgent            ThreadSourceKind = "subAgent"
	ThreadSourceKindSubAgentReview      ThreadSourceKind = "subAgentReview"
	ThreadSourceKindSubAgentCompact     ThreadSourceKind = "subAgentCompact"
	ThreadSourceKindSubAgentThreadSpawn ThreadSourceKind = "subAgentThreadSpawn"
	ThreadSourceKindSubAgentOther       ThreadSourceKind = "subAgentOther"
	ThreadSourceKindUnknown             ThreadSourceKind = "unknown"
)

// ForcedLoginMethod represents the forced login method for account authentication.
type ForcedLoginMethod string

const (
	ForcedLoginMethodChatGPT ForcedLoginMethod = "chatgpt"
	ForcedLoginMethodAPI     ForcedLoginMethod = "api"
)

// ChatgptAuthTokensRefreshReason represents the reason for auth token refresh.
type ChatgptAuthTokensRefreshReason string

const (
	ChatgptAuthTokensRefreshReasonUnauthorized ChatgptAuthTokensRefreshReason = "unauthorized"
)

// SkillApprovalDecision represents the decision for a skill approval request.
type SkillApprovalDecision string

const (
	SkillApprovalDecisionApprove SkillApprovalDecision = "approve"
	SkillApprovalDecisionDecline SkillApprovalDecision = "decline"
)

// FileChangeApprovalDecision represents the decision for a file change approval request.
type FileChangeApprovalDecision string

const (
	FileChangeApprovalDecisionAccept           FileChangeApprovalDecision = "accept"
	FileChangeApprovalDecisionAcceptForSession FileChangeApprovalDecision = "acceptForSession"
	FileChangeApprovalDecisionDecline          FileChangeApprovalDecision = "decline"
)

// AppToolApproval represents the approval mode for an app tool.
type AppToolApproval string

const (
	AppToolApprovalAuto    AppToolApproval = "auto"
	AppToolApprovalPrompt  AppToolApproval = "prompt"
	AppToolApprovalApprove AppToolApproval = "approve"
)

// ResidencyRequirement represents a data residency requirement.
type ResidencyRequirement string

const (
	ResidencyRequirementUS ResidencyRequirement = "us"
)

// ReasoningEffort represents the reasoning effort level for a model.
type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

// InputModality represents a canonical user-input modality tag advertised by a model.
type InputModality string

const (
	InputModalityText  InputModality = "text"
	InputModalityImage InputModality = "image"
)

// ReasoningSummaryMode represents enum variant ("auto" | "concise" | "detailed" | "none")
type ReasoningSummaryMode string

func (ReasoningSummaryMode) isReasoningSummary() {}

const (
	ReasoningSummaryModeAuto     ReasoningSummaryMode = "auto"
	ReasoningSummaryModeConcise  ReasoningSummaryMode = "concise"
	ReasoningSummaryModeDetailed ReasoningSummaryMode = "detailed"
	ReasoningSummaryModeNone     ReasoningSummaryMode = "none"
)
