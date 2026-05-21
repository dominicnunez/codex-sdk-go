package protocol

// AddAgentMessageDeltaListener appends a listener for item/agentMessage/delta notifications.
func (c *Client) AddAgentMessageDeltaListener(handler func(AgentMessageDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyAgentMessageDelta, handler)
}

// AddFileChangeOutputDeltaListener appends a listener for item/fileChange/outputDelta notifications.
func (c *Client) AddFileChangeOutputDeltaListener(handler func(FileChangeOutputDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyFileChangeOutputDelta, handler)
}

// AddFileChangePatchUpdatedListener appends a listener for item/fileChange/patchUpdated notifications.
func (c *Client) AddFileChangePatchUpdatedListener(handler func(FileChangePatchUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyFileChangePatchUpdated, handler)
}

// AddPlanDeltaListener appends a listener for item/plan/delta notifications.
func (c *Client) AddPlanDeltaListener(handler func(PlanDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyPlanDelta, handler)
}

// AddReasoningTextDeltaListener appends a listener for item/reasoning/textDelta notifications.
func (c *Client) AddReasoningTextDeltaListener(handler func(ReasoningTextDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyReasoningTextDelta, handler)
}

// AddReasoningSummaryTextDeltaListener appends a listener for item/reasoning/summaryTextDelta notifications.
func (c *Client) AddReasoningSummaryTextDeltaListener(handler func(ReasoningSummaryTextDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyReasoningSummaryTextDelta, handler)
}

// AddReasoningSummaryPartAddedListener appends a listener for item/reasoning/summaryPartAdded notifications.
func (c *Client) AddReasoningSummaryPartAddedListener(handler func(ReasoningSummaryPartAddedNotification)) func() {
	return addTypedNotificationListener(c, notifyReasoningSummaryPartAdded, handler)
}

// AddItemStartedListener appends a listener for item/started notifications.
func (c *Client) AddItemStartedListener(handler func(ItemStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyItemStarted, handler)
}

// AddItemCompletedListener appends a listener for item/completed notifications.
func (c *Client) AddItemCompletedListener(handler func(ItemCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyItemCompleted, handler)
}

// AddThreadStartedListener appends a listener for thread/started notifications.
func (c *Client) AddThreadStartedListener(handler func(ThreadStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadStarted, handler)
}

// AddThreadClosedListener appends a listener for thread/closed notifications.
func (c *Client) AddThreadClosedListener(handler func(ThreadClosedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadClosed, handler)
}

// AddThreadArchivedListener appends a listener for thread/archived notifications.
func (c *Client) AddThreadArchivedListener(handler func(ThreadArchivedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadArchived, handler)
}

// AddThreadUnarchivedListener appends a listener for thread/unarchived notifications.
func (c *Client) AddThreadUnarchivedListener(handler func(ThreadUnarchivedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadUnarchived, handler)
}

// AddThreadGoalUpdatedListener appends a listener for thread/goal/updated notifications.
func (c *Client) AddThreadGoalUpdatedListener(handler func(ThreadGoalUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadGoalUpdated, handler)
}

// AddThreadGoalClearedListener appends a listener for thread/goal/cleared notifications.
func (c *Client) AddThreadGoalClearedListener(handler func(ThreadGoalClearedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadGoalCleared, handler)
}

// AddThreadNameUpdatedListener appends a listener for thread/name/updated notifications.
func (c *Client) AddThreadNameUpdatedListener(handler func(ThreadNameUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadNameUpdated, handler)
}

// AddThreadSettingsUpdatedListener appends a listener for thread/settings/updated notifications.
func (c *Client) AddThreadSettingsUpdatedListener(handler func(ThreadSettingsUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadSettingsUpdated, handler)
}

// AddThreadStatusChangedListener appends a listener for thread/status/changed notifications.
func (c *Client) AddThreadStatusChangedListener(handler func(ThreadStatusChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadStatusChanged, handler)
}

// AddThreadTokenUsageUpdatedListener appends a listener for thread/tokenUsage/updated notifications.
func (c *Client) AddThreadTokenUsageUpdatedListener(handler func(ThreadTokenUsageUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadTokenUsageUpdated, handler)
}

// AddTurnStartedListener appends a listener for turn/started notifications.
func (c *Client) AddTurnStartedListener(handler func(TurnStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyTurnStarted, handler)
}

// AddTurnCompletedListener appends a listener for turn/completed notifications.
func (c *Client) AddTurnCompletedListener(handler func(TurnCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyTurnCompleted, handler)
}

// AddTurnPlanUpdatedListener appends a listener for turn/plan/updated notifications.
func (c *Client) AddTurnPlanUpdatedListener(handler func(TurnPlanUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyTurnPlanUpdated, handler)
}

// AddTurnDiffUpdatedListener appends a listener for turn/diff/updated notifications.
func (c *Client) AddTurnDiffUpdatedListener(handler func(TurnDiffUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyTurnDiffUpdated, handler)
}

// AddAccountUpdatedListener appends a listener for account/updated notifications.
func (c *Client) AddAccountUpdatedListener(handler func(AccountUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyAccountUpdated, handler)
}

// AddAccountLoginCompletedListener appends a listener for account/login/completed notifications.
func (c *Client) AddAccountLoginCompletedListener(handler func(AccountLoginCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyAccountLoginCompleted, handler)
}

// AddAccountRateLimitsUpdatedListener appends a listener for account/rateLimits/updated notifications.
func (c *Client) AddAccountRateLimitsUpdatedListener(handler func(AccountRateLimitsUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyAccountRateLimitsUpdated, handler)
}

// AddThreadRealtimeStartedListener appends a listener for thread/realtime/started notifications.
func (c *Client) AddThreadRealtimeStartedListener(handler func(ThreadRealtimeStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeStarted, handler)
}

// AddThreadRealtimeClosedListener appends a listener for thread/realtime/closed notifications.
func (c *Client) AddThreadRealtimeClosedListener(handler func(ThreadRealtimeClosedNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeClosed, handler)
}

// AddThreadRealtimeErrorListener appends a listener for thread/realtime/error notifications.
func (c *Client) AddThreadRealtimeErrorListener(handler func(ThreadRealtimeErrorNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeError, handler)
}

// AddThreadRealtimeItemAddedListener appends a listener for thread/realtime/itemAdded notifications.
func (c *Client) AddThreadRealtimeItemAddedListener(handler func(ThreadRealtimeItemAddedNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeItemAdded, handler)
}

// AddThreadRealtimeOutputAudioDeltaListener appends a listener for thread/realtime/outputAudio/delta notifications.
func (c *Client) AddThreadRealtimeOutputAudioDeltaListener(handler func(ThreadRealtimeOutputAudioDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeOutputAudioDelta, handler)
}

// AddThreadRealtimeSdpListener appends a listener for thread/realtime/sdp notifications.
func (c *Client) AddThreadRealtimeSdpListener(handler func(ThreadRealtimeSdpNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeSdp, handler)
}

// AddThreadRealtimeTranscriptDeltaListener appends a listener for thread/realtime/transcript/delta notifications.
func (c *Client) AddThreadRealtimeTranscriptDeltaListener(handler func(ThreadRealtimeTranscriptDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeTranscriptDelta, handler)
}

// AddThreadRealtimeTranscriptDoneListener appends a listener for thread/realtime/transcript/done notifications.
func (c *Client) AddThreadRealtimeTranscriptDoneListener(handler func(ThreadRealtimeTranscriptDoneNotification)) func() {
	return addTypedNotificationListener(c, notifyRealtimeTranscriptDone, handler)
}

// AddWindowsSandboxSetupCompletedListener appends a listener for windowsSandbox/setupCompleted notifications.
func (c *Client) AddWindowsSandboxSetupCompletedListener(handler func(WindowsSandboxSetupCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyWindowsSandboxSetupCompleted, handler)
}

// AddWindowsWorldWritableWarningListener appends a listener for windows/worldWritableWarning notifications.
func (c *Client) AddWindowsWorldWritableWarningListener(handler func(WindowsWorldWritableWarningNotification)) func() {
	return addTypedNotificationListener(c, notifyWindowsWorldWritableWarning, handler)
}

// AddContextCompactedListener appends a listener for thread/compacted notifications.
//
// Deprecated: Use ContextCompaction item type instead.
func (c *Client) AddContextCompactedListener(handler func(ContextCompactedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadCompacted, handler)
}

// AddDeprecationNoticeListener appends a listener for deprecationNotice notifications.
func (c *Client) AddDeprecationNoticeListener(handler func(DeprecationNoticeNotification)) func() {
	return addTypedNotificationListener(c, notifyDeprecationNotice, handler)
}

// AddErrorListener appends a listener for error notifications.
func (c *Client) AddErrorListener(handler func(ErrorNotification)) func() {
	return addTypedNotificationListener(c, notifyError, handler)
}

// AddWarningListener appends a listener for warning notifications.
func (c *Client) AddWarningListener(handler func(WarningNotification)) func() {
	return addTypedNotificationListener(c, notifyWarning, handler)
}

// AddGuardianWarningListener appends a listener for guardianWarning notifications.
func (c *Client) AddGuardianWarningListener(handler func(GuardianWarningNotification)) func() {
	return addTypedNotificationListener(c, notifyGuardianWarning, handler)
}

// AddRemoteControlStatusChangedListener appends a listener for remoteControl/status/changed notifications.
func (c *Client) AddRemoteControlStatusChangedListener(handler func(RemoteControlStatusChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyRemoteControlStatusChanged, handler)
}

// AddTerminalInteractionListener appends a listener for item/commandExecution/terminalInteraction notifications.
func (c *Client) AddTerminalInteractionListener(handler func(TerminalInteractionNotification)) func() {
	return addTypedNotificationListener(c, notifyTerminalInteraction, handler)
}

// AddMcpServerOauthLoginCompletedListener appends a listener for mcpServer/oauthLogin/completed notifications.
func (c *Client) AddMcpServerOauthLoginCompletedListener(handler func(McpServerOauthLoginCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyMcpServerOauthLoginCompleted, handler)
}

// AddMcpServerStatusUpdatedListener appends a listener for mcpServer/startupStatus/updated notifications.
func (c *Client) AddMcpServerStatusUpdatedListener(handler func(McpServerStatusUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyMcpServerStatusUpdated, handler)
}

// AddMcpToolCallProgressListener appends a listener for item/mcpToolCall/progress notifications.
func (c *Client) AddMcpToolCallProgressListener(handler func(McpToolCallProgressNotification)) func() {
	return addTypedNotificationListener(c, notifyMcpToolCallProgress, handler)
}

// AddServerRequestResolvedListener appends a listener for serverRequest/resolved notifications.
func (c *Client) AddServerRequestResolvedListener(handler func(ServerRequestResolvedNotification)) func() {
	return addTypedNotificationListener(c, notifyServerRequestResolved, handler)
}

// AddModelReroutedListener appends a listener for model/rerouted notifications.
func (c *Client) AddModelReroutedListener(handler func(ModelReroutedNotification)) func() {
	return addTypedNotificationListener(c, notifyModelRerouted, handler)
}

// AddModelVerificationListener appends a listener for model/verification notifications.
func (c *Client) AddModelVerificationListener(handler func(ModelVerificationNotification)) func() {
	return addTypedNotificationListener(c, notifyModelVerification, handler)
}

// AddFuzzyFileSearchSessionCompletedListener appends a listener for fuzzyFileSearch/sessionCompleted notifications.
func (c *Client) AddFuzzyFileSearchSessionCompletedListener(handler func(FuzzyFileSearchSessionCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyFuzzyFileSearchSessionCompleted, handler)
}

// AddFuzzyFileSearchSessionUpdatedListener appends a listener for fuzzyFileSearch/sessionUpdated notifications.
func (c *Client) AddFuzzyFileSearchSessionUpdatedListener(handler func(FuzzyFileSearchSessionUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyFuzzyFileSearchSessionUpdated, handler)
}

// AddCommandExecutionOutputDeltaListener appends a listener for item/commandExecution/outputDelta notifications.
func (c *Client) AddCommandExecutionOutputDeltaListener(handler func(CommandExecutionOutputDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyCommandExecutionOutputDelta, handler)
}

// AddCommandExecOutputDeltaListener appends a listener for command/exec/outputDelta notifications.
func (c *Client) AddCommandExecOutputDeltaListener(handler func(CommandExecOutputDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyCommandExecOutputDelta, handler)
}

// AddProcessOutputDeltaListener appends a listener for process/outputDelta notifications.
func (c *Client) AddProcessOutputDeltaListener(handler func(ProcessOutputDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyProcessOutputDelta, handler)
}

// AddProcessExitedListener appends a listener for process/exited notifications.
func (c *Client) AddProcessExitedListener(handler func(ProcessExitedNotification)) func() {
	return addTypedNotificationListener(c, notifyProcessExited, handler)
}

// AddFsChangedListener appends a listener for fs/changed notifications.
func (c *Client) AddFsChangedListener(handler func(FsChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyFsChanged, handler)
}

// AddExternalAgentConfigImportCompletedListener appends a listener for externalAgentConfig/import/completed notifications.
func (c *Client) AddExternalAgentConfigImportCompletedListener(handler func(ExternalAgentConfigImportCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyExternalAgentConfigImportCompleted, handler)
}

// AddAppListUpdatedListener appends a listener for app/list/updated notifications.
func (c *Client) AddAppListUpdatedListener(handler func(AppListUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyAppListUpdated, handler)
}

// AddConfigWarningListener appends a listener for configWarning notifications.
func (c *Client) AddConfigWarningListener(handler func(ConfigWarningNotification)) func() {
	return addTypedNotificationListener(c, notifyConfigWarning, handler)
}

// AddSkillsChangedListener appends a listener for skills/changed notifications.
func (c *Client) AddSkillsChangedListener(handler func(SkillsChangedNotification)) func() {
	return addTypedNotificationListener(c, notifySkillsChanged, handler)
}

// AddHookStartedListener appends a listener for hook/started notifications.
func (c *Client) AddHookStartedListener(handler func(HookStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyHookStarted, handler)
}

// AddHookCompletedListener appends a listener for hook/completed notifications.
func (c *Client) AddHookCompletedListener(handler func(HookCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyHookCompleted, handler)
}

// AddItemGuardianApprovalReviewStartedListener appends a listener for item/autoApprovalReview/started notifications.
func (c *Client) AddItemGuardianApprovalReviewStartedListener(handler func(ItemGuardianApprovalReviewStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyItemGuardianApprovalReviewStarted, handler)
}

// AddItemGuardianApprovalReviewCompletedListener appends a listener for item/autoApprovalReview/completed notifications.
func (c *Client) AddItemGuardianApprovalReviewCompletedListener(handler func(ItemGuardianApprovalReviewCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyItemGuardianApprovalReviewCompleted, handler)
}
