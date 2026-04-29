package codex

// Approval method names used in server→client request dispatch.
const (
	methodApplyPatchApproval              = "applyPatchApproval"
	methodCommandExecutionRequestApproval = "item/commandExecution/requestApproval"
	methodExecCommandApproval             = "execCommandApproval"
	methodFileChangeRequestApproval       = "item/fileChange/requestApproval"
	methodPermissionsRequestApproval      = "item/permissions/requestApproval"
	methodDynamicToolCall                 = "item/tool/call"
	methodToolRequestUserInput            = "item/tool/requestUserInput"
	methodChatgptAuthTokensRefresh        = "account/chatgptAuthTokens/refresh"
	methodMcpServerElicitationRequest     = "mcpServer/elicitation/request"
)

// Notification method names for server→client event dispatch.
const (
	// Streaming / item notifications
	notifyAgentMessageDelta         = "item/agentMessage/delta"
	notifyFileChangeOutputDelta     = "item/fileChange/outputDelta"
	notifyPlanDelta                 = "item/plan/delta"
	notifyReasoningTextDelta        = "item/reasoning/textDelta"
	notifyReasoningSummaryTextDelta = "item/reasoning/summaryTextDelta"
	notifyReasoningSummaryPartAdded = "item/reasoning/summaryPartAdded"
	notifyItemStarted               = "item/started"
	notifyItemCompleted             = "item/completed"

	// Thread notifications
	notifyThreadStarted           = "thread/started"
	notifyThreadClosed            = "thread/closed"
	notifyThreadArchived          = "thread/archived"
	notifyThreadUnarchived        = "thread/unarchived"
	notifyThreadGoalUpdated       = "thread/goal/updated"
	notifyThreadGoalCleared       = "thread/goal/cleared"
	notifyThreadNameUpdated       = "thread/name/updated"
	notifyThreadStatusChanged     = "thread/status/changed"
	notifyThreadTokenUsageUpdated = "thread/tokenUsage/updated"

	// Turn notifications
	notifyTurnStarted     = "turn/started"
	notifyTurnCompleted   = "turn/completed"
	notifyTurnPlanUpdated = "turn/plan/updated"
	notifyTurnDiffUpdated = "turn/diff/updated"

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
	notifyRealtimeSdp              = "thread/realtime/sdp"
	notifyRealtimeTranscriptDelta  = "thread/realtime/transcript/delta"
	notifyRealtimeTranscriptDone   = "thread/realtime/transcript/done"

	// System notifications
	notifyWindowsSandboxSetupCompleted = "windowsSandbox/setupCompleted"
	notifyWindowsWorldWritableWarning  = "windows/worldWritableWarning"
	notifyThreadCompacted              = "thread/compacted"
	notifyDeprecationNotice            = "deprecationNotice"
	notifyError                        = "error"
	notifyWarning                      = "warning"
	notifyGuardianWarning              = "guardianWarning"
	notifyRemoteControlStatusChanged   = "remoteControl/status/changed"
	notifyTerminalInteraction          = "item/commandExecution/terminalInteraction"

	// MCP notifications
	notifyMcpServerOauthLoginCompleted = "mcpServer/oauthLogin/completed"
	notifyMcpServerStatusUpdated       = "mcpServer/startupStatus/updated"
	notifyMcpToolCallProgress          = "item/mcpToolCall/progress"

	// Server request notifications
	notifyServerRequestResolved = "serverRequest/resolved"

	// Model notifications
	notifyModelRerouted     = "model/rerouted"
	notifyModelVerification = "model/verification"

	// Fuzzy search notifications
	notifyFuzzyFileSearchSessionCompleted = "fuzzyFileSearch/sessionCompleted"
	notifyFuzzyFileSearchSessionUpdated   = "fuzzyFileSearch/sessionUpdated"

	// Command notifications
	notifyCommandExecutionOutputDelta = "item/commandExecution/outputDelta"
	notifyCommandExecOutputDelta      = "command/exec/outputDelta"
	notifyFileChangePatchUpdated      = "item/fileChange/patchUpdated"

	// Filesystem notifications
	notifyFsChanged = "fs/changed"

	// External agent notifications
	notifyExternalAgentConfigImportCompleted = "externalAgentConfig/import/completed"

	// App notifications
	notifyAppListUpdated = "app/list/updated"

	// Config notifications
	notifyConfigWarning = "configWarning"

	// Skills notifications
	notifySkillsChanged = "skills/changed"

	// Hook notifications
	notifyHookStarted   = "hook/started"
	notifyHookCompleted = "hook/completed"

	// Guardian review notifications
	notifyItemGuardianApprovalReviewStarted   = "item/autoApprovalReview/started"
	notifyItemGuardianApprovalReviewCompleted = "item/autoApprovalReview/completed"
)

// Client→server request method names.
const (
	methodInitialize                        = "initialize"
	methodAccountRead                       = "account/read"
	methodAccountRateLimitsRead             = "account/rateLimits/read"
	methodAccountLoginStart                 = "account/login/start"
	methodAccountLoginCancel                = "account/login/cancel"
	methodAccountLogout                     = "account/logout"
	methodThreadStart                       = "thread/start"
	methodThreadRead                        = "thread/read"
	methodThreadList                        = "thread/list"
	methodThreadLoadedList                  = "thread/loaded/list"
	methodThreadResume                      = "thread/resume"
	methodThreadFork                        = "thread/fork"
	methodThreadRollback                    = "thread/rollback"
	methodThreadShellCommand                = "thread/shellCommand"
	methodThreadApproveGuardianDeniedAction = "thread/approveGuardianDeniedAction"
	methodThreadTurnsList                   = "thread/turns/list"
	methodThreadInjectItems                 = "thread/inject_items"
	methodThreadNameSet                     = "thread/name/set"
	methodThreadArchive                     = "thread/archive"
	methodThreadUnarchive                   = "thread/unarchive"
	methodThreadUnsubscribe                 = "thread/unsubscribe"
	methodThreadCompactStart                = "thread/compact/start"
	methodTurnStart                         = "turn/start"
	methodTurnInterrupt                     = "turn/interrupt"
	methodTurnSteer                         = "turn/steer"
	methodCommandExec                       = "command/exec"
	methodCommandExecWrite                  = "command/exec/write"
	methodCommandExecTerminate              = "command/exec/terminate"
	methodCommandExecResize                 = "command/exec/resize"
	methodModelList                         = "model/list"
	methodModelProviderCapabilitiesRead     = "modelProvider/capabilities/read"
	methodConfigRead                        = "config/read"
	methodConfigRequirementsRead            = "configRequirements/read"
	methodConfigValueWrite                  = "config/value/write"
	methodConfigBatchWrite                  = "config/batchWrite"
	methodMcpServerStatusList               = "mcpServerStatus/list"
	methodMcpServerOauthLogin               = "mcpServer/oauth/login"
	methodConfigMcpServerReload             = "config/mcpServer/reload"
	methodMcpResourceRead                   = "mcpServer/resource/read"
	methodMcpServerToolCall                 = "mcpServer/tool/call"
	methodFeedbackUpload                    = "feedback/upload"
	methodWindowsSandboxSetupStart          = "windowsSandbox/setupStart"
	methodExperimentalFeatureList           = "experimentalFeature/list"
	methodExperimentalFeatureEnablementSet  = "experimentalFeature/enablement/set"
	methodAppList                           = "app/list"
	methodReviewStart                       = "review/start"
	methodExternalAgentConfigDetect         = "externalAgentConfig/detect"
	methodExternalAgentConfigImport         = "externalAgentConfig/import"
	methodSkillsList                        = "skills/list"
	methodSkillsConfigWrite                 = "skills/config/write"
	methodPluginList                        = "plugin/list"
	methodPluginRead                        = "plugin/read"
	methodPluginInstall                     = "plugin/install"
	methodPluginUninstall                   = "plugin/uninstall"
	methodMarketplaceAdd                    = "marketplace/add"
	methodMarketplaceRemove                 = "marketplace/remove"
	methodMarketplaceUpgrade                = "marketplace/upgrade"
	methodDeviceKeyCreate                   = "device/key/create"
	methodDeviceKeyPublic                   = "device/key/public"
	methodDeviceKeySign                     = "device/key/sign"
	methodFsReadFile                        = "fs/readFile"
	methodFsWriteFile                       = "fs/writeFile"
	methodFsCreateDirectory                 = "fs/createDirectory"
	methodFsGetMetadata                     = "fs/getMetadata"
	methodFsReadDirectory                   = "fs/readDirectory"
	methodFsRemove                          = "fs/remove"
	methodFsCopy                            = "fs/copy"
	methodFsWatch                           = "fs/watch"
	methodFsUnwatch                         = "fs/unwatch"
	methodThreadMetadataUpdate              = "thread/metadata/update"
	methodAccountSendAddCreditsNudgeEmail   = "account/sendAddCreditsNudgeEmail"
	methodFuzzyFileSearch                   = "fuzzyFileSearch"
)
