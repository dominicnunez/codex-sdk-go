package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// ConfigReadParams represents parameters for config/read request
type ConfigReadParams struct {
	Cwd           *string `json:"cwd,omitempty"`
	IncludeLayers *bool   `json:"includeLayers,omitempty"`
}

// ConfigReadResponse represents response from config/read request
type ConfigReadResponse struct {
	Config  *Config                       `json:"config"`
	Layers  *[]ConfigLayer                `json:"layers,omitempty"`
	Origins map[string]ConfigLayerMetadata `json:"origins"`
}

// Config represents the effective configuration
type Config struct {
	Analytics                   *AnalyticsConfig       `json:"analytics,omitempty"`
	ApprovalPolicy              *AskForApprovalWrapper `json:"approval_policy,omitempty"`
	CompactPrompt               *string                `json:"compact_prompt,omitempty"`
	DeveloperInstructions       *string                `json:"developer_instructions,omitempty"`
	ForcedChatgptWorkspaceID    *string                `json:"forced_chatgpt_workspace_id,omitempty"`
	ForcedLoginMethod           *ForcedLoginMethod     `json:"forced_login_method,omitempty"`
	Instructions                *string                `json:"instructions,omitempty"`
	Model                       *string                `json:"model,omitempty"`
	ModelAutoCompactTokenLimit  *int64                 `json:"model_auto_compact_token_limit,omitempty"`
	ModelContextWindow          *int64                 `json:"model_context_window,omitempty"`
	ModelProvider               *string                `json:"model_provider,omitempty"`
	ModelReasoningEffort        *ReasoningEffort       `json:"model_reasoning_effort,omitempty"`
	ModelReasoningSummary       *ReasoningSummaryWrapper `json:"model_reasoning_summary,omitempty"`
	ModelVerbosity              *Verbosity             `json:"model_verbosity,omitempty"`
	Profile                     *string                `json:"profile,omitempty"`
	Profiles                    map[string]ProfileV2   `json:"profiles,omitempty"`
	ReviewModel                 *string                `json:"review_model,omitempty"`
	SandboxMode                 *SandboxMode           `json:"sandbox_mode,omitempty"`
	SandboxWorkspaceWrite       *SandboxWorkspaceWrite `json:"sandbox_workspace_write,omitempty"`
	Tools                       *ToolsV2               `json:"tools,omitempty"`
	WebSearch                   *WebSearchMode         `json:"web_search,omitempty"`
}

// AnalyticsConfig represents analytics configuration
type AnalyticsConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// ReasoningSummary interface for reasoning summary modes
type ReasoningSummary interface {
	isReasoningSummary()
}

// ReasoningSummaryMode represents enum variant ("auto" | "concise" | "detailed" | "none")
type ReasoningSummaryMode string

func (ReasoningSummaryMode) isReasoningSummary() {}

// ReasoningSummaryWrapper wraps ReasoningSummary for JSON marshaling
type ReasoningSummaryWrapper struct {
	Value ReasoningSummary
}

func (w *ReasoningSummaryWrapper) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	w.Value = ReasoningSummaryMode(str)
	return nil
}

func (w ReasoningSummaryWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil {
		return json.Marshal(nil)
	}
	if mode, ok := w.Value.(ReasoningSummaryMode); ok {
		return json.Marshal(string(mode))
	}
	return json.Marshal(nil)
}

// SandboxWorkspaceWrite represents workspace write settings
type SandboxWorkspaceWrite struct {
	ExcludeSlashTmp      *bool    `json:"exclude_slash_tmp,omitempty"`
	ExcludeTmpdirEnvVar  *bool    `json:"exclude_tmpdir_env_var,omitempty"`
	NetworkAccess        *bool    `json:"network_access,omitempty"`
	WritableRoots        []string `json:"writable_roots,omitempty"`
}

// ToolsV2 represents tools configuration
type ToolsV2 struct {
	ViewImage *bool `json:"view_image,omitempty"`
	WebSearch *bool `json:"web_search,omitempty"`
}

// ProfileV2 represents a named configuration profile
type ProfileV2 struct {
	ApprovalPolicy       *AskForApprovalWrapper   `json:"approval_policy,omitempty"`
	ChatgptBaseURL       *string                  `json:"chatgpt_base_url,omitempty"`
	Model                *string                  `json:"model,omitempty"`
	ModelProvider        *string                  `json:"model_provider,omitempty"`
	ModelReasoningEffort *ReasoningEffort          `json:"model_reasoning_effort,omitempty"`
	ModelReasoningSummary *ReasoningSummaryWrapper `json:"model_reasoning_summary,omitempty"`
	ModelVerbosity       *Verbosity               `json:"model_verbosity,omitempty"`
	WebSearch            *WebSearchMode           `json:"web_search,omitempty"`
}

// ConfigLayer represents a configuration layer
type ConfigLayer struct {
	Config         json.RawMessage         `json:"config"`
	DisabledReason *string                 `json:"disabledReason,omitempty"`
	Name           ConfigLayerSourceWrapper `json:"name"`
	Version        string                  `json:"version"`
}

// ConfigLayerMetadata represents metadata about a config layer
type ConfigLayerMetadata struct {
	Name    ConfigLayerSourceWrapper `json:"name"`
	Version string                  `json:"version"`
}

// ConfigLayerSource interface for discriminated union
type ConfigLayerSource interface {
	isConfigLayerSource()
}

// MdmConfigLayerSource represents MDM managed preferences
type MdmConfigLayerSource struct {
	Domain string `json:"domain"`
	Key    string `json:"key"`
}

func (MdmConfigLayerSource) isConfigLayerSource() {}

// SystemConfigLayerSource represents system managed config file
type SystemConfigLayerSource struct {
	File string `json:"file"`
}

func (SystemConfigLayerSource) isConfigLayerSource() {}

// UserConfigLayerSource represents user config from $CODEX_HOME/config.toml
type UserConfigLayerSource struct {
	File string `json:"file"`
}

func (UserConfigLayerSource) isConfigLayerSource() {}

// ProjectConfigLayerSource represents project .codex/ folder
type ProjectConfigLayerSource struct {
	DotCodexFolder string `json:"dotCodexFolder"`
}

func (ProjectConfigLayerSource) isConfigLayerSource() {}

// SessionFlagsConfigLayerSource represents session-layer overrides
type SessionFlagsConfigLayerSource struct{}

func (SessionFlagsConfigLayerSource) isConfigLayerSource() {}

// LegacyManagedConfigTomlFromFileConfigLayerSource represents legacy managed_config.toml from file
type LegacyManagedConfigTomlFromFileConfigLayerSource struct {
	File string `json:"file"`
}

func (LegacyManagedConfigTomlFromFileConfigLayerSource) isConfigLayerSource() {}

// LegacyManagedConfigTomlFromMdmConfigLayerSource represents legacy managed_config.toml from MDM
type LegacyManagedConfigTomlFromMdmConfigLayerSource struct{}

func (LegacyManagedConfigTomlFromMdmConfigLayerSource) isConfigLayerSource() {}

// ConfigLayerSourceWrapper wraps ConfigLayerSource for JSON marshaling
type ConfigLayerSourceWrapper struct {
	Value ConfigLayerSource
}

func (w *ConfigLayerSourceWrapper) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	typeBytes, ok := obj["type"]
	if !ok {
		return fmt.Errorf("config layer source: missing type key")
	}

	var typeStr string
	if err := json.Unmarshal(typeBytes, &typeStr); err != nil {
		return err
	}

	switch typeStr {
	case "mdm":
		var v MdmConfigLayerSource
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "system":
		var v SystemConfigLayerSource
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "user":
		var v UserConfigLayerSource
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "project":
		var v ProjectConfigLayerSource
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "sessionFlags":
		w.Value = SessionFlagsConfigLayerSource{}
	case "legacyManagedConfigTomlFromFile":
		var v LegacyManagedConfigTomlFromFileConfigLayerSource
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "legacyManagedConfigTomlFromMdm":
		w.Value = LegacyManagedConfigTomlFromMdmConfigLayerSource{}
	default:
		return fmt.Errorf("unknown config layer source type: %s", typeStr)
	}

	return nil
}

func (w ConfigLayerSourceWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil {
		return json.Marshal(nil)
	}

	switch v := w.Value.(type) {
	case MdmConfigLayerSource:
		return json.Marshal(struct {
			Type   string `json:"type"`
			Domain string `json:"domain"`
			Key    string `json:"key"`
		}{
			Type:   "mdm",
			Domain: v.Domain,
			Key:    v.Key,
		})
	case SystemConfigLayerSource:
		return json.Marshal(struct {
			Type string `json:"type"`
			File string `json:"file"`
		}{
			Type: "system",
			File: v.File,
		})
	case UserConfigLayerSource:
		return json.Marshal(struct {
			Type string `json:"type"`
			File string `json:"file"`
		}{
			Type: "user",
			File: v.File,
		})
	case ProjectConfigLayerSource:
		return json.Marshal(struct {
			Type           string `json:"type"`
			DotCodexFolder string `json:"dotCodexFolder"`
		}{
			Type:           "project",
			DotCodexFolder: v.DotCodexFolder,
		})
	case SessionFlagsConfigLayerSource:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{
			Type: "sessionFlags",
		})
	case LegacyManagedConfigTomlFromFileConfigLayerSource:
		return json.Marshal(struct {
			Type string `json:"type"`
			File string `json:"file"`
		}{
			Type: "legacyManagedConfigTomlFromFile",
			File: v.File,
		})
	case LegacyManagedConfigTomlFromMdmConfigLayerSource:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{
			Type: "legacyManagedConfigTomlFromMdm",
		})
	}

	return nil, fmt.Errorf("unknown ConfigLayerSource type: %T", w.Value)
}

// ConfigRequirementsReadResponse represents response from config/requirements/read request
type ConfigRequirementsReadResponse struct {
	Requirements *ConfigRequirements `json:"requirements,omitempty"`
}

// ConfigRequirements represents configuration requirements
type ConfigRequirements struct {
	AllowedApprovalPolicies *[]AskForApprovalWrapper `json:"allowedApprovalPolicies,omitempty"`
	AllowedSandboxModes     *[]SandboxMode           `json:"allowedSandboxModes,omitempty"`
	AllowedWebSearchModes   *[]WebSearchMode         `json:"allowedWebSearchModes,omitempty"`
	EnforceResidency        *ResidencyRequirement    `json:"enforceResidency,omitempty"`
}

// ConfigValueWriteParams represents parameters for config/value/write request
type ConfigValueWriteParams struct {
	KeyPath         string          `json:"keyPath"`
	MergeStrategy   MergeStrategy   `json:"mergeStrategy"`
	Value           json.RawMessage `json:"value"`
	FilePath        *string         `json:"filePath,omitempty"`
	ExpectedVersion *string         `json:"expectedVersion,omitempty"`
}

// ConfigBatchWriteParams represents parameters for config/batch/write request
type ConfigBatchWriteParams struct {
	Edits           []ConfigEdit `json:"edits"`
	FilePath        *string      `json:"filePath,omitempty"`
	ExpectedVersion *string      `json:"expectedVersion,omitempty"`
}

// ConfigEdit represents a single edit in a batch write
type ConfigEdit struct {
	KeyPath       string          `json:"keyPath"`
	MergeStrategy MergeStrategy   `json:"mergeStrategy"`
	Value         json.RawMessage `json:"value"`
}

// ConfigWriteResponse represents response from config write operations
type ConfigWriteResponse struct {
	FilePath           string               `json:"filePath"`
	Status             WriteStatus          `json:"status"`
	Version            string               `json:"version"`
	OverriddenMetadata *OverriddenMetadata  `json:"overriddenMetadata,omitempty"`
}

// OverriddenMetadata represents info when value was overridden by higher layer
type OverriddenMetadata struct {
	EffectiveValue   json.RawMessage     `json:"effectiveValue"`
	Message          string              `json:"message"`
	OverridingLayer  ConfigLayerMetadata `json:"overridingLayer"`
}

// ConfigWarningNotification represents notification/config/warning notification
type ConfigWarningNotification struct {
	Summary string      `json:"summary"`
	Details *string     `json:"details,omitempty"`
	Path    *string     `json:"path,omitempty"`
	Range   *TextRange  `json:"range,omitempty"`
}

// TextRange represents a range in a text file
type TextRange struct {
	Start TextPosition `json:"start"`
	End   TextPosition `json:"end"`
}

// TextPosition represents a position in a text file
type TextPosition struct {
	Line   uint `json:"line"`   // 1-based
	Column uint `json:"column"` // 1-based
}

// ConfigService provides config-related operations
type ConfigService struct {
	client *Client
}

func newConfigService(client *Client) *ConfigService {
	return &ConfigService{client: client}
}

// Read reads the current configuration
func (s *ConfigService) Read(ctx context.Context, params ConfigReadParams) (ConfigReadResponse, error) {
	var resp ConfigReadResponse
	err := s.client.sendRequest(ctx, "config/read", params, &resp)
	return resp, err
}

// ReadRequirements reads configuration requirements
func (s *ConfigService) ReadRequirements(ctx context.Context) (ConfigRequirementsReadResponse, error) {
	var resp ConfigRequirementsReadResponse
	err := s.client.sendRequest(ctx, "configRequirements/read", nil, &resp)
	return resp, err
}

// Write writes a single config value
func (s *ConfigService) Write(ctx context.Context, params ConfigValueWriteParams) (ConfigWriteResponse, error) {
	var resp ConfigWriteResponse
	err := s.client.sendRequest(ctx, "config/value/write", params, &resp)
	return resp, err
}

// BatchWrite writes multiple config values atomically
func (s *ConfigService) BatchWrite(ctx context.Context, params ConfigBatchWriteParams) (ConfigWriteResponse, error) {
	var resp ConfigWriteResponse
	err := s.client.sendRequest(ctx, "config/batchWrite", params, &resp)
	return resp, err
}

// OnConfigWarning registers a listener for config warning notifications
func (c *Client) OnConfigWarning(handler func(ConfigWarningNotification)) {
	c.OnNotification("configWarning", func(ctx context.Context, notif Notification) {
		var n ConfigWarningNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}
