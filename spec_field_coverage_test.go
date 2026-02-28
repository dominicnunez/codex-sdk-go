package codex

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// fieldInfo describes a single Go struct field's JSON representation.
type fieldInfo struct {
	fieldName  string // Go field name
	isOptional bool   // true when the JSON tag contains "omitempty"
}

// structJSONFields extracts a map of JSON tag name → fieldInfo from a Go struct type.
// Fields tagged with `json:"-"` or without JSON tags are skipped.
func structJSONFields(t reflect.Type) map[string]fieldInfo {
	out := make(map[string]fieldInfo)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" || name == "-" {
			continue
		}
		omitempty := false
		for _, opt := range parts[1:] {
			if opt == "omitempty" {
				omitempty = true
			}
		}
		out[name] = fieldInfo{fieldName: f.Name, isOptional: omitempty}
	}
	return out
}

// schemaTopLevel is the minimal structure we parse from a spec JSON file.
type schemaTopLevel struct {
	Properties  map[string]json.RawMessage `json:"properties"`
	Required    []string                   `json:"required"`
	Definitions map[string]json.RawMessage `json:"definitions"`
	OneOf       []json.RawMessage          `json:"oneOf"`
}

// schemaFields reads a spec file and returns the set of top-level property names
// and the set of required property names.
func schemaFields(path string) (properties []string, required map[string]bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}
	for k := range s.Properties {
		properties = append(properties, k)
	}
	sort.Strings(properties)
	required = make(map[string]bool, len(s.Required))
	for _, r := range s.Required {
		required[r] = true
	}
	return properties, required, nil
}

// enumDef represents a parsed enum definition from a spec.
type enumDef struct {
	Enum  []string          `json:"enum"`
	OneOf []json.RawMessage `json:"oneOf"`
}

// schemaEnumValues reads a spec file and extracts all string enum values for
// the named definition. It handles both direct `enum` arrays and `oneOf` arrays
// where each variant contains a single-value `enum`.
func schemaEnumValues(path string, defName string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	raw, ok := s.Definitions[defName]
	if !ok {
		return nil, nil
	}
	var def enumDef
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, err
	}

	// Direct enum array (e.g., Personality, SandboxMode)
	if len(def.Enum) > 0 {
		return def.Enum, nil
	}

	// oneOf array where each variant is a single-value string enum
	var vals []string
	for _, variant := range def.OneOf {
		var v enumDef
		if err := json.Unmarshal(variant, &v); err != nil {
			continue
		}
		vals = append(vals, v.Enum...)
	}
	return vals, nil
}

// TestSpecFieldCoverage verifies that Go struct fields and enum constants match
// the properties and enum values defined in the JSON schemas under specs/.
//
// This test uses manually maintained registries that link schema names to Go types.
// If a new schema appears, TestSpecCoverage will fail first (name-level check), and
// the developer then adds the type to the field registry here.
func TestSpecFieldCoverage(t *testing.T) {
	t.Run("StructFields", testStructFields)
	t.Run("EnumValues", testEnumValues)
}

// structEntry maps a spec file path to its Go struct type.
type structEntry struct {
	specPath string
	goType   reflect.Type
}

func testStructFields(t *testing.T) {
	// Registry of schema file → Go struct.
	// Excludes:
	//   - 13 "implemented differently" schemas (JSONRPC*, method dispatch, etc.)
	//   - Top-level oneOf unions (LoginAccountParams, LoginAccountResponse)
	//   - Schemas with no properties (empty response objects)
	//   - Schemas whose Go type uses interface{}/json.RawMessage for complex unions
	registry := []structEntry{
		// Root-level specs (approval types)
		{"specs/ApplyPatchApprovalParams.json", reflect.TypeOf(ApplyPatchApprovalParams{})},
		{"specs/ApplyPatchApprovalResponse.json", reflect.TypeOf(ApplyPatchApprovalResponse{})},
		{"specs/ChatgptAuthTokensRefreshParams.json", reflect.TypeOf(ChatgptAuthTokensRefreshParams{})},
		{"specs/ChatgptAuthTokensRefreshResponse.json", reflect.TypeOf(ChatgptAuthTokensRefreshResponse{})},
		{"specs/CommandExecutionRequestApprovalParams.json", reflect.TypeOf(CommandExecutionRequestApprovalParams{})},
		{"specs/CommandExecutionRequestApprovalResponse.json", reflect.TypeOf(CommandExecutionRequestApprovalResponse{})},
		{"specs/DynamicToolCallParams.json", reflect.TypeOf(DynamicToolCallParams{})},
		{"specs/DynamicToolCallResponse.json", reflect.TypeOf(DynamicToolCallResponse{})},
		{"specs/ExecCommandApprovalParams.json", reflect.TypeOf(ExecCommandApprovalParams{})},
		{"specs/ExecCommandApprovalResponse.json", reflect.TypeOf(ExecCommandApprovalResponse{})},
		{"specs/FileChangeRequestApprovalParams.json", reflect.TypeOf(FileChangeRequestApprovalParams{})},
		{"specs/FileChangeRequestApprovalResponse.json", reflect.TypeOf(FileChangeRequestApprovalResponse{})},
		{"specs/FuzzyFileSearchParams.json", reflect.TypeOf(FuzzyFileSearchParams{})},
		{"specs/FuzzyFileSearchResponse.json", reflect.TypeOf(FuzzyFileSearchResponse{})},
		{"specs/FuzzyFileSearchSessionCompletedNotification.json", reflect.TypeOf(FuzzyFileSearchSessionCompletedNotification{})},
		{"specs/FuzzyFileSearchSessionUpdatedNotification.json", reflect.TypeOf(FuzzyFileSearchSessionUpdatedNotification{})},
		{"specs/ToolRequestUserInputParams.json", reflect.TypeOf(ToolRequestUserInputParams{})},
		{"specs/ToolRequestUserInputResponse.json", reflect.TypeOf(ToolRequestUserInputResponse{})},

		// v1 specs
		{"specs/v1/InitializeParams.json", reflect.TypeOf(InitializeParams{})},
		{"specs/v1/InitializeResponse.json", reflect.TypeOf(InitializeResponse{})},

		// v2 account
		{"specs/v2/GetAccountParams.json", reflect.TypeOf(GetAccountParams{})},
		{"specs/v2/GetAccountResponse.json", reflect.TypeOf(GetAccountResponse{})},
		{"specs/v2/GetAccountRateLimitsResponse.json", reflect.TypeOf(GetAccountRateLimitsResponse{})},
		{"specs/v2/CancelLoginAccountParams.json", reflect.TypeOf(CancelLoginAccountParams{})},
		{"specs/v2/CancelLoginAccountResponse.json", reflect.TypeOf(CancelLoginAccountResponse{})},

		// v2 account notifications
		{"specs/v2/AccountUpdatedNotification.json", reflect.TypeOf(AccountUpdatedNotification{})},
		{"specs/v2/AccountLoginCompletedNotification.json", reflect.TypeOf(AccountLoginCompletedNotification{})},
		{"specs/v2/AccountRateLimitsUpdatedNotification.json", reflect.TypeOf(AccountRateLimitsUpdatedNotification{})},

		// v2 apps
		{"specs/v2/AppsListParams.json", reflect.TypeOf(AppsListParams{})},
		{"specs/v2/AppsListResponse.json", reflect.TypeOf(AppsListResponse{})},
		{"specs/v2/AppListUpdatedNotification.json", reflect.TypeOf(AppListUpdatedNotification{})},

		// v2 command
		{"specs/v2/CommandExecParams.json", reflect.TypeOf(CommandExecParams{})},
		{"specs/v2/CommandExecResponse.json", reflect.TypeOf(CommandExecResponse{})},
		{"specs/v2/CommandExecutionOutputDeltaNotification.json", reflect.TypeOf(CommandExecutionOutputDeltaNotification{})},

		// v2 config
		{"specs/v2/ConfigReadParams.json", reflect.TypeOf(ConfigReadParams{})},
		{"specs/v2/ConfigReadResponse.json", reflect.TypeOf(ConfigReadResponse{})},
		{"specs/v2/ConfigRequirementsReadResponse.json", reflect.TypeOf(ConfigRequirementsReadResponse{})},
		{"specs/v2/ConfigValueWriteParams.json", reflect.TypeOf(ConfigValueWriteParams{})},
		{"specs/v2/ConfigBatchWriteParams.json", reflect.TypeOf(ConfigBatchWriteParams{})},
		{"specs/v2/ConfigWriteResponse.json", reflect.TypeOf(ConfigWriteResponse{})},
		{"specs/v2/ConfigWarningNotification.json", reflect.TypeOf(ConfigWarningNotification{})},

		// v2 experimental
		{"specs/v2/ExperimentalFeatureListParams.json", reflect.TypeOf(ExperimentalFeatureListParams{})},
		{"specs/v2/ExperimentalFeatureListResponse.json", reflect.TypeOf(ExperimentalFeatureListResponse{})},

		// v2 external agent
		{"specs/v2/ExternalAgentConfigDetectParams.json", reflect.TypeOf(ExternalAgentConfigDetectParams{})},
		{"specs/v2/ExternalAgentConfigDetectResponse.json", reflect.TypeOf(ExternalAgentConfigDetectResponse{})},
		{"specs/v2/ExternalAgentConfigImportParams.json", reflect.TypeOf(ExternalAgentConfigImportParams{})},

		// v2 feedback
		{"specs/v2/FeedbackUploadParams.json", reflect.TypeOf(FeedbackUploadParams{})},
		{"specs/v2/FeedbackUploadResponse.json", reflect.TypeOf(FeedbackUploadResponse{})},

		// v2 streaming notifications
		{"specs/v2/AgentMessageDeltaNotification.json", reflect.TypeOf(AgentMessageDeltaNotification{})},
		{"specs/v2/FileChangeOutputDeltaNotification.json", reflect.TypeOf(FileChangeOutputDeltaNotification{})},
		{"specs/v2/PlanDeltaNotification.json", reflect.TypeOf(PlanDeltaNotification{})},
		{"specs/v2/ReasoningTextDeltaNotification.json", reflect.TypeOf(ReasoningTextDeltaNotification{})},
		{"specs/v2/ReasoningSummaryTextDeltaNotification.json", reflect.TypeOf(ReasoningSummaryTextDeltaNotification{})},
		{"specs/v2/ReasoningSummaryPartAddedNotification.json", reflect.TypeOf(ReasoningSummaryPartAddedNotification{})},
		{"specs/v2/ItemStartedNotification.json", reflect.TypeOf(ItemStartedNotification{})},
		{"specs/v2/ItemCompletedNotification.json", reflect.TypeOf(ItemCompletedNotification{})},

		// v2 MCP
		{"specs/v2/ListMcpServerStatusParams.json", reflect.TypeOf(ListMcpServerStatusParams{})},
		{"specs/v2/ListMcpServerStatusResponse.json", reflect.TypeOf(ListMcpServerStatusResponse{})},
		{"specs/v2/McpServerOauthLoginParams.json", reflect.TypeOf(McpServerOauthLoginParams{})},
		{"specs/v2/McpServerOauthLoginResponse.json", reflect.TypeOf(McpServerOauthLoginResponse{})},
		{"specs/v2/McpServerOauthLoginCompletedNotification.json", reflect.TypeOf(McpServerOauthLoginCompletedNotification{})},
		{"specs/v2/McpToolCallProgressNotification.json", reflect.TypeOf(McpToolCallProgressNotification{})},

		// v2 model
		{"specs/v2/ModelListParams.json", reflect.TypeOf(ModelListParams{})},
		{"specs/v2/ModelListResponse.json", reflect.TypeOf(ModelListResponse{})},
		{"specs/v2/ModelReroutedNotification.json", reflect.TypeOf(ModelReroutedNotification{})},

		// v2 review
		{"specs/v2/ReviewStartParams.json", reflect.TypeOf(ReviewStartParams{})},
		{"specs/v2/ReviewStartResponse.json", reflect.TypeOf(ReviewStartResponse{})},

		// v2 skills
		{"specs/v2/SkillsListParams.json", reflect.TypeOf(SkillsListParams{})},
		{"specs/v2/SkillsListResponse.json", reflect.TypeOf(SkillsListResponse{})},
		{"specs/v2/SkillsConfigWriteParams.json", reflect.TypeOf(SkillsConfigWriteParams{})},
		{"specs/v2/SkillsConfigWriteResponse.json", reflect.TypeOf(SkillsConfigWriteResponse{})},
		{"specs/v2/SkillsRemoteReadParams.json", reflect.TypeOf(SkillsRemoteReadParams{})},
		{"specs/v2/SkillsRemoteReadResponse.json", reflect.TypeOf(SkillsRemoteReadResponse{})},
		{"specs/v2/SkillsRemoteWriteParams.json", reflect.TypeOf(SkillsRemoteWriteParams{})},
		{"specs/v2/SkillsRemoteWriteResponse.json", reflect.TypeOf(SkillsRemoteWriteResponse{})},

		// v2 system
		{"specs/v2/WindowsSandboxSetupStartParams.json", reflect.TypeOf(WindowsSandboxSetupStartParams{})},
		{"specs/v2/WindowsSandboxSetupStartResponse.json", reflect.TypeOf(WindowsSandboxSetupStartResponse{})},
		{"specs/v2/WindowsSandboxSetupCompletedNotification.json", reflect.TypeOf(WindowsSandboxSetupCompletedNotification{})},
		{"specs/v2/WindowsWorldWritableWarningNotification.json", reflect.TypeOf(WindowsWorldWritableWarningNotification{})},
		{"specs/v2/ContextCompactedNotification.json", reflect.TypeOf(ContextCompactedNotification{})},
		{"specs/v2/DeprecationNoticeNotification.json", reflect.TypeOf(DeprecationNoticeNotification{})},
		{"specs/v2/ErrorNotification.json", reflect.TypeOf(ErrorNotification{})},
		{"specs/v2/TerminalInteractionNotification.json", reflect.TypeOf(TerminalInteractionNotification{})},

		// v2 thread
		{"specs/v2/ThreadStartParams.json", reflect.TypeOf(ThreadStartParams{})},
		{"specs/v2/ThreadStartResponse.json", reflect.TypeOf(ThreadStartResponse{})},
		{"specs/v2/ThreadReadParams.json", reflect.TypeOf(ThreadReadParams{})},
		{"specs/v2/ThreadReadResponse.json", reflect.TypeOf(ThreadReadResponse{})},
		{"specs/v2/ThreadListParams.json", reflect.TypeOf(ThreadListParams{})},
		{"specs/v2/ThreadListResponse.json", reflect.TypeOf(ThreadListResponse{})},
		{"specs/v2/ThreadLoadedListParams.json", reflect.TypeOf(ThreadLoadedListParams{})},
		{"specs/v2/ThreadLoadedListResponse.json", reflect.TypeOf(ThreadLoadedListResponse{})},
		{"specs/v2/ThreadResumeParams.json", reflect.TypeOf(ThreadResumeParams{})},
		{"specs/v2/ThreadResumeResponse.json", reflect.TypeOf(ThreadResumeResponse{})},
		{"specs/v2/ThreadForkParams.json", reflect.TypeOf(ThreadForkParams{})},
		{"specs/v2/ThreadForkResponse.json", reflect.TypeOf(ThreadForkResponse{})},
		{"specs/v2/ThreadRollbackParams.json", reflect.TypeOf(ThreadRollbackParams{})},
		{"specs/v2/ThreadRollbackResponse.json", reflect.TypeOf(ThreadRollbackResponse{})},
		{"specs/v2/ThreadSetNameParams.json", reflect.TypeOf(ThreadSetNameParams{})},
		{"specs/v2/ThreadSetNameResponse.json", reflect.TypeOf(ThreadSetNameResponse{})},
		{"specs/v2/ThreadArchiveParams.json", reflect.TypeOf(ThreadArchiveParams{})},
		{"specs/v2/ThreadUnarchiveParams.json", reflect.TypeOf(ThreadUnarchiveParams{})},
		{"specs/v2/ThreadUnsubscribeParams.json", reflect.TypeOf(ThreadUnsubscribeParams{})},
		{"specs/v2/ThreadUnsubscribeResponse.json", reflect.TypeOf(ThreadUnsubscribeResponse{})},
		{"specs/v2/ThreadCompactStartParams.json", reflect.TypeOf(ThreadCompactStartParams{})},

		// v2 thread notifications
		{"specs/v2/ThreadStartedNotification.json", reflect.TypeOf(ThreadStartedNotification{})},
		{"specs/v2/ThreadClosedNotification.json", reflect.TypeOf(ThreadClosedNotification{})},
		{"specs/v2/ThreadArchivedNotification.json", reflect.TypeOf(ThreadArchivedNotification{})},
		{"specs/v2/ThreadUnarchivedNotification.json", reflect.TypeOf(ThreadUnarchivedNotification{})},
		{"specs/v2/ThreadNameUpdatedNotification.json", reflect.TypeOf(ThreadNameUpdatedNotification{})},
		{"specs/v2/ThreadStatusChangedNotification.json", reflect.TypeOf(ThreadStatusChangedNotification{})},
		{"specs/v2/ThreadTokenUsageUpdatedNotification.json", reflect.TypeOf(ThreadTokenUsageUpdatedNotification{})},
		{"specs/v2/ServerRequestResolvedNotification.json", reflect.TypeOf(ServerRequestResolvedNotification{})},

		// v2 realtime
		{"specs/v2/ThreadRealtimeStartedNotification.json", reflect.TypeOf(ThreadRealtimeStartedNotification{})},
		{"specs/v2/ThreadRealtimeClosedNotification.json", reflect.TypeOf(ThreadRealtimeClosedNotification{})},
		{"specs/v2/ThreadRealtimeErrorNotification.json", reflect.TypeOf(ThreadRealtimeErrorNotification{})},
		{"specs/v2/ThreadRealtimeItemAddedNotification.json", reflect.TypeOf(ThreadRealtimeItemAddedNotification{})},
		{"specs/v2/ThreadRealtimeOutputAudioDeltaNotification.json", reflect.TypeOf(ThreadRealtimeOutputAudioDeltaNotification{})},

		// v2 turn
		{"specs/v2/TurnStartParams.json", reflect.TypeOf(TurnStartParams{})},
		{"specs/v2/TurnStartResponse.json", reflect.TypeOf(TurnStartResponse{})},
		{"specs/v2/TurnInterruptParams.json", reflect.TypeOf(TurnInterruptParams{})},
		{"specs/v2/TurnSteerParams.json", reflect.TypeOf(TurnSteerParams{})},
		{"specs/v2/TurnSteerResponse.json", reflect.TypeOf(TurnSteerResponse{})},

		// v2 turn notifications
		{"specs/v2/TurnStartedNotification.json", reflect.TypeOf(TurnStartedNotification{})},
		{"specs/v2/TurnCompletedNotification.json", reflect.TypeOf(TurnCompletedNotification{})},
		{"specs/v2/TurnPlanUpdatedNotification.json", reflect.TypeOf(TurnPlanUpdatedNotification{})},
		{"specs/v2/TurnDiffUpdatedNotification.json", reflect.TypeOf(TurnDiffUpdatedNotification{})},
	}

	for _, entry := range registry {
		entry := entry
		t.Run(entry.specPath, func(t *testing.T) {
			properties, required, err := schemaFields(entry.specPath)
			if err != nil {
				t.Fatalf("failed to parse schema: %v", err)
			}
			if len(properties) == 0 {
				t.Skip("schema has no properties")
			}

			goFields := structJSONFields(entry.goType)

			// Check every schema property has a matching JSON tag.
			for _, prop := range properties {
				fi, ok := goFields[prop]
				if !ok {
					t.Errorf("schema property %q has no matching JSON field on %s", prop, entry.goType.Name())
					continue
				}

				// Check required/optional alignment.
				if required[prop] && fi.isOptional {
					t.Errorf("schema requires %q but Go field %s.%s has omitempty", prop, entry.goType.Name(), fi.fieldName)
				}
			}
		})
	}
}

// enumEntry maps a schema definition name to the spec file that defines it
// and the Go constant values that should cover it.
type enumEntry struct {
	specPath string   // spec file containing the definition
	defName  string   // key under "definitions" in the spec
	goValues []string // string values of Go constants
}

func testEnumValues(t *testing.T) {
	// Registry of enum definitions → Go constant values.
	// Each entry picks one canonical spec file that defines the enum
	// (many are duplicated across specs; we only need to check once).
	registry := []enumEntry{
		// From ServerNotification.json definitions
		{
			specPath: "specs/v2/ModelReroutedNotification.json",
			defName:  "ModelRerouteReason",
			goValues: enumStrings(
				ModelRerouteReasonHighRiskCyberActivity,
			),
		},
		{
			specPath: "specs/v2/AccountRateLimitsUpdatedNotification.json",
			defName:  "PlanType",
			goValues: enumStrings(
				PlanTypeFree, PlanTypeGo, PlanTypePlus, PlanTypePro,
				PlanTypeTeam, PlanTypeBusiness, PlanTypeEnterprise,
				PlanTypeEdu, PlanTypeUnknown,
			),
		},
		{
			specPath: "specs/v2/AccountUpdatedNotification.json",
			defName:  "AuthMode",
			goValues: enumStrings(
				AuthModeAPIKey, AuthModeChatGPT, AuthModeChatGPTAuthTokens,
			),
		},
		{
			specPath: "specs/v2/CancelLoginAccountResponse.json",
			defName:  "CancelLoginAccountStatus",
			goValues: enumStrings(
				CancelLoginAccountStatusCanceled, CancelLoginAccountStatusNotFound,
			),
		},

		// Enums from enums.go
		{
			specPath: "specs/v2/ReviewStartResponse.json",
			defName:  "TurnStatus",
			goValues: enumStrings(
				TurnStatusCompleted, TurnStatusInterrupted,
				TurnStatusFailed, TurnStatusInProgress,
			),
		},
		{
			specPath: "specs/v2/ThreadStartParams.json",
			defName:  "Personality",
			goValues: enumStrings(
				PersonalityNone, PersonalityFriendly, PersonalityPragmatic,
			),
		},
		{
			specPath: "specs/v2/TurnStartParams.json",
			defName:  "ModeKind",
			goValues: enumStrings(
				ModeKindPlan, ModeKindDefault,
			),
		},
		{
			specPath: "specs/v2/ConfigBatchWriteParams.json",
			defName:  "MergeStrategy",
			goValues: enumStrings(
				MergeStrategyReplace, MergeStrategyUpsert,
			),
		},
		{
			specPath: "specs/v2/ConfigReadResponse.json",
			defName:  "Verbosity",
			goValues: enumStrings(
				VerbosityLow, VerbosityMedium, VerbosityHigh,
			),
		},
		{
			specPath: "specs/v2/ThreadStartParams.json",
			defName:  "SandboxMode",
			goValues: enumStrings(
				SandboxModeReadOnly, SandboxModeWorkspaceWrite,
				SandboxModeDangerFullAccess,
			),
		},
		{
			specPath: "specs/v2/ConfigReadResponse.json",
			defName:  "WebSearchMode",
			goValues: enumStrings(
				WebSearchModeDisabled, WebSearchModeCached, WebSearchModeLive,
			),
		},
		{
			specPath: "specs/v2/ConfigWriteResponse.json",
			defName:  "WriteStatus",
			goValues: enumStrings(
				WriteStatusOK, WriteStatusOKOverridden,
			),
		},
		{
			specPath: "specs/CommandExecutionRequestApprovalParams.json",
			defName:  "NetworkApprovalProtocol",
			goValues: enumStrings(
				NetworkApprovalProtocolHTTP, NetworkApprovalProtocolHTTPS,
				NetworkApprovalProtocolSocks5TCP, NetworkApprovalProtocolSocks5UDP,
			),
		},
		{
			specPath: "specs/CommandExecutionRequestApprovalParams.json",
			defName:  "NetworkPolicyRuleAction",
			goValues: enumStrings(
				NetworkPolicyRuleActionAllow, NetworkPolicyRuleActionDeny,
			),
		},
		{
			specPath: "specs/v2/TurnStartParams.json",
			defName:  "ReasoningEffort",
			goValues: enumStrings(
				ReasoningEffortNone, ReasoningEffortMinimal, ReasoningEffortLow,
				ReasoningEffortMedium, ReasoningEffortHigh, ReasoningEffortXHigh,
			),
		},
		{
			specPath: "specs/v2/ModelListResponse.json",
			defName:  "InputModality",
			goValues: enumStrings(
				InputModalityText, InputModalityImage,
			),
		},
		{
			specPath: "specs/FileChangeRequestApprovalResponse.json",
			defName:  "FileChangeApprovalDecision",
			goValues: enumStrings(
				FileChangeApprovalDecisionAccept, FileChangeApprovalDecisionAcceptForSession,
				FileChangeApprovalDecisionDecline, FileChangeApprovalDecisionCancel,
			),
		},
		{
			specPath: "specs/CommandExecutionRequestApprovalResponse.json",
			defName:  "CommandExecutionApprovalDecision",
			goValues: []string{
				CommandExecutionApprovalDecisionAccept,
				CommandExecutionApprovalDecisionAcceptForSession,
				CommandExecutionApprovalDecisionDecline,
				CommandExecutionApprovalDecisionCancel,
			},
		},
		{
			specPath: "specs/ChatgptAuthTokensRefreshParams.json",
			defName:  "ChatgptAuthTokensRefreshReason",
			goValues: enumStrings(
				ChatgptAuthTokensRefreshReasonUnauthorized,
			),
		},
		{
			specPath: "specs/v2/ConfigReadResponse.json",
			defName:  "AppToolApproval",
			goValues: enumStrings(
				AppToolApprovalAuto, AppToolApprovalPrompt, AppToolApprovalApprove,
			),
		},
		{
			specPath: "specs/v2/ConfigRequirementsReadResponse.json",
			defName:  "ResidencyRequirement",
			goValues: enumStrings(
				ResidencyRequirementUS,
			),
		},
		{
			specPath: "specs/v2/ConfigReadResponse.json",
			defName:  "ForcedLoginMethod",
			goValues: enumStrings(
				ForcedLoginMethodChatGPT, ForcedLoginMethodAPI,
			),
		},
		{
			specPath: "specs/v2/ConfigReadResponse.json",
			defName:  "ReasoningSummary",
			goValues: enumStrings(
				ReasoningSummaryModeAuto, ReasoningSummaryModeConcise,
				ReasoningSummaryModeDetailed, ReasoningSummaryModeNone,
			),
		},

		// Enums from event_types.go
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "MessagePhase",
			goValues: enumStrings(
				MessagePhaseCommentary, MessagePhaseFinalAnswer,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "CommandExecutionStatus",
			goValues: enumStrings(
				CommandExecutionStatusInProgress, CommandExecutionStatusCompleted,
				CommandExecutionStatusFailed, CommandExecutionStatusDeclined,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "PatchApplyStatus",
			goValues: enumStrings(
				PatchApplyStatusInProgress, PatchApplyStatusCompleted,
				PatchApplyStatusFailed, PatchApplyStatusDeclined,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "McpToolCallStatus",
			goValues: enumStrings(
				McpToolCallStatusInProgress, McpToolCallStatusCompleted,
				McpToolCallStatusFailed,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "DynamicToolCallStatus",
			goValues: enumStrings(
				DynamicToolCallStatusInProgress, DynamicToolCallStatusCompleted,
				DynamicToolCallStatusFailed,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentStatus",
			goValues: enumStrings(
				CollabAgentStatusPendingInit, CollabAgentStatusRunning,
				CollabAgentStatusCompleted, CollabAgentStatusErrored,
				CollabAgentStatusShutdown, CollabAgentStatusNotFound,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentTool",
			goValues: enumStrings(
				CollabAgentToolSpawnAgent, CollabAgentToolSendInput,
				CollabAgentToolResumeAgent, CollabAgentToolWait,
				CollabAgentToolCloseAgent,
			),
		},
		{
			specPath: "specs/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentToolCallStatus",
			goValues: enumStrings(
				CollabAgentToolCallStatusInProgress, CollabAgentToolCallStatusCompleted,
				CollabAgentToolCallStatusFailed,
			),
		},

		// Remaining enums from various files
		{
			specPath: "specs/v2/ListMcpServerStatusResponse.json",
			defName:  "McpAuthStatus",
			goValues: enumStrings(
				McpAuthStatusUnsupported, McpAuthStatusNotLoggedIn,
				McpAuthStatusBearerToken, McpAuthStatusOAuth,
			),
		},
		{
			specPath: "specs/v2/ReviewStartParams.json",
			defName:  "ReviewDelivery",
			goValues: enumStrings(
				ReviewDeliveryInline, ReviewDeliveryDetached,
			),
		},
		{
			specPath: "specs/v2/SkillsListResponse.json",
			defName:  "SkillScope",
			goValues: enumStrings(
				SkillScopeUser, SkillScopeRepo,
				SkillScopeSystem, SkillScopeAdmin,
			),
		},
		{
			specPath: "specs/v2/SkillsRemoteReadParams.json",
			defName:  "HazelnutScope",
			goValues: enumStrings(
				HazelnutScopeExample, HazelnutScopeWorkspaceShared,
				HazelnutScopeAllShared, HazelnutScopePersonal,
			),
		},
		{
			specPath: "specs/v2/SkillsRemoteReadParams.json",
			defName:  "ProductSurface",
			goValues: enumStrings(
				ProductSurfaceChatGPT, ProductSurfaceCodex,
				ProductSurfaceAPI, ProductSurfaceAtlas,
			),
		},
		{
			specPath: "specs/v2/ExperimentalFeatureListResponse.json",
			defName:  "ExperimentalFeatureStage",
			goValues: enumStrings(
				ExperimentalFeatureStageBeta, ExperimentalFeatureStageUnderDevelopment,
				ExperimentalFeatureStageStable, ExperimentalFeatureStageDeprecated,
				ExperimentalFeatureStageRemoved,
			),
		},
		{
			specPath: "specs/v2/ExternalAgentConfigDetectResponse.json",
			defName:  "ExternalAgentConfigMigrationItemType",
			goValues: enumStrings(
				MigrationItemTypeAgentsMd, MigrationItemTypeConfig,
				MigrationItemTypeSkills, MigrationItemTypeMcpServerConfig,
			),
		},
		{
			specPath: "specs/v2/WindowsSandboxSetupCompletedNotification.json",
			defName:  "WindowsSandboxSetupMode",
			goValues: enumStrings(
				WindowsSandboxSetupModeElevated, WindowsSandboxSetupModeUnelevated,
			),
		},
		{
			specPath: "specs/v2/ThreadStatusChangedNotification.json",
			defName:  "ThreadActiveFlag",
			goValues: enumStrings(
				ThreadActiveFlagWaitingOnApproval, ThreadActiveFlagWaitingOnUserInput,
			),
		},
		{
			specPath: "specs/v2/CommandExecParams.json",
			defName:  "NetworkAccess",
			goValues: enumStrings(
				NetworkAccessRestricted, NetworkAccessEnabled,
			),
		},

		// Enums in v2 specs that are also in ClientRequest/EventMsg/codex_app_server
		{
			specPath: "specs/v2/ThreadListParams.json",
			defName:  "ThreadSortKey",
			goValues: enumStrings(
				ThreadSortKeyCreatedAt, ThreadSortKeyUpdatedAt,
			),
		},
		{
			specPath: "specs/v2/ThreadListParams.json",
			defName:  "ThreadSourceKind",
			goValues: enumStrings(
				ThreadSourceKindCLI, ThreadSourceKindVSCode,
				ThreadSourceKindExec, ThreadSourceKindAppServer,
				ThreadSourceKindSubAgent, ThreadSourceKindSubAgentReview,
				ThreadSourceKindSubAgentCompact, ThreadSourceKindSubAgentThreadSpawn,
				ThreadSourceKindSubAgentOther, ThreadSourceKindUnknown,
			),
		},
		{
			specPath: "specs/v2/ThreadResumeParams.json",
			defName:  "LocalShellStatus",
			goValues: enumStrings(
				LocalShellStatusCompleted, LocalShellStatusInProgress,
				LocalShellStatusIncomplete,
			),
		},
		{
			specPath: "specs/v2/ThreadUnsubscribeResponse.json",
			defName:  "ThreadUnsubscribeStatus",
			goValues: enumStrings(
				ThreadUnsubscribeStatusNotLoaded, ThreadUnsubscribeStatusNotSubscribed,
				ThreadUnsubscribeStatusUnsubscribed,
			),
		},
		{
			specPath: "specs/v2/TurnPlanUpdatedNotification.json",
			defName:  "TurnPlanStepStatus",
			goValues: enumStrings(
				TurnPlanStepStatusPending, TurnPlanStepStatusInProgress,
				TurnPlanStepStatusCompleted,
			),
		},
		{
			specPath: "specs/codex_app_server_protocol.schemas.json",
			defName:  "ExecCommandSource",
			goValues: enumStrings(
				ExecCommandSourceAgent, ExecCommandSourceUserShell,
				ExecCommandSourceUnifiedExecStartup, ExecCommandSourceUnifiedExecInteraction,
			),
		},
		{
			specPath: "specs/codex_app_server_protocol.schemas.json",
			defName:  "ExecCommandStatus",
			goValues: enumStrings(
				ExecCommandStatusCompleted, ExecCommandStatusFailed,
				ExecCommandStatusDeclined,
			),
		},
		{
			specPath: "specs/codex_app_server_protocol.schemas.json",
			defName:  "ExecOutputStream",
			goValues: enumStrings(
				ExecOutputStreamStdout, ExecOutputStreamStderr,
			),
		},
		{
			specPath: "specs/codex_app_server_protocol.schemas.json",
			defName:  "TurnAbortReason",
			goValues: enumStrings(
				TurnAbortReasonInterrupted, TurnAbortReasonReplaced,
				TurnAbortReasonReviewEnded,
			),
		},
	}

	for _, entry := range registry {
		entry := entry
		name := entry.specPath + "/" + entry.defName
		t.Run(name, func(t *testing.T) {
			specVals, err := schemaEnumValues(entry.specPath, entry.defName)
			if err != nil {
				t.Fatalf("failed to parse schema enum: %v", err)
			}
			if len(specVals) == 0 {
				t.Fatalf("no enum values found for %s in %s", entry.defName, entry.specPath)
			}

			goSet := make(map[string]bool, len(entry.goValues))
			for _, v := range entry.goValues {
				goSet[v] = true
			}

			for _, sv := range specVals {
				if !goSet[sv] {
					t.Errorf("spec enum value %q (%s) missing from Go constants", sv, entry.defName)
				}
			}
		})
	}
}

// enumStrings is a helper that converts typed string enum constants to []string.
func enumStrings[T ~string](vals ...T) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = string(v)
	}
	return out
}
