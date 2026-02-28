package codex

// Approval method names used in server→client request dispatch.
const (
	methodApplyPatchApproval                = "applyPatchApproval"
	methodCommandExecutionRequestApproval   = "item/commandExecution/requestApproval"
	methodExecCommandApproval               = "execCommandApproval"
	methodFileChangeRequestApproval         = "item/fileChange/requestApproval"
	methodSkillRequestApproval              = "skill/requestApproval"
	methodDynamicToolCall                   = "item/tool/call"
	methodToolRequestUserInput              = "item/tool/requestUserInput"
	methodChatgptAuthTokensRefresh          = "account/chatgptAuthTokens/refresh"
)

// Notification method names for server→client event dispatch.
const (
	// Streaming / item notifications
	notifyAgentMessageDelta        = "item/agentMessage/delta"
	notifyFileChangeOutputDelta    = "item/fileChange/outputDelta"
	notifyPlanDelta                = "item/plan/delta"
	notifyReasoningTextDelta       = "item/reasoning/textDelta"
	notifyReasoningSummaryTextDelta = "item/reasoning/summaryTextDelta"
	notifyReasoningSummaryPartAdded = "item/reasoning/summaryPartAdded"
	notifyItemStarted              = "item/started"
	notifyItemCompleted            = "item/completed"

	// Thread notifications
	notifyThreadStarted            = "thread/started"
	notifyThreadClosed             = "thread/closed"
	notifyThreadArchived           = "thread/archived"
	notifyThreadUnarchived         = "thread/unarchived"
	notifyThreadNameUpdated        = "thread/name/updated"
	notifyThreadStatusChanged      = "thread/status/changed"
	notifyThreadTokenUsageUpdated  = "thread/tokenUsage/updated"

	// Turn notifications
	notifyTurnStarted              = "turn/started"
	notifyTurnCompleted            = "turn/completed"
	notifyTurnPlanUpdated          = "turn/plan/updated"
	notifyTurnDiffUpdated          = "turn/diff/updated"

	// Account notifications
	notifyAccountUpdated           = "account/updated"
	notifyAccountLoginCompleted    = "account/login/completed"
	notifyAccountRateLimitsUpdated = "account/rateLimits/updated"

	// Realtime notifications
	notifyRealtimeStarted          = "thread/realtime/started"
	notifyRealtimeClosed           = "thread/realtime/closed"
	notifyRealtimeError            = "thread/realtime/error"
	notifyRealtimeItemAdded        = "thread/realtime/itemAdded"
	notifyRealtimeOutputAudioDelta = "thread/realtime/outputAudio/delta"

	// System notifications
	notifyWindowsSandboxSetupCompleted = "windowsSandbox/setupCompleted"
	notifyWindowsWorldWritableWarning  = "windows/worldWritableWarning"
	notifyThreadCompacted              = "thread/compacted"
	notifyDeprecationNotice            = "deprecationNotice"
	notifyError                        = "error"
	notifyTerminalInteraction          = "item/commandExecution/terminalInteraction"

	// MCP notifications
	notifyMcpServerOauthLoginCompleted = "mcpServer/oauthLogin/completed"
	notifyMcpToolCallProgress          = "item/mcpToolCall/progress"

	// Model notifications
	notifyModelRerouted = "model/rerouted"

	// Fuzzy search notifications
	notifyFuzzyFileSearchSessionCompleted = "fuzzyFileSearch/sessionCompleted"
	notifyFuzzyFileSearchSessionUpdated   = "fuzzyFileSearch/sessionUpdated"

	// Command notifications
	notifyCommandExecutionOutputDelta = "item/commandExecution/outputDelta"

	// App notifications
	notifyAppListUpdated = "app/list/updated"

	// Config notifications
	notifyConfigWarning = "configWarning"
)

// Client→server request method names.
const (
	methodInitialize              = "initialize"
	methodAccountRead             = "account/read"
	methodAccountRateLimitsRead   = "account/rateLimits/read"
	methodAccountLoginStart       = "account/login/start"
	methodAccountLoginCancel      = "account/login/cancel"
	methodAccountLogout           = "account/logout"
	methodThreadStart             = "thread/start"
	methodThreadRead              = "thread/read"
	methodThreadList              = "thread/list"
	methodThreadLoadedList        = "thread/loaded/list"
	methodThreadResume            = "thread/resume"
	methodThreadFork              = "thread/fork"
	methodThreadRollback          = "thread/rollback"
	methodThreadNameSet           = "thread/name/set"
	methodThreadArchive           = "thread/archive"
	methodThreadUnarchive         = "thread/unarchive"
	methodThreadUnsubscribe       = "thread/unsubscribe"
	methodThreadCompactStart      = "thread/compact/start"
	methodTurnStart               = "turn/start"
	methodTurnInterrupt           = "turn/interrupt"
	methodTurnSteer               = "turn/steer"
	methodCommandExec             = "command/exec"
	methodModelList               = "model/list"
	methodConfigRead              = "config/read"
	methodConfigRequirementsRead  = "configRequirements/read"
	methodConfigValueWrite        = "config/value/write"
	methodConfigBatchWrite        = "config/batchWrite"
	methodMcpServerStatusList     = "mcpServerStatus/list"
	methodMcpServerOauthLogin     = "mcpServer/oauth/login"
	methodConfigMcpServerReload   = "config/mcpServer/reload"
	methodFeedbackUpload          = "feedback/upload"
	methodWindowsSandboxSetupStart = "windowsSandbox/setupStart"
	methodExperimentalFeatureList = "experimentalFeature/list"
	methodAppList                 = "app/list"
	methodReviewStart             = "review/start"
	methodExternalAgentConfigDetect = "externalAgentConfig/detect"
	methodExternalAgentConfigImport = "externalAgentConfig/import"
	methodSkillsList              = "skills/list"
	methodSkillsConfigWrite       = "skills/config/write"
	methodSkillsRemoteList        = "skills/remote/list"
	methodSkillsRemoteExport      = "skills/remote/export"
	methodFuzzyFileSearch         = "fuzzyFileSearch"
)
