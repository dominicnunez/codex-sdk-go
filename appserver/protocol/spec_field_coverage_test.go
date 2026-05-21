package protocol

import (
	"encoding/json"
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
	data, err := readSpecFile(path)
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

// schemaDefinitionVariantFields reads a spec file, looks up a definition by name,
// finds the oneOf variant matching the given title, and returns its property names
// and required set. Used for ThreadItem variants defined inside definitions.ThreadItem.oneOf.
func schemaDefinitionVariantFields(path, defName, variantTitle string) (properties []string, required map[string]bool, err error) {
	data, err := readSpecFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}
	raw, ok := s.Definitions[defName]
	if !ok {
		return nil, nil, nil
	}
	var def struct {
		OneOf []json.RawMessage `json:"oneOf"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, nil, err
	}
	for _, variant := range def.OneOf {
		var v schemaTopLevel
		if err := json.Unmarshal(variant, &v); err != nil {
			continue
		}
		var titleCheck struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(variant, &titleCheck); err != nil {
			continue
		}
		if titleCheck.Title != variantTitle {
			continue
		}
		for k := range v.Properties {
			properties = append(properties, k)
		}
		sort.Strings(properties)
		required = make(map[string]bool, len(v.Required))
		for _, r := range v.Required {
			required[r] = true
		}
		return properties, required, nil
	}
	return nil, nil, nil
}

func schemaTopLevelVariantFields(path, variantTitle string) (properties []string, required map[string]bool, err error) {
	data, err := readSpecFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}
	for _, variant := range s.OneOf {
		var v schemaTopLevel
		if err := json.Unmarshal(variant, &v); err != nil {
			continue
		}
		var titleCheck struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(variant, &titleCheck); err != nil {
			continue
		}
		if titleCheck.Title != variantTitle {
			continue
		}
		for k := range v.Properties {
			properties = append(properties, k)
		}
		sort.Strings(properties)
		required = make(map[string]bool, len(v.Required))
		for _, r := range v.Required {
			required[r] = true
		}
		return properties, required, nil
	}
	return nil, nil, nil
}

func schemaTopLevelVariantFieldsByRequired(path string, requiredFields []string) (properties []string, required map[string]bool, err error) {
	data, err := readSpecFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}

	expected := make(map[string]struct{}, len(requiredFields))
	for _, field := range requiredFields {
		expected[field] = struct{}{}
	}

	for _, variant := range s.OneOf {
		var v schemaTopLevel
		if err := json.Unmarshal(variant, &v); err != nil {
			continue
		}
		if len(v.Required) != len(expected) {
			continue
		}

		match := true
		for _, field := range v.Required {
			if _, ok := expected[field]; !ok {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		for k := range v.Properties {
			properties = append(properties, k)
		}
		sort.Strings(properties)
		required = make(map[string]bool, len(v.Required))
		for _, r := range v.Required {
			required[r] = true
		}
		return properties, required, nil
	}

	return nil, nil, nil
}

func schemaDefinitionFields(path, defName string) (properties []string, required map[string]bool, err error) {
	data, err := readSpecFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s schemaTopLevel
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}
	raw, ok := s.Definitions[defName]
	if !ok {
		return nil, nil, nil
	}
	var def schemaTopLevel
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, nil, err
	}
	for k := range def.Properties {
		properties = append(properties, k)
	}
	sort.Strings(properties)
	required = make(map[string]bool, len(def.Required))
	for _, r := range def.Required {
		required[r] = true
	}
	return properties, required, nil
}

// definitionVariantEntry maps a spec file + definition + variant title to a Go struct type.
type definitionVariantEntry struct {
	specPath     string
	defName      string
	variantTitle string
	goType       reflect.Type
}

type topLevelVariantEntry struct {
	specPath     string
	variantTitle string
	goType       reflect.Type
}

type topLevelRequiredVariantEntry struct {
	specPath       string
	requiredFields []string
	goType         reflect.Type
}

type definitionStructEntry struct {
	specPath string
	defName  string
	goType   reflect.Type
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
	data, err := readSpecFile(path)
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
// the properties and enum values defined in the JSON schemas under schema/json/.
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
	//   - Top-level oneOf unions handled by dedicated variant checks below
	//   - Schemas with no properties (empty response objects)
	//   - Schemas whose Go type uses interface{}/json.RawMessage for complex unions
	registry := []structEntry{
		// Root-level specs (approval types)
		{"schema/json/ApplyPatchApprovalParams.json", reflect.TypeOf(ApplyPatchApprovalParams{})},
		{"schema/json/ApplyPatchApprovalResponse.json", reflect.TypeOf(ApplyPatchApprovalResponse{})},
		{"schema/json/AttestationGenerateResponse.json", reflect.TypeOf(AttestationGenerateResponse{})},
		{"schema/json/ChatgptAuthTokensRefreshParams.json", reflect.TypeOf(ChatgptAuthTokensRefreshParams{})},
		{"schema/json/ChatgptAuthTokensRefreshResponse.json", reflect.TypeOf(ChatgptAuthTokensRefreshResponse{})},
		{"schema/json/CommandExecutionRequestApprovalParams.json", reflect.TypeOf(CommandExecutionRequestApprovalParams{})},
		{"schema/json/CommandExecutionRequestApprovalResponse.json", reflect.TypeOf(CommandExecutionRequestApprovalResponse{})},
		{"schema/json/DynamicToolCallParams.json", reflect.TypeOf(DynamicToolCallParams{})},
		{"schema/json/DynamicToolCallResponse.json", reflect.TypeOf(DynamicToolCallResponse{})},
		{"schema/json/ExecCommandApprovalParams.json", reflect.TypeOf(ExecCommandApprovalParams{})},
		{"schema/json/ExecCommandApprovalResponse.json", reflect.TypeOf(ExecCommandApprovalResponse{})},
		{"schema/json/FileChangeRequestApprovalParams.json", reflect.TypeOf(FileChangeRequestApprovalParams{})},
		{"schema/json/FileChangeRequestApprovalResponse.json", reflect.TypeOf(FileChangeRequestApprovalResponse{})},
		{"schema/json/FuzzyFileSearchParams.json", reflect.TypeOf(FuzzyFileSearchParams{})},
		{"schema/json/FuzzyFileSearchResponse.json", reflect.TypeOf(FuzzyFileSearchResponse{})},
		{"schema/json/McpServerElicitationRequestParams.json", reflect.TypeOf(McpServerElicitationRequestParams{})},
		{"schema/json/McpServerElicitationRequestResponse.json", reflect.TypeOf(McpServerElicitationRequestResponse{})},
		{"schema/json/PermissionsRequestApprovalParams.json", reflect.TypeOf(PermissionsRequestApprovalParams{})},
		{"schema/json/PermissionsRequestApprovalResponse.json", reflect.TypeOf(PermissionsRequestApprovalResponse{})},
		{"schema/json/FuzzyFileSearchSessionCompletedNotification.json", reflect.TypeOf(FuzzyFileSearchSessionCompletedNotification{})},
		{"schema/json/FuzzyFileSearchSessionUpdatedNotification.json", reflect.TypeOf(FuzzyFileSearchSessionUpdatedNotification{})},
		{"schema/json/ToolRequestUserInputParams.json", reflect.TypeOf(ToolRequestUserInputParams{})},
		{"schema/json/ToolRequestUserInputResponse.json", reflect.TypeOf(ToolRequestUserInputResponse{})},

		// v1 specs
		{"schema/json/v1/InitializeParams.json", reflect.TypeOf(InitializeParams{})},
		{"schema/json/v1/InitializeResponse.json", reflect.TypeOf(InitializeResponse{})},

		// v2 account
		{"schema/json/v2/GetAccountParams.json", reflect.TypeOf(GetAccountParams{})},
		{"schema/json/v2/GetAccountResponse.json", reflect.TypeOf(GetAccountResponse{})},
		{"schema/json/v2/GetAccountRateLimitsResponse.json", reflect.TypeOf(GetAccountRateLimitsResponse{})},
		{"schema/json/v2/SendAddCreditsNudgeEmailParams.json", reflect.TypeOf(SendAddCreditsNudgeEmailParams{})},
		{"schema/json/v2/SendAddCreditsNudgeEmailResponse.json", reflect.TypeOf(SendAddCreditsNudgeEmailResponse{})},
		{"schema/json/v2/CancelLoginAccountParams.json", reflect.TypeOf(CancelLoginAccountParams{})},
		{"schema/json/v2/CancelLoginAccountResponse.json", reflect.TypeOf(CancelLoginAccountResponse{})},

		// v2 account notifications
		{"schema/json/v2/AccountUpdatedNotification.json", reflect.TypeOf(AccountUpdatedNotification{})},
		{"schema/json/v2/AccountLoginCompletedNotification.json", reflect.TypeOf(AccountLoginCompletedNotification{})},
		{"schema/json/v2/AccountRateLimitsUpdatedNotification.json", reflect.TypeOf(AccountRateLimitsUpdatedNotification{})},

		// v2 apps
		{"schema/json/v2/AppsListParams.json", reflect.TypeOf(AppsListParams{})},
		{"schema/json/v2/AppsListResponse.json", reflect.TypeOf(AppsListResponse{})},
		{"schema/json/v2/AppListUpdatedNotification.json", reflect.TypeOf(AppListUpdatedNotification{})},

		// v2 command
		{"schema/json/v2/CommandExecParams.json", reflect.TypeOf(CommandExecParams{})},
		{"schema/json/v2/CommandExecResponse.json", reflect.TypeOf(CommandExecResponse{})},
		{"schema/json/v2/CommandExecWriteParams.json", reflect.TypeOf(CommandExecWriteParams{})},
		{"schema/json/v2/CommandExecWriteResponse.json", reflect.TypeOf(CommandExecWriteResponse{})},
		{"schema/json/v2/CommandExecTerminateParams.json", reflect.TypeOf(CommandExecTerminateParams{})},
		{"schema/json/v2/CommandExecTerminateResponse.json", reflect.TypeOf(CommandExecTerminateResponse{})},
		{"schema/json/v2/CommandExecResizeParams.json", reflect.TypeOf(CommandExecResizeParams{})},
		{"schema/json/v2/CommandExecResizeResponse.json", reflect.TypeOf(CommandExecResizeResponse{})},
		{"schema/json/v2/CommandExecOutputDeltaNotification.json", reflect.TypeOf(CommandExecOutputDeltaNotification{})},
		{"schema/json/v2/CommandExecutionOutputDeltaNotification.json", reflect.TypeOf(CommandExecutionOutputDeltaNotification{})},

		// v2 config
		{"schema/json/v2/ConfigReadParams.json", reflect.TypeOf(ConfigReadParams{})},
		{"schema/json/v2/ConfigReadResponse.json", reflect.TypeOf(ConfigReadResponse{})},
		{"schema/json/v2/ConfigRequirementsReadResponse.json", reflect.TypeOf(ConfigRequirementsReadResponse{})},
		{"schema/json/v2/ConfigValueWriteParams.json", reflect.TypeOf(ConfigValueWriteParams{})},
		{"schema/json/v2/ConfigBatchWriteParams.json", reflect.TypeOf(ConfigBatchWriteParams{})},
		{"schema/json/v2/ConfigWriteResponse.json", reflect.TypeOf(ConfigWriteResponse{})},
		{"schema/json/v2/ConfigWarningNotification.json", reflect.TypeOf(ConfigWarningNotification{})},

		// v2 experimental
		{"schema/json/v2/ExperimentalFeatureListParams.json", reflect.TypeOf(ExperimentalFeatureListParams{})},
		{"schema/json/v2/ExperimentalFeatureListResponse.json", reflect.TypeOf(ExperimentalFeatureListResponse{})},
		{"schema/json/v2/ExperimentalFeatureEnablementSetParams.json", reflect.TypeOf(ExperimentalFeatureEnablementSetParams{})},
		{"schema/json/v2/ExperimentalFeatureEnablementSetResponse.json", reflect.TypeOf(ExperimentalFeatureEnablementSetResponse{})},

		// v2 external agent
		{"schema/json/v2/ExternalAgentConfigDetectParams.json", reflect.TypeOf(ExternalAgentConfigDetectParams{})},
		{"schema/json/v2/ExternalAgentConfigDetectResponse.json", reflect.TypeOf(ExternalAgentConfigDetectResponse{})},
		{"schema/json/v2/ExternalAgentConfigImportParams.json", reflect.TypeOf(ExternalAgentConfigImportParams{})},
		{"schema/json/v2/ExternalAgentConfigImportCompletedNotification.json", reflect.TypeOf(ExternalAgentConfigImportCompletedNotification{})},

		// v2 feedback
		{"schema/json/v2/FeedbackUploadParams.json", reflect.TypeOf(FeedbackUploadParams{})},
		{"schema/json/v2/FeedbackUploadResponse.json", reflect.TypeOf(FeedbackUploadResponse{})},

		// v2 filesystem
		{"schema/json/v2/FsReadFileParams.json", reflect.TypeOf(FsReadFileParams{})},
		{"schema/json/v2/FsReadFileResponse.json", reflect.TypeOf(FsReadFileResponse{})},
		{"schema/json/v2/FsWriteFileParams.json", reflect.TypeOf(FsWriteFileParams{})},
		{"schema/json/v2/FsWriteFileResponse.json", reflect.TypeOf(FsWriteFileResponse{})},
		{"schema/json/v2/FsCreateDirectoryParams.json", reflect.TypeOf(FsCreateDirectoryParams{})},
		{"schema/json/v2/FsCreateDirectoryResponse.json", reflect.TypeOf(FsCreateDirectoryResponse{})},
		{"schema/json/v2/FsGetMetadataParams.json", reflect.TypeOf(FsGetMetadataParams{})},
		{"schema/json/v2/FsGetMetadataResponse.json", reflect.TypeOf(FsGetMetadataResponse{})},
		{"schema/json/v2/FsReadDirectoryParams.json", reflect.TypeOf(FsReadDirectoryParams{})},
		{"schema/json/v2/FsReadDirectoryResponse.json", reflect.TypeOf(FsReadDirectoryResponse{})},
		{"schema/json/v2/FsRemoveParams.json", reflect.TypeOf(FsRemoveParams{})},
		{"schema/json/v2/FsRemoveResponse.json", reflect.TypeOf(FsRemoveResponse{})},
		{"schema/json/v2/FsCopyParams.json", reflect.TypeOf(FsCopyParams{})},
		{"schema/json/v2/FsCopyResponse.json", reflect.TypeOf(FsCopyResponse{})},
		{"schema/json/v2/FsWatchParams.json", reflect.TypeOf(FsWatchParams{})},
		{"schema/json/v2/FsWatchResponse.json", reflect.TypeOf(FsWatchResponse{})},
		{"schema/json/v2/FsUnwatchParams.json", reflect.TypeOf(FsUnwatchParams{})},
		{"schema/json/v2/FsUnwatchResponse.json", reflect.TypeOf(FsUnwatchResponse{})},
		{"schema/json/v2/FsChangedNotification.json", reflect.TypeOf(FsChangedNotification{})},

		// v2 streaming notifications
		{"schema/json/v2/AgentMessageDeltaNotification.json", reflect.TypeOf(AgentMessageDeltaNotification{})},
		{"schema/json/v2/FileChangeOutputDeltaNotification.json", reflect.TypeOf(FileChangeOutputDeltaNotification{})},
		{"schema/json/v2/FileChangePatchUpdatedNotification.json", reflect.TypeOf(FileChangePatchUpdatedNotification{})},
		{"schema/json/v2/PlanDeltaNotification.json", reflect.TypeOf(PlanDeltaNotification{})},
		{"schema/json/v2/ReasoningTextDeltaNotification.json", reflect.TypeOf(ReasoningTextDeltaNotification{})},
		{"schema/json/v2/ReasoningSummaryTextDeltaNotification.json", reflect.TypeOf(ReasoningSummaryTextDeltaNotification{})},
		{"schema/json/v2/ReasoningSummaryPartAddedNotification.json", reflect.TypeOf(ReasoningSummaryPartAddedNotification{})},
		{"schema/json/v2/ItemStartedNotification.json", reflect.TypeOf(ItemStartedNotification{})},
		{"schema/json/v2/ItemCompletedNotification.json", reflect.TypeOf(ItemCompletedNotification{})},

		// v2 MCP
		{"schema/json/v2/ListMcpServerStatusParams.json", reflect.TypeOf(ListMcpServerStatusParams{})},
		{"schema/json/v2/ListMcpServerStatusResponse.json", reflect.TypeOf(ListMcpServerStatusResponse{})},
		{"schema/json/v2/McpServerOauthLoginParams.json", reflect.TypeOf(McpServerOauthLoginParams{})},
		{"schema/json/v2/McpServerOauthLoginResponse.json", reflect.TypeOf(McpServerOauthLoginResponse{})},
		{"schema/json/v2/McpResourceReadParams.json", reflect.TypeOf(McpResourceReadParams{})},
		{"schema/json/v2/McpResourceReadResponse.json", reflect.TypeOf(McpResourceReadResponse{})},
		{"schema/json/v2/McpServerToolCallParams.json", reflect.TypeOf(McpServerToolCallParams{})},
		{"schema/json/v2/McpServerToolCallResponse.json", reflect.TypeOf(McpServerToolCallResponse{})},
		{"schema/json/v2/McpServerOauthLoginCompletedNotification.json", reflect.TypeOf(McpServerOauthLoginCompletedNotification{})},
		{"schema/json/v2/McpServerStatusUpdatedNotification.json", reflect.TypeOf(McpServerStatusUpdatedNotification{})},
		{"schema/json/v2/McpToolCallProgressNotification.json", reflect.TypeOf(McpToolCallProgressNotification{})},

		// v2 model
		{"schema/json/v2/ModelListParams.json", reflect.TypeOf(ModelListParams{})},
		{"schema/json/v2/ModelListResponse.json", reflect.TypeOf(ModelListResponse{})},
		{"schema/json/v2/ModelProviderCapabilitiesReadParams.json", reflect.TypeOf(ModelProviderCapabilitiesReadParams{})},
		{"schema/json/v2/ModelProviderCapabilitiesReadResponse.json", reflect.TypeOf(ModelProviderCapabilitiesReadResponse{})},
		{"schema/json/v2/ModelReroutedNotification.json", reflect.TypeOf(ModelReroutedNotification{})},
		{"schema/json/v2/ModelVerificationNotification.json", reflect.TypeOf(ModelVerificationNotification{})},

		// v2 review
		{"schema/json/v2/ReviewStartParams.json", reflect.TypeOf(ReviewStartParams{})},
		{"schema/json/v2/ReviewStartResponse.json", reflect.TypeOf(ReviewStartResponse{})},

		// v2 plugin
		{"schema/json/v2/PluginListParams.json", reflect.TypeOf(PluginListParams{})},
		{"schema/json/v2/PluginListResponse.json", reflect.TypeOf(PluginListResponse{})},
		{"schema/json/v2/PluginInstalledParams.json", reflect.TypeOf(PluginInstalledParams{})},
		{"schema/json/v2/PluginInstalledResponse.json", reflect.TypeOf(PluginInstalledResponse{})},
		{"schema/json/v2/PluginReadParams.json", reflect.TypeOf(PluginReadParams{})},
		{"schema/json/v2/PluginReadResponse.json", reflect.TypeOf(PluginReadResponse{})},
		{"schema/json/v2/PluginInstallParams.json", reflect.TypeOf(PluginInstallParams{})},
		{"schema/json/v2/PluginInstallResponse.json", reflect.TypeOf(PluginInstallResponse{})},
		{"schema/json/v2/PluginUninstallParams.json", reflect.TypeOf(PluginUninstallParams{})},
		{"schema/json/v2/PluginUninstallResponse.json", reflect.TypeOf(PluginUninstallResponse{})},
		{"schema/json/v2/PluginSkillReadParams.json", reflect.TypeOf(PluginSkillReadParams{})},
		{"schema/json/v2/PluginSkillReadResponse.json", reflect.TypeOf(PluginSkillReadResponse{})},
		{"schema/json/v2/PluginShareCheckoutParams.json", reflect.TypeOf(PluginShareCheckoutParams{})},
		{"schema/json/v2/PluginShareCheckoutResponse.json", reflect.TypeOf(PluginShareCheckoutResponse{})},
		{"schema/json/v2/PluginShareDeleteParams.json", reflect.TypeOf(PluginShareDeleteParams{})},
		{"schema/json/v2/PluginShareListResponse.json", reflect.TypeOf(PluginShareListResponse{})},
		{"schema/json/v2/PluginShareSaveParams.json", reflect.TypeOf(PluginShareSaveParams{})},
		{"schema/json/v2/PluginShareSaveResponse.json", reflect.TypeOf(PluginShareSaveResponse{})},
		{"schema/json/v2/PluginShareUpdateTargetsParams.json", reflect.TypeOf(PluginShareUpdateTargetsParams{})},
		{"schema/json/v2/PluginShareUpdateTargetsResponse.json", reflect.TypeOf(PluginShareUpdateTargetsResponse{})},

		// v2 marketplace
		{"schema/json/v2/MarketplaceAddParams.json", reflect.TypeOf(MarketplaceAddParams{})},
		{"schema/json/v2/MarketplaceAddResponse.json", reflect.TypeOf(MarketplaceAddResponse{})},
		{"schema/json/v2/MarketplaceRemoveParams.json", reflect.TypeOf(MarketplaceRemoveParams{})},
		{"schema/json/v2/MarketplaceRemoveResponse.json", reflect.TypeOf(MarketplaceRemoveResponse{})},
		{"schema/json/v2/MarketplaceUpgradeParams.json", reflect.TypeOf(MarketplaceUpgradeParams{})},
		{"schema/json/v2/MarketplaceUpgradeResponse.json", reflect.TypeOf(MarketplaceUpgradeResponse{})},

		// v2 permission profiles
		{"schema/json/v2/PermissionProfileListParams.json", reflect.TypeOf(PermissionProfileListParams{})},
		{"schema/json/v2/PermissionProfileListResponse.json", reflect.TypeOf(PermissionProfileListResponse{})},

		// v2 skills
		{"schema/json/v2/SkillsListParams.json", reflect.TypeOf(SkillsListParams{})},
		{"schema/json/v2/SkillsListResponse.json", reflect.TypeOf(SkillsListResponse{})},
		{"schema/json/v2/SkillsConfigWriteParams.json", reflect.TypeOf(SkillsConfigWriteParams{})},
		{"schema/json/v2/SkillsConfigWriteResponse.json", reflect.TypeOf(SkillsConfigWriteResponse{})},
		{"schema/json/v2/SkillsChangedNotification.json", reflect.TypeOf(SkillsChangedNotification{})},

		// v2 hooks
		{"schema/json/v2/HooksListParams.json", reflect.TypeOf(HooksListParams{})},
		{"schema/json/v2/HooksListResponse.json", reflect.TypeOf(HooksListResponse{})},

		// v2 system
		{"schema/json/v2/HookStartedNotification.json", reflect.TypeOf(HookStartedNotification{})},
		{"schema/json/v2/HookCompletedNotification.json", reflect.TypeOf(HookCompletedNotification{})},
		{"schema/json/v2/ItemGuardianApprovalReviewStartedNotification.json", reflect.TypeOf(ItemGuardianApprovalReviewStartedNotification{})},
		{"schema/json/v2/ItemGuardianApprovalReviewCompletedNotification.json", reflect.TypeOf(ItemGuardianApprovalReviewCompletedNotification{})},
		{"schema/json/v2/WindowsSandboxSetupStartParams.json", reflect.TypeOf(WindowsSandboxSetupStartParams{})},
		{"schema/json/v2/WindowsSandboxSetupStartResponse.json", reflect.TypeOf(WindowsSandboxSetupStartResponse{})},
		{"schema/json/v2/WindowsSandboxReadinessResponse.json", reflect.TypeOf(WindowsSandboxReadinessResponse{})},
		{"schema/json/v2/WindowsSandboxSetupCompletedNotification.json", reflect.TypeOf(WindowsSandboxSetupCompletedNotification{})},
		{"schema/json/v2/WindowsWorldWritableWarningNotification.json", reflect.TypeOf(WindowsWorldWritableWarningNotification{})},
		{"schema/json/v2/ContextCompactedNotification.json", reflect.TypeOf(ContextCompactedNotification{})},
		{"schema/json/v2/DeprecationNoticeNotification.json", reflect.TypeOf(DeprecationNoticeNotification{})},
		{"schema/json/v2/ErrorNotification.json", reflect.TypeOf(ErrorNotification{})},
		{"schema/json/v2/WarningNotification.json", reflect.TypeOf(WarningNotification{})},
		{"schema/json/v2/GuardianWarningNotification.json", reflect.TypeOf(GuardianWarningNotification{})},
		{"schema/json/v2/RemoteControlStatusChangedNotification.json", reflect.TypeOf(RemoteControlStatusChangedNotification{})},
		{"schema/json/v2/TerminalInteractionNotification.json", reflect.TypeOf(TerminalInteractionNotification{})},

		// v2 thread
		{"schema/json/v2/ThreadStartParams.json", reflect.TypeOf(ThreadStartParams{})},
		{"schema/json/v2/ThreadStartResponse.json", reflect.TypeOf(ThreadStartResponse{})},
		{"schema/json/v2/ThreadReadParams.json", reflect.TypeOf(ThreadReadParams{})},
		{"schema/json/v2/ThreadReadResponse.json", reflect.TypeOf(ThreadReadResponse{})},
		{"schema/json/v2/ThreadListParams.json", reflect.TypeOf(ThreadListParams{})},
		{"schema/json/v2/ThreadListResponse.json", reflect.TypeOf(ThreadListResponse{})},
		{"schema/json/v2/ThreadLoadedListParams.json", reflect.TypeOf(ThreadLoadedListParams{})},
		{"schema/json/v2/ThreadLoadedListResponse.json", reflect.TypeOf(ThreadLoadedListResponse{})},
		{"schema/json/v2/ThreadShellCommandParams.json", reflect.TypeOf(ThreadShellCommandParams{})},
		{"schema/json/v2/ThreadShellCommandResponse.json", reflect.TypeOf(ThreadShellCommandResponse{})},
		{"schema/json/v2/ThreadApproveGuardianDeniedActionParams.json", reflect.TypeOf(ThreadApproveGuardianDeniedActionParams{})},
		{"schema/json/v2/ThreadApproveGuardianDeniedActionResponse.json", reflect.TypeOf(ThreadApproveGuardianDeniedActionResponse{})},
		{"schema/json/v2/ThreadInjectItemsParams.json", reflect.TypeOf(ThreadInjectItemsParams{})},
		{"schema/json/v2/ThreadInjectItemsResponse.json", reflect.TypeOf(ThreadInjectItemsResponse{})},
		{"schema/json/v2/ThreadResumeParams.json", reflect.TypeOf(ThreadResumeParams{})},
		{"schema/json/v2/ThreadResumeResponse.json", reflect.TypeOf(ThreadResumeResponse{})},
		{"schema/json/v2/ThreadForkParams.json", reflect.TypeOf(ThreadForkParams{})},
		{"schema/json/v2/ThreadForkResponse.json", reflect.TypeOf(ThreadForkResponse{})},
		{"schema/json/v2/ThreadMetadataUpdateParams.json", reflect.TypeOf(ThreadMetadataUpdateParams{})},
		{"schema/json/v2/ThreadMetadataUpdateResponse.json", reflect.TypeOf(ThreadMetadataUpdateResponse{})},
		{"schema/json/v2/ThreadRollbackParams.json", reflect.TypeOf(ThreadRollbackParams{})},
		{"schema/json/v2/ThreadRollbackResponse.json", reflect.TypeOf(ThreadRollbackResponse{})},
		{"schema/json/v2/ThreadSetNameParams.json", reflect.TypeOf(ThreadSetNameParams{})},
		{"schema/json/v2/ThreadSetNameResponse.json", reflect.TypeOf(ThreadSetNameResponse{})},
		{"schema/json/v2/ThreadArchiveParams.json", reflect.TypeOf(ThreadArchiveParams{})},
		{"schema/json/v2/ThreadUnarchiveParams.json", reflect.TypeOf(ThreadUnarchiveParams{})},
		{"schema/json/v2/ThreadUnsubscribeParams.json", reflect.TypeOf(ThreadUnsubscribeParams{})},
		{"schema/json/v2/ThreadUnsubscribeResponse.json", reflect.TypeOf(ThreadUnsubscribeResponse{})},
		{"schema/json/v2/ThreadCompactStartParams.json", reflect.TypeOf(ThreadCompactStartParams{})},

		// v2 thread notifications
		{"schema/json/v2/ThreadStartedNotification.json", reflect.TypeOf(ThreadStartedNotification{})},
		{"schema/json/v2/ThreadClosedNotification.json", reflect.TypeOf(ThreadClosedNotification{})},
		{"schema/json/v2/ThreadArchivedNotification.json", reflect.TypeOf(ThreadArchivedNotification{})},
		{"schema/json/v2/ThreadUnarchivedNotification.json", reflect.TypeOf(ThreadUnarchivedNotification{})},
		{"schema/json/v2/ThreadGoalUpdatedNotification.json", reflect.TypeOf(ThreadGoalUpdatedNotification{})},
		{"schema/json/v2/ThreadGoalClearedNotification.json", reflect.TypeOf(ThreadGoalClearedNotification{})},
		{"schema/json/v2/ThreadSettingsUpdatedNotification.json", reflect.TypeOf(ThreadSettingsUpdatedNotification{})},
		{"schema/json/v2/ThreadNameUpdatedNotification.json", reflect.TypeOf(ThreadNameUpdatedNotification{})},
		{"schema/json/v2/ThreadStatusChangedNotification.json", reflect.TypeOf(ThreadStatusChangedNotification{})},
		{"schema/json/v2/ThreadTokenUsageUpdatedNotification.json", reflect.TypeOf(ThreadTokenUsageUpdatedNotification{})},
		{"schema/json/v2/ServerRequestResolvedNotification.json", reflect.TypeOf(ServerRequestResolvedNotification{})},

		// v2 realtime
		{"schema/json/v2/ThreadRealtimeStartedNotification.json", reflect.TypeOf(ThreadRealtimeStartedNotification{})},
		{"schema/json/v2/ThreadRealtimeClosedNotification.json", reflect.TypeOf(ThreadRealtimeClosedNotification{})},
		{"schema/json/v2/ThreadRealtimeErrorNotification.json", reflect.TypeOf(ThreadRealtimeErrorNotification{})},
		{"schema/json/v2/ThreadRealtimeItemAddedNotification.json", reflect.TypeOf(ThreadRealtimeItemAddedNotification{})},
		{"schema/json/v2/ThreadRealtimeOutputAudioDeltaNotification.json", reflect.TypeOf(ThreadRealtimeOutputAudioDeltaNotification{})},
		{"schema/json/v2/ThreadRealtimeSdpNotification.json", reflect.TypeOf(ThreadRealtimeSdpNotification{})},
		{"schema/json/v2/ThreadRealtimeTranscriptDeltaNotification.json", reflect.TypeOf(ThreadRealtimeTranscriptDeltaNotification{})},
		{"schema/json/v2/ThreadRealtimeTranscriptDoneNotification.json", reflect.TypeOf(ThreadRealtimeTranscriptDoneNotification{})},

		// v2 process notifications
		{"schema/json/v2/ProcessOutputDeltaNotification.json", reflect.TypeOf(ProcessOutputDeltaNotification{})},
		{"schema/json/v2/ProcessExitedNotification.json", reflect.TypeOf(ProcessExitedNotification{})},

		// v2 turn
		{"schema/json/v2/TurnStartParams.json", reflect.TypeOf(TurnStartParams{})},
		{"schema/json/v2/TurnStartResponse.json", reflect.TypeOf(TurnStartResponse{})},
		{"schema/json/v2/TurnInterruptParams.json", reflect.TypeOf(TurnInterruptParams{})},
		{"schema/json/v2/TurnSteerParams.json", reflect.TypeOf(TurnSteerParams{})},
		{"schema/json/v2/TurnSteerResponse.json", reflect.TypeOf(TurnSteerResponse{})},

		// v2 turn notifications
		{"schema/json/v2/TurnStartedNotification.json", reflect.TypeOf(TurnStartedNotification{})},
		{"schema/json/v2/TurnCompletedNotification.json", reflect.TypeOf(TurnCompletedNotification{})},
		{"schema/json/v2/TurnPlanUpdatedNotification.json", reflect.TypeOf(TurnPlanUpdatedNotification{})},
		{"schema/json/v2/TurnDiffUpdatedNotification.json", reflect.TypeOf(TurnDiffUpdatedNotification{})},
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

	// ThreadItem variant structs — these live inside definitions.ThreadItem.oneOf
	// in the ItemStartedNotification spec, not as separate spec files.
	specFile := "schema/json/v2/ItemStartedNotification.json"
	variantRegistry := []definitionVariantEntry{
		{specFile, "ThreadItem", "UserMessageThreadItem", reflect.TypeOf(UserMessageThreadItem{})},
		{specFile, "ThreadItem", "AgentMessageThreadItem", reflect.TypeOf(AgentMessageThreadItem{})},
		{specFile, "ThreadItem", "PlanThreadItem", reflect.TypeOf(PlanThreadItem{})},
		{specFile, "ThreadItem", "ReasoningThreadItem", reflect.TypeOf(ReasoningThreadItem{})},
		{specFile, "ThreadItem", "CommandExecutionThreadItem", reflect.TypeOf(CommandExecutionThreadItem{})},
		{specFile, "ThreadItem", "FileChangeThreadItem", reflect.TypeOf(FileChangeThreadItem{})},
		{specFile, "ThreadItem", "McpToolCallThreadItem", reflect.TypeOf(McpToolCallThreadItem{})},
		{specFile, "ThreadItem", "DynamicToolCallThreadItem", reflect.TypeOf(DynamicToolCallThreadItem{})},
		{specFile, "ThreadItem", "CollabAgentToolCallThreadItem", reflect.TypeOf(CollabAgentToolCallThreadItem{})},
		{specFile, "ThreadItem", "WebSearchThreadItem", reflect.TypeOf(WebSearchThreadItem{})},
		{specFile, "ThreadItem", "ImageViewThreadItem", reflect.TypeOf(ImageViewThreadItem{})},
		{specFile, "ThreadItem", "EnteredReviewModeThreadItem", reflect.TypeOf(EnteredReviewModeThreadItem{})},
		{specFile, "ThreadItem", "ExitedReviewModeThreadItem", reflect.TypeOf(ExitedReviewModeThreadItem{})},
		{specFile, "ThreadItem", "ContextCompactionThreadItem", reflect.TypeOf(ContextCompactionThreadItem{})},
	}

	for _, entry := range variantRegistry {
		entry := entry
		name := entry.specPath + "/definitions." + entry.defName + "." + entry.variantTitle
		t.Run(name, func(t *testing.T) {
			properties, required, err := schemaDefinitionVariantFields(
				entry.specPath, entry.defName, entry.variantTitle,
			)
			if err != nil {
				t.Fatalf("failed to parse schema variant: %v", err)
			}
			if len(properties) == 0 {
				t.Skip("variant has no properties")
			}

			goFields := structJSONFields(entry.goType)

			for _, prop := range properties {
				// Skip the "type" discriminator field — it's handled by MarshalJSON/UnmarshalJSON,
				// not stored as a struct field.
				if prop == "type" {
					continue
				}

				fi, ok := goFields[prop]
				if !ok {
					t.Errorf("schema property %q has no matching JSON field on %s", prop, entry.goType.Name())
					continue
				}

				if required[prop] && fi.isOptional {
					t.Errorf("schema requires %q but Go field %s.%s has omitempty", prop, entry.goType.Name(), fi.fieldName)
				}
			}
		})
	}

	topLevelVariants := []topLevelVariantEntry{
		{"schema/json/v2/LoginAccountResponse.json", "ApiKeyv2::LoginAccountResponse", reflect.TypeOf(ApiKeyLoginAccountResponse{})},
		{"schema/json/v2/LoginAccountResponse.json", "Chatgptv2::LoginAccountResponse", reflect.TypeOf(ChatgptLoginAccountResponse{})},
		{"schema/json/v2/LoginAccountResponse.json", "ChatgptAuthTokensv2::LoginAccountResponse", reflect.TypeOf(ChatgptAuthTokensLoginAccountResponse{})},
	}

	for _, entry := range topLevelVariants {
		entry := entry
		name := entry.specPath + "/oneOf." + entry.variantTitle
		t.Run(name, func(t *testing.T) {
			properties, required, err := schemaTopLevelVariantFields(entry.specPath, entry.variantTitle)
			if err != nil {
				t.Fatalf("failed to parse schema variant: %v", err)
			}
			if len(properties) == 0 {
				t.Skip("variant has no properties")
			}

			goFields := structJSONFields(entry.goType)
			for _, prop := range properties {
				fi, ok := goFields[prop]
				if !ok {
					t.Errorf("schema property %q has no matching JSON field on %s", prop, entry.goType.Name())
					continue
				}
				if required[prop] && fi.isOptional {
					t.Errorf("schema requires %q but Go field %s.%s has omitempty", prop, entry.goType.Name(), fi.fieldName)
				}
			}
		})
	}

	topLevelRequiredVariants := []topLevelRequiredVariantEntry{
		{"schema/json/McpServerElicitationRequestParams.json", []string{"message", "mode", "requestedSchema"}, reflect.TypeOf(McpServerElicitationRequestParams{})},
		{"schema/json/McpServerElicitationRequestParams.json", []string{"elicitationId", "message", "mode", "url"}, reflect.TypeOf(McpServerElicitationRequestParams{})},
	}

	for _, entry := range topLevelRequiredVariants {
		entry := entry
		name := entry.specPath + "/oneOf.required=" + strings.Join(entry.requiredFields, ",")
		t.Run(name, func(t *testing.T) {
			properties, _, err := schemaTopLevelVariantFieldsByRequired(entry.specPath, entry.requiredFields)
			if err != nil {
				t.Fatalf("failed to parse schema variant: %v", err)
			}
			if len(properties) == 0 {
				t.Skip("variant has no properties")
			}

			goFields := structJSONFields(entry.goType)
			for _, prop := range properties {
				_, ok := goFields[prop]
				if !ok {
					t.Errorf("schema property %q has no matching JSON field on %s", prop, entry.goType.Name())
				}
			}
		})
	}

	definitionStructs := []definitionStructEntry{
		{"schema/json/v2/AppsListResponse.json", "AppInfo", reflect.TypeOf(AppInfo{})},
		{"schema/json/v2/AppsListResponse.json", "AppBranding", reflect.TypeOf(AppBranding{})},
		{"schema/json/v2/AppsListResponse.json", "AppMetadata", reflect.TypeOf(AppMetadata{})},
		{"schema/json/v2/AppsListResponse.json", "AppReview", reflect.TypeOf(AppReview{})},
		{"schema/json/v2/AppsListResponse.json", "AppScreenshot", reflect.TypeOf(AppScreenshot{})},
		{"schema/json/v2/ConfigRequirementsReadResponse.json", "ConfigRequirements", reflect.TypeOf(ConfigRequirements{})},
		{"schema/json/v2/ConfigRequirementsReadResponse.json", "ComputerUseRequirements", reflect.TypeOf(ComputerUseRequirements{})},
		{"schema/json/v2/HookStartedNotification.json", "HookOutputEntry", reflect.TypeOf(HookOutputEntry{})},
		{"schema/json/v2/HookStartedNotification.json", "HookRunSummary", reflect.TypeOf(HookRunSummary{})},
		{"schema/json/v2/ItemStartedNotification.json", "MemoryCitation", reflect.TypeOf(MemoryCitation{})},
		{"schema/json/v2/ItemStartedNotification.json", "MemoryCitationEntry", reflect.TypeOf(MemoryCitationEntry{})},
		{"schema/json/v2/ItemGuardianApprovalReviewStartedNotification.json", "GuardianApprovalReview", reflect.TypeOf(GuardianApprovalReview{})},
		{"schema/json/v2/MarketplaceUpgradeResponse.json", "MarketplaceUpgradeErrorInfo", reflect.TypeOf(MarketplaceUpgradeErrorInfo{})},
		{"schema/json/v2/PermissionProfileListResponse.json", "PermissionProfileSummary", reflect.TypeOf(PermissionProfileSummary{})},
		{"schema/json/v2/PluginListResponse.json", "MarketplaceLoadErrorInfo", reflect.TypeOf(MarketplaceLoadErrorInfo{})},
		{"schema/json/v2/ThreadSettingsUpdatedNotification.json", "ActivePermissionProfile", reflect.TypeOf(ActivePermissionProfile{})},
		{"schema/json/v2/ThreadSettingsUpdatedNotification.json", "ThreadSettings", reflect.TypeOf(ThreadSettings{})},
		{"schema/json/v2/ThreadGoalUpdatedNotification.json", "ThreadGoal", reflect.TypeOf(ThreadGoal{})},
		{"schema/json/v2/ThreadRealtimeOutputAudioDeltaNotification.json", "ThreadRealtimeAudioChunk", reflect.TypeOf(ThreadRealtimeAudioChunk{})},
	}

	for _, entry := range definitionStructs {
		entry := entry
		name := entry.specPath + "/definitions." + entry.defName
		t.Run(name, func(t *testing.T) {
			properties, required, err := schemaDefinitionFields(entry.specPath, entry.defName)
			if err != nil {
				t.Fatalf("failed to parse schema definition: %v", err)
			}
			if len(properties) == 0 {
				t.Skip("definition has no properties")
			}

			goFields := structJSONFields(entry.goType)
			for _, prop := range properties {
				fi, ok := goFields[prop]
				if !ok {
					t.Errorf("schema property %q has no matching JSON field on %s", prop, entry.goType.Name())
					continue
				}
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
			specPath: "schema/json/v2/ModelReroutedNotification.json",
			defName:  "ModelRerouteReason",
			goValues: enumStrings(
				ModelRerouteReasonHighRiskCyberActivity,
			),
		},
		{
			specPath: "schema/json/v2/AccountRateLimitsUpdatedNotification.json",
			defName:  "PlanType",
			goValues: enumStrings(
				PlanTypeFree, PlanTypeGo, PlanTypePlus, PlanTypePro, PlanTypeProLite,
				PlanTypeTeam, PlanTypeBusiness, PlanTypeEnterprise,
				PlanTypeEdu, PlanTypeSelfServeBusinessUsageBased,
				PlanTypeEnterpriseCBPUsageBased, PlanTypeUnknown,
			),
		},
		{
			specPath: "schema/json/v2/AccountUpdatedNotification.json",
			defName:  "PlanType",
			goValues: enumStrings(
				PlanTypeFree, PlanTypeGo, PlanTypePlus, PlanTypePro, PlanTypeProLite,
				PlanTypeTeam, PlanTypeBusiness, PlanTypeEnterprise,
				PlanTypeEdu, PlanTypeSelfServeBusinessUsageBased,
				PlanTypeEnterpriseCBPUsageBased, PlanTypeUnknown,
			),
		},
		{
			specPath: "schema/json/v2/AccountUpdatedNotification.json",
			defName:  "AuthMode",
			goValues: enumStrings(
				AuthModeAPIKey, AuthModeChatGPT, AuthModeChatGPTAuthTokens,
				AuthModeAgentIdentity,
			),
		},
		{
			specPath: "schema/json/v2/CancelLoginAccountResponse.json",
			defName:  "CancelLoginAccountStatus",
			goValues: enumStrings(
				CancelLoginAccountStatusCanceled, CancelLoginAccountStatusNotFound,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookEventName",
			goValues: enumStrings(
				HookEventNameSessionStart, HookEventNameUserPromptSubmit,
				HookEventNameSubagentStart,
				HookEventNamePreToolUse, HookEventNamePermissionRequest,
				HookEventNamePostToolUse, HookEventNamePreCompact,
				HookEventNamePostCompact, HookEventNameStop,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookExecutionMode",
			goValues: enumStrings(
				HookExecutionModeSync, HookExecutionModeAsync,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookHandlerType",
			goValues: enumStrings(
				HookHandlerTypeCommand, HookHandlerTypePrompt, HookHandlerTypeAgent,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookOutputEntryKind",
			goValues: enumStrings(
				HookOutputEntryKindWarning, HookOutputEntryKindStop, HookOutputEntryKindFeedback,
				HookOutputEntryKindContext, HookOutputEntryKindError,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookRunStatus",
			goValues: enumStrings(
				HookRunStatusRunning, HookRunStatusCompleted, HookRunStatusFailed,
				HookRunStatusBlocked, HookRunStatusStopped,
			),
		},
		{
			specPath: "schema/json/v2/HookStartedNotification.json",
			defName:  "HookScope",
			goValues: enumStrings(
				HookScopeThread, HookScopeTurn,
			),
		},
		{
			specPath: "schema/json/v2/ItemGuardianApprovalReviewStartedNotification.json",
			defName:  "GuardianApprovalReviewStatus",
			goValues: enumStrings(
				GuardianApprovalReviewStatusInProgress, GuardianApprovalReviewStatusApproved,
				GuardianApprovalReviewStatusDenied, GuardianApprovalReviewStatusAborted,
				GuardianApprovalReviewStatusTimedOut,
			),
		},
		{
			specPath: "schema/json/v2/ItemGuardianApprovalReviewStartedNotification.json",
			defName:  "GuardianRiskLevel",
			goValues: enumStrings(
				GuardianRiskLevelLow, GuardianRiskLevelMedium, GuardianRiskLevelHigh,
				GuardianRiskLevelCritical,
			),
		},

		// Enums from enums.go
		{
			specPath: "schema/json/v2/ReviewStartResponse.json",
			defName:  "TurnStatus",
			goValues: enumStrings(
				TurnStatusCompleted, TurnStatusInterrupted,
				TurnStatusFailed, TurnStatusInProgress,
			),
		},
		{
			specPath: "schema/json/v2/ThreadStartParams.json",
			defName:  "Personality",
			goValues: enumStrings(
				PersonalityNone, PersonalityFriendly, PersonalityPragmatic,
			),
		},
		{
			specPath: "schema/json/v2/TurnStartParams.json",
			defName:  "ModeKind",
			goValues: enumStrings(
				ModeKindPlan, ModeKindDefault,
			),
		},
		{
			specPath: "schema/json/v2/ConfigBatchWriteParams.json",
			defName:  "MergeStrategy",
			goValues: enumStrings(
				MergeStrategyReplace, MergeStrategyUpsert,
			),
		},
		{
			specPath: "schema/json/v2/ConfigReadResponse.json",
			defName:  "Verbosity",
			goValues: enumStrings(
				VerbosityLow, VerbosityMedium, VerbosityHigh,
			),
		},
		{
			specPath: "schema/json/v2/ThreadStartParams.json",
			defName:  "SandboxMode",
			goValues: enumStrings(
				SandboxModeReadOnly, SandboxModeWorkspaceWrite,
				SandboxModeDangerFullAccess,
			),
		},
		{
			specPath: "schema/json/v2/ConfigReadResponse.json",
			defName:  "WebSearchMode",
			goValues: enumStrings(
				WebSearchModeDisabled, WebSearchModeCached, WebSearchModeLive,
			),
		},
		{
			specPath: "schema/json/v2/ConfigWriteResponse.json",
			defName:  "WriteStatus",
			goValues: enumStrings(
				WriteStatusOK, WriteStatusOKOverridden,
			),
		},
		{
			specPath: "schema/json/CommandExecutionRequestApprovalParams.json",
			defName:  "NetworkApprovalProtocol",
			goValues: enumStrings(
				NetworkApprovalProtocolHTTP, NetworkApprovalProtocolHTTPS,
				NetworkApprovalProtocolSocks5TCP, NetworkApprovalProtocolSocks5UDP,
			),
		},
		{
			specPath: "schema/json/CommandExecutionRequestApprovalParams.json",
			defName:  "NetworkPolicyRuleAction",
			goValues: enumStrings(
				NetworkPolicyRuleActionAllow, NetworkPolicyRuleActionDeny,
			),
		},
		{
			specPath: "schema/json/v2/TurnStartParams.json",
			defName:  "ReasoningEffort",
			goValues: enumStrings(
				ReasoningEffortNone, ReasoningEffortMinimal, ReasoningEffortLow,
				ReasoningEffortMedium, ReasoningEffortHigh, ReasoningEffortXHigh,
			),
		},
		{
			specPath: "schema/json/v2/ModelListResponse.json",
			defName:  "InputModality",
			goValues: enumStrings(
				InputModalityText, InputModalityImage,
			),
		},
		{
			specPath: "schema/json/FileChangeRequestApprovalResponse.json",
			defName:  "FileChangeApprovalDecision",
			goValues: enumStrings(
				FileChangeApprovalDecisionAccept, FileChangeApprovalDecisionAcceptForSession,
				FileChangeApprovalDecisionDecline, FileChangeApprovalDecisionCancel,
			),
		},
		{
			specPath: "schema/json/CommandExecutionRequestApprovalResponse.json",
			defName:  "CommandExecutionApprovalDecision",
			goValues: []string{
				CommandExecutionApprovalDecisionAccept,
				CommandExecutionApprovalDecisionAcceptForSession,
				CommandExecutionApprovalDecisionDecline,
				CommandExecutionApprovalDecisionCancel,
			},
		},
		{
			specPath: "schema/json/ChatgptAuthTokensRefreshParams.json",
			defName:  "ChatgptAuthTokensRefreshReason",
			goValues: enumStrings(
				ChatgptAuthTokensRefreshReasonUnauthorized,
			),
		},
		{
			specPath: "schema/json/v2/ConfigReadResponse.json",
			defName:  "AppToolApproval",
			goValues: enumStrings(
				AppToolApprovalAuto, AppToolApprovalPrompt, AppToolApprovalApprove,
			),
		},
		{
			specPath: "schema/json/v2/ConfigRequirementsReadResponse.json",
			defName:  "ResidencyRequirement",
			goValues: enumStrings(
				ResidencyRequirementUS,
			),
		},
		{
			specPath: "schema/json/v2/ConfigReadResponse.json",
			defName:  "ForcedLoginMethod",
			goValues: enumStrings(
				ForcedLoginMethodChatGPT, ForcedLoginMethodAPI,
			),
		},
		{
			specPath: "schema/json/v2/ConfigReadResponse.json",
			defName:  "ReasoningSummary",
			goValues: enumStrings(
				ReasoningSummaryModeAuto, ReasoningSummaryModeConcise,
				ReasoningSummaryModeDetailed, ReasoningSummaryModeNone,
			),
		},

		// Enums from event_types.go
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "MessagePhase",
			goValues: enumStrings(
				MessagePhaseCommentary, MessagePhaseFinalAnswer,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "CommandExecutionStatus",
			goValues: enumStrings(
				CommandExecutionStatusInProgress, CommandExecutionStatusCompleted,
				CommandExecutionStatusFailed, CommandExecutionStatusDeclined,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "PatchApplyStatus",
			goValues: enumStrings(
				PatchApplyStatusInProgress, PatchApplyStatusCompleted,
				PatchApplyStatusFailed, PatchApplyStatusDeclined,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "McpToolCallStatus",
			goValues: enumStrings(
				McpToolCallStatusInProgress, McpToolCallStatusCompleted,
				McpToolCallStatusFailed,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "DynamicToolCallStatus",
			goValues: enumStrings(
				DynamicToolCallStatusInProgress, DynamicToolCallStatusCompleted,
				DynamicToolCallStatusFailed,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentStatus",
			goValues: enumStrings(
				CollabAgentStatusPendingInit, CollabAgentStatusRunning,
				CollabAgentStatusInterrupted, CollabAgentStatusCompleted, CollabAgentStatusErrored,
				CollabAgentStatusShutdown, CollabAgentStatusNotFound,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentTool",
			goValues: enumStrings(
				CollabAgentToolSpawnAgent, CollabAgentToolSendInput,
				CollabAgentToolResumeAgent, CollabAgentToolWait,
				CollabAgentToolCloseAgent,
			),
		},
		{
			specPath: "schema/json/v2/ItemCompletedNotification.json",
			defName:  "CollabAgentToolCallStatus",
			goValues: enumStrings(
				CollabAgentToolCallStatusInProgress, CollabAgentToolCallStatusCompleted,
				CollabAgentToolCallStatusFailed,
			),
		},

		// Remaining enums from various files
		{
			specPath: "schema/json/v2/ListMcpServerStatusResponse.json",
			defName:  "McpAuthStatus",
			goValues: enumStrings(
				McpAuthStatusUnsupported, McpAuthStatusNotLoggedIn,
				McpAuthStatusBearerToken, McpAuthStatusOAuth,
			),
		},
		{
			specPath: "schema/json/v2/ReviewStartParams.json",
			defName:  "ReviewDelivery",
			goValues: enumStrings(
				ReviewDeliveryInline, ReviewDeliveryDetached,
			),
		},
		{
			specPath: "schema/json/v2/SkillsListResponse.json",
			defName:  "SkillScope",
			goValues: enumStrings(
				SkillScopeUser, SkillScopeRepo,
				SkillScopeSystem, SkillScopeAdmin,
			),
		},
		{
			specPath: "schema/json/v2/ExperimentalFeatureListResponse.json",
			defName:  "ExperimentalFeatureStage",
			goValues: enumStrings(
				ExperimentalFeatureStageBeta, ExperimentalFeatureStageUnderDevelopment,
				ExperimentalFeatureStageStable, ExperimentalFeatureStageDeprecated,
				ExperimentalFeatureStageRemoved,
			),
		},
		{
			specPath: "schema/json/v2/ExternalAgentConfigDetectResponse.json",
			defName:  "ExternalAgentConfigMigrationItemType",
			goValues: enumStrings(
				MigrationItemTypeAgentsMd, MigrationItemTypeConfig,
				MigrationItemTypeSkills, MigrationItemTypeMcpServerConfig,
				MigrationItemTypePlugins, MigrationItemTypeSubagents,
				MigrationItemTypeHooks, MigrationItemTypeCommands,
				MigrationItemTypeSessions,
			),
		},
		{
			specPath: "schema/json/v2/WindowsSandboxSetupCompletedNotification.json",
			defName:  "WindowsSandboxSetupMode",
			goValues: enumStrings(
				WindowsSandboxSetupModeElevated, WindowsSandboxSetupModeUnelevated,
			),
		},
		{
			specPath: "schema/json/v2/ThreadStatusChangedNotification.json",
			defName:  "ThreadActiveFlag",
			goValues: enumStrings(
				ThreadActiveFlagWaitingOnApproval, ThreadActiveFlagWaitingOnUserInput,
			),
		},
		{
			specPath: "schema/json/v2/CommandExecParams.json",
			defName:  "NetworkAccess",
			goValues: enumStrings(
				NetworkAccessRestricted, NetworkAccessEnabled,
			),
		},
		{
			specPath: "schema/json/v2/CommandExecOutputDeltaNotification.json",
			defName:  "CommandExecOutputStream",
			goValues: enumStrings(
				CommandExecOutputStreamStdout, CommandExecOutputStreamStderr,
			),
		},

		// Enums in v2 specs that are also in ClientRequest/EventMsg/codex_app_server
		{
			specPath: "schema/json/v2/ThreadListParams.json",
			defName:  "ThreadSortKey",
			goValues: enumStrings(
				ThreadSortKeyCreatedAt, ThreadSortKeyUpdatedAt,
			),
		},
		{
			specPath: "schema/json/v2/ThreadListParams.json",
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
			specPath: "schema/json/v2/ThreadResumeParams.json",
			defName:  "LocalShellStatus",
			goValues: enumStrings(
				LocalShellStatusCompleted, LocalShellStatusInProgress,
				LocalShellStatusIncomplete,
			),
		},
		{
			specPath: "schema/json/v2/ThreadUnsubscribeResponse.json",
			defName:  "ThreadUnsubscribeStatus",
			goValues: enumStrings(
				ThreadUnsubscribeStatusNotLoaded, ThreadUnsubscribeStatusNotSubscribed,
				ThreadUnsubscribeStatusUnsubscribed,
			),
		},
		{
			specPath: "schema/json/v2/TurnPlanUpdatedNotification.json",
			defName:  "TurnPlanStepStatus",
			goValues: enumStrings(
				TurnPlanStepStatusPending, TurnPlanStepStatusInProgress,
				TurnPlanStepStatusCompleted,
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
