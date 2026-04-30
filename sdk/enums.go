package codex

import "fmt"

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

var validTurnStatuses = map[TurnStatus]struct{}{
	TurnStatusCompleted:   {},
	TurnStatusInterrupted: {},
	TurnStatusFailed:      {},
	TurnStatusInProgress:  {},
}

func (s *TurnStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "turn.status", validTurnStatuses, s)
}

// TurnAbortReason represents the reason a turn was aborted.
type TurnAbortReason string

const (
	TurnAbortReasonInterrupted TurnAbortReason = "interrupted"
	TurnAbortReasonReplaced    TurnAbortReason = "replaced"
	TurnAbortReasonReviewEnded TurnAbortReason = "review_ended"
)

// Personality represents the assistant personality style.
type Personality string

const (
	PersonalityNone      Personality = "none"
	PersonalityFriendly  Personality = "friendly"
	PersonalityPragmatic Personality = "pragmatic"
)

var validPersonalities = map[Personality]struct{}{
	PersonalityNone:      {},
	PersonalityFriendly:  {},
	PersonalityPragmatic: {},
}

func (p Personality) MarshalJSON() ([]byte, error) {
	return marshalEnumString("personality", p, validPersonalities)
}

func (p *Personality) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "personality", validPersonalities, p)
}

// ApprovalsReviewer controls who reviews approval requests.
type ApprovalsReviewer string

const (
	ApprovalsReviewerUser             ApprovalsReviewer = "user"
	ApprovalsReviewerAutoReview       ApprovalsReviewer = "auto_review"
	ApprovalsReviewerGuardianSubagent ApprovalsReviewer = "guardian_subagent"
)

var validApprovalsReviewers = map[ApprovalsReviewer]struct{}{
	ApprovalsReviewerUser:             {},
	ApprovalsReviewerAutoReview:       {},
	ApprovalsReviewerGuardianSubagent: {},
}

func (r ApprovalsReviewer) MarshalJSON() ([]byte, error) {
	return marshalEnumString("approvalsReviewer", r, validApprovalsReviewers)
}

func (r *ApprovalsReviewer) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "approvalsReviewer", validApprovalsReviewers, r)
}

// ServiceTier selects the runtime service tier.
type ServiceTier string

const (
	ServiceTierFast ServiceTier = "fast"
	ServiceTierFlex ServiceTier = "flex"
)

var validServiceTiers = map[ServiceTier]struct{}{
	ServiceTierFast: {},
	ServiceTierFlex: {},
}

func (s ServiceTier) MarshalJSON() ([]byte, error) {
	return marshalEnumString("serviceTier", s, validServiceTiers)
}

func (s *ServiceTier) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "serviceTier", validServiceTiers, s)
}

// ModeKind represents the initial collaboration mode.
type ModeKind string

const (
	ModeKindPlan    ModeKind = "plan"
	ModeKindDefault ModeKind = "default"
)

var validModeKinds = map[ModeKind]struct{}{
	ModeKindPlan:    {},
	ModeKindDefault: {},
}

func (m ModeKind) MarshalJSON() ([]byte, error) {
	return marshalEnumString("mode", m, validModeKinds)
}

func (m *ModeKind) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "mode", validModeKinds, m)
}

// MergeStrategy represents the merge strategy for config writes.
type MergeStrategy string

const (
	MergeStrategyReplace MergeStrategy = "replace"
	MergeStrategyUpsert  MergeStrategy = "upsert"
)

var validMergeStrategies = map[MergeStrategy]struct{}{
	MergeStrategyReplace: {},
	MergeStrategyUpsert:  {},
}

func (m MergeStrategy) MarshalJSON() ([]byte, error) {
	return marshalEnumString("mergeStrategy", m, validMergeStrategies)
}

func (m *MergeStrategy) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "mergeStrategy", validMergeStrategies, m)
}

// Verbosity controls output length/detail on models via the Responses API.
type Verbosity string

const (
	VerbosityLow    Verbosity = "low"
	VerbosityMedium Verbosity = "medium"
	VerbosityHigh   Verbosity = "high"
)

var validVerbosityLevels = map[Verbosity]struct{}{
	VerbosityLow:    {},
	VerbosityMedium: {},
	VerbosityHigh:   {},
}

func (v Verbosity) MarshalJSON() ([]byte, error) {
	return marshalEnumString("verbosity", v, validVerbosityLevels)
}

func (v *Verbosity) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "verbosity", validVerbosityLevels, v)
}

// SandboxMode represents the sandbox access mode.
type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

var validSandboxModes = map[SandboxMode]struct{}{
	SandboxModeReadOnly:         {},
	SandboxModeWorkspaceWrite:   {},
	SandboxModeDangerFullAccess: {},
}

func (m SandboxMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("sandboxMode", m, validSandboxModes)
}

func (m *SandboxMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "sandboxMode", validSandboxModes, m)
}

// WebSearchMode represents the web search behavior mode.
type WebSearchMode string

const (
	WebSearchModeDisabled WebSearchMode = "disabled"
	WebSearchModeCached   WebSearchMode = "cached"
	WebSearchModeLive     WebSearchMode = "live"
)

var validWebSearchModes = map[WebSearchMode]struct{}{
	WebSearchModeDisabled: {},
	WebSearchModeCached:   {},
	WebSearchModeLive:     {},
}

func (m WebSearchMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("webSearchMode", m, validWebSearchModes)
}

func (m *WebSearchMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "webSearchMode", validWebSearchModes, m)
}

// WriteStatus represents the result of a config write operation.
type WriteStatus string

const (
	WriteStatusOK           WriteStatus = "ok"
	WriteStatusOKOverridden WriteStatus = "okOverridden"
)

var validWriteStatuses = map[WriteStatus]struct{}{
	WriteStatusOK:           {},
	WriteStatusOKOverridden: {},
}

func validateWriteStatusField(field string, value WriteStatus) error {
	return validateEnumValue(field, value, validWriteStatuses)
}

// NetworkApprovalProtocol represents the protocol in a network approval context.
type NetworkApprovalProtocol string

const (
	NetworkApprovalProtocolHTTP      NetworkApprovalProtocol = "http"
	NetworkApprovalProtocolHTTPS     NetworkApprovalProtocol = "https"
	NetworkApprovalProtocolSocks5TCP NetworkApprovalProtocol = "socks5Tcp"
	NetworkApprovalProtocolSocks5UDP NetworkApprovalProtocol = "socks5Udp"
)

var validNetworkApprovalProtocols = map[NetworkApprovalProtocol]struct{}{
	NetworkApprovalProtocolHTTP:      {},
	NetworkApprovalProtocolHTTPS:     {},
	NetworkApprovalProtocolSocks5TCP: {},
	NetworkApprovalProtocolSocks5UDP: {},
}

func validateNetworkApprovalProtocolField(field string, value NetworkApprovalProtocol) error {
	return validateEnumValue(field, value, validNetworkApprovalProtocols)
}

// NetworkPolicyRuleAction represents the action for a network policy rule.
type NetworkPolicyRuleAction string

const (
	NetworkPolicyRuleActionAllow NetworkPolicyRuleAction = "allow"
	NetworkPolicyRuleActionDeny  NetworkPolicyRuleAction = "deny"
)

func validateNetworkPolicyRuleAction(action NetworkPolicyRuleAction) error {
	switch action {
	case NetworkPolicyRuleActionAllow, NetworkPolicyRuleActionDeny:
		return nil
	default:
		return fmt.Errorf("invalid network policy action %q", action)
	}
}

// ExecCommandSource represents where a command execution originated.
type ExecCommandSource string

const (
	ExecCommandSourceAgent                  ExecCommandSource = "agent"
	ExecCommandSourceUserShell              ExecCommandSource = "user_shell"
	ExecCommandSourceUnifiedExecStartup     ExecCommandSource = "unified_exec_startup"
	ExecCommandSourceUnifiedExecInteraction ExecCommandSource = "unified_exec_interaction"
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

var validThreadActiveFlags = map[ThreadActiveFlag]struct{}{
	ThreadActiveFlagWaitingOnApproval:  {},
	ThreadActiveFlagWaitingOnUserInput: {},
}

func (f *ThreadActiveFlag) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "thread.status.activeFlags", validThreadActiveFlags, f)
}

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

var validThreadSortKeys = map[ThreadSortKey]struct{}{
	ThreadSortKeyCreatedAt: {},
	ThreadSortKeyUpdatedAt: {},
}

func (k ThreadSortKey) MarshalJSON() ([]byte, error) {
	return marshalEnumString("sortKey", k, validThreadSortKeys)
}

func (k *ThreadSortKey) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "sortKey", validThreadSortKeys, k)
}

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

var validThreadSourceKinds = map[ThreadSourceKind]struct{}{
	ThreadSourceKindCLI:                 {},
	ThreadSourceKindVSCode:              {},
	ThreadSourceKindExec:                {},
	ThreadSourceKindAppServer:           {},
	ThreadSourceKindSubAgent:            {},
	ThreadSourceKindSubAgentReview:      {},
	ThreadSourceKindSubAgentCompact:     {},
	ThreadSourceKindSubAgentThreadSpawn: {},
	ThreadSourceKindSubAgentOther:       {},
	ThreadSourceKindUnknown:             {},
}

func (k ThreadSourceKind) MarshalJSON() ([]byte, error) {
	return marshalEnumString("sourceKinds", k, validThreadSourceKinds)
}

func (k *ThreadSourceKind) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "sourceKinds", validThreadSourceKinds, k)
}

// ForcedLoginMethod represents the forced login method for account authentication.
type ForcedLoginMethod string

const (
	ForcedLoginMethodChatGPT ForcedLoginMethod = "chatgpt"
	ForcedLoginMethodAPI     ForcedLoginMethod = "api"
)

var validForcedLoginMethods = map[ForcedLoginMethod]struct{}{
	ForcedLoginMethodChatGPT: {},
	ForcedLoginMethodAPI:     {},
}

func (m ForcedLoginMethod) MarshalJSON() ([]byte, error) {
	return marshalEnumString("forcedLoginMethod", m, validForcedLoginMethods)
}

func (m *ForcedLoginMethod) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "forcedLoginMethod", validForcedLoginMethods, m)
}

// ChatgptAuthTokensRefreshReason represents the reason for auth token refresh.
type ChatgptAuthTokensRefreshReason string

const (
	ChatgptAuthTokensRefreshReasonUnauthorized ChatgptAuthTokensRefreshReason = "unauthorized"
)

var validChatgptAuthTokensRefreshReasons = map[ChatgptAuthTokensRefreshReason]struct{}{
	ChatgptAuthTokensRefreshReasonUnauthorized: {},
}

func validateChatgptAuthTokensRefreshReasonField(field string, value ChatgptAuthTokensRefreshReason) error {
	return validateEnumValue(field, value, validChatgptAuthTokensRefreshReasons)
}

// FileChangeApprovalDecision represents the decision for a file change approval request.
type FileChangeApprovalDecision string

const (
	FileChangeApprovalDecisionAccept           FileChangeApprovalDecision = "accept"
	FileChangeApprovalDecisionAcceptForSession FileChangeApprovalDecision = "acceptForSession"
	FileChangeApprovalDecisionDecline          FileChangeApprovalDecision = "decline"
	FileChangeApprovalDecisionCancel           FileChangeApprovalDecision = "cancel"
)

var validFileChangeApprovalDecisions = map[FileChangeApprovalDecision]struct{}{
	FileChangeApprovalDecisionAccept:           {},
	FileChangeApprovalDecisionAcceptForSession: {},
	FileChangeApprovalDecisionDecline:          {},
	FileChangeApprovalDecisionCancel:           {},
}

func validateFileChangeApprovalDecisionField(field string, value FileChangeApprovalDecision) error {
	return validateEnumValue(field, value, validFileChangeApprovalDecisions)
}

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

var validResidencyRequirements = map[ResidencyRequirement]struct{}{
	ResidencyRequirementUS: {},
}

func (r ResidencyRequirement) MarshalJSON() ([]byte, error) {
	return marshalEnumString("residencyRequirement", r, validResidencyRequirements)
}

func (r *ResidencyRequirement) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "residencyRequirement", validResidencyRequirements, r)
}

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

var validReasoningEfforts = map[ReasoningEffort]struct{}{
	ReasoningEffortNone:    {},
	ReasoningEffortMinimal: {},
	ReasoningEffortLow:     {},
	ReasoningEffortMedium:  {},
	ReasoningEffortHigh:    {},
	ReasoningEffortXHigh:   {},
}

func (r ReasoningEffort) MarshalJSON() ([]byte, error) {
	return marshalEnumString("reasoningEffort", r, validReasoningEfforts)
}

func (r *ReasoningEffort) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "reasoningEffort", validReasoningEfforts, r)
}

// InputModality represents a canonical user-input modality tag advertised by a model.
type InputModality string

const (
	InputModalityText  InputModality = "text"
	InputModalityImage InputModality = "image"
)

var validInputModalities = map[InputModality]struct{}{
	InputModalityText:  {},
	InputModalityImage: {},
}

func (m *InputModality) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "inputModality", validInputModalities, m)
}

// ReasoningSummaryMode represents enum variant ("auto" | "concise" | "detailed" | "none")
type ReasoningSummaryMode string

func (ReasoningSummaryMode) isReasoningSummary() {}

const (
	ReasoningSummaryModeAuto     ReasoningSummaryMode = "auto"
	ReasoningSummaryModeConcise  ReasoningSummaryMode = "concise"
	ReasoningSummaryModeDetailed ReasoningSummaryMode = "detailed"
	ReasoningSummaryModeNone     ReasoningSummaryMode = "none"
)

var validReasoningSummaryModes = map[ReasoningSummaryMode]struct{}{
	ReasoningSummaryModeAuto:     {},
	ReasoningSummaryModeConcise:  {},
	ReasoningSummaryModeDetailed: {},
	ReasoningSummaryModeNone:     {},
}

func (m ReasoningSummaryMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("reasoningSummary", m, validReasoningSummaryModes)
}

func (m *ReasoningSummaryMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "reasoningSummary", validReasoningSummaryModes, m)
}
