package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// AppsConfig holds global defaults and settings keyed by connector ID.
// Apps are flattened alongside _default in the upstream wire representation.
type AppsConfig struct {
	Default *AppsDefaultConfig   `json:"_default,omitempty"`
	Apps    map[string]AppConfig `json:"-"`
}

func (c *AppsConfig) UnmarshalJSON(data []byte) error {
	if err := validateInboundObjectFields(data, nil, nil); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := jsondecode.Unmarshal(data, &fields); err != nil {
		return err
	}
	decoded := AppsConfig{Apps: make(map[string]AppConfig)}
	for key, raw := range fields {
		if key == "_default" {
			if err := jsondecode.Unmarshal(raw, &decoded.Default); err != nil {
				return fmt.Errorf("apps defaults: %w", err)
			}
			continue
		}
		var app AppConfig
		if err := jsondecode.Unmarshal(raw, &app); err != nil {
			return fmt.Errorf("app %s: %w", quotedValueDiagnostic(key), err)
		}
		decoded.Apps[key] = app
	}
	*c = decoded
	return nil
}

func (c AppsConfig) MarshalJSON() ([]byte, error) {
	fields := make(map[string]interface{}, len(c.Apps)+1)
	for key, app := range c.Apps {
		if key == "_default" {
			return nil, fmt.Errorf("app ID _default is reserved for apps defaults")
		}
		fields[key] = app
	}
	if c.Default != nil {
		fields["_default"] = c.Default
	}
	return json.Marshal(fields)
}

// AppsDefaultConfig contains default connector permissions returned by config/read.
type AppsDefaultConfig struct {
	Enabled                  bool               `json:"enabled"`
	ApprovalsReviewer        *ApprovalsReviewer `json:"approvals_reviewer,omitempty"`
	DestructiveEnabled       bool               `json:"destructive_enabled"`
	OpenWorldEnabled         bool               `json:"open_world_enabled"`
	DefaultToolsApprovalMode *AppToolApproval   `json:"default_tools_approval_mode,omitempty"`
}

func (c *AppsDefaultConfig) UnmarshalJSON(data []byte) error {
	type wire AppsDefaultConfig
	decoded := wire{Enabled: true, DestructiveEnabled: true, OpenWorldEnabled: true}
	if err := unmarshalInboundObject(data, &decoded, nil, []string{"enabled", "destructive_enabled", "open_world_enabled"}); err != nil {
		return err
	}
	*c = AppsDefaultConfig(decoded)
	return nil
}

// AppConfig contains settings for one connector. Optional booleans retain explicit false values.
type AppConfig struct {
	Enabled                  bool                   `json:"enabled"`
	OmitToolsFrom            *[]ToolExposureSurface `json:"omit_tools_from,omitempty"`
	ApprovalsReviewer        *ApprovalsReviewer     `json:"approvals_reviewer,omitempty"`
	DestructiveEnabled       *bool                  `json:"destructive_enabled,omitempty"`
	OpenWorldEnabled         *bool                  `json:"open_world_enabled,omitempty"`
	DefaultToolsApprovalMode *AppToolApproval       `json:"default_tools_approval_mode,omitempty"`
	DefaultToolsEnabled      *bool                  `json:"default_tools_enabled,omitempty"`
	Tools                    *AppToolsConfig        `json:"tools,omitempty"`
	Links                    *AppLinksConfig        `json:"links,omitempty"`
}

func (c *AppConfig) UnmarshalJSON(data []byte) error {
	type wire AppConfig
	decoded := wire{Enabled: true}
	if err := unmarshalInboundObject(data, &decoded, nil, []string{"enabled"}); err != nil {
		return err
	}
	*c = AppConfig(decoded)
	return nil
}

// AppToolsConfig is flattened on the wire: each key is a tool name.
type AppToolsConfig map[string]AppToolConfig

func (c *AppToolsConfig) UnmarshalJSON(data []byte) error {
	if err := validateInboundObjectFields(data, nil, nil); err != nil {
		return err
	}
	type wire AppToolsConfig
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = AppToolsConfig(decoded)
	return nil
}

// AppToolConfig contains the enabled and approval settings for one tool.
type AppToolConfig struct {
	Enabled      *bool            `json:"enabled,omitempty"`
	ApprovalMode *AppToolApproval `json:"approval_mode,omitempty"`
}

func (c *AppToolConfig) UnmarshalJSON(data []byte) error {
	type wire AppToolConfig
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	*c = AppToolConfig(decoded)
	return nil
}

// AppLinksConfig is flattened on the wire: each key is a connected account's link ID.
type AppLinksConfig map[string]AppLinkConfig

func (c *AppLinksConfig) UnmarshalJSON(data []byte) error {
	if err := validateInboundObjectFields(data, nil, nil); err != nil {
		return err
	}
	type wire AppLinksConfig
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = AppLinksConfig(decoded)
	return nil
}

// AppLinkConfig contains approval settings for one connected account.
type AppLinkConfig struct {
	ApprovalsReviewer        *ApprovalsReviewer `json:"approvals_reviewer,omitempty"`
	DefaultToolsApprovalMode *AppToolApproval   `json:"default_tools_approval_mode,omitempty"`
}

func (c *AppLinkConfig) UnmarshalJSON(data []byte) error {
	type wire AppLinkConfig
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	*c = AppLinkConfig(decoded)
	return nil
}

var validAppToolApprovals = map[AppToolApproval]struct{}{
	AppToolApprovalAuto: {}, AppToolApprovalPrompt: {}, AppToolApprovalWrites: {}, AppToolApprovalApprove: {},
}

func (v AppToolApproval) MarshalJSON() ([]byte, error) {
	return marshalEnumString("AppToolApproval", v, validAppToolApprovals)
}

func (v *AppToolApproval) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "AppToolApproval", validAppToolApprovals, v)
}

// ToolExposureSurface identifies a model-facing surface for connector tools.
type ToolExposureSurface string

const (
	ToolExposureSurfaceCodeMode ToolExposureSurface = "code_mode"
	ToolExposureSurfaceDeferred ToolExposureSurface = "deferred"
	ToolExposureSurfaceDirect   ToolExposureSurface = "direct"
)

var validToolExposureSurfaces = map[ToolExposureSurface]struct{}{
	ToolExposureSurfaceCodeMode: {}, ToolExposureSurfaceDeferred: {}, ToolExposureSurfaceDirect: {},
}

func (v ToolExposureSurface) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ToolExposureSurface", v, validToolExposureSurfaces)
}

func (v *ToolExposureSurface) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ToolExposureSurface", validToolExposureSurfaces, v)
}
