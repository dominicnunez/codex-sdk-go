package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// SkillScope defines the scope of a skill (user, repo, system, admin)
type SkillScope string

const (
	SkillScopeUser   SkillScope = "user"
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeSystem SkillScope = "system"
	SkillScopeAdmin  SkillScope = "admin"
)

var validSkillScopes = map[SkillScope]struct{}{
	SkillScopeUser:   {},
	SkillScopeRepo:   {},
	SkillScopeSystem: {},
	SkillScopeAdmin:  {},
}

func (s *SkillScope) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "skill.scope", validSkillScopes, s)
}

// SkillInterface defines optional UI metadata for a skill
type SkillInterface struct {
	DisplayName      *string `json:"displayName,omitempty"`
	ShortDescription *string `json:"shortDescription,omitempty"`
	DefaultPrompt    *string `json:"defaultPrompt,omitempty"`
	BrandColor       *string `json:"brandColor,omitempty"`
	IconSmall        *string `json:"iconSmall,omitempty"`
	IconLarge        *string `json:"iconLarge,omitempty"`
}

// SkillToolDependency represents a tool that a skill depends on
type SkillToolDependency struct {
	Type        string  `json:"type"`
	Value       string  `json:"value"`
	Command     *string `json:"command,omitempty"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
	Transport   *string `json:"transport,omitempty"`
}

func (d *SkillToolDependency) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type", "value"); err != nil {
		return err
	}
	type wire SkillToolDependency
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*d = SkillToolDependency(decoded)
	return nil
}

// SkillDependencies defines dependencies for a skill
type SkillDependencies struct {
	Tools []SkillToolDependency `json:"tools"`
}

func (d *SkillDependencies) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "tools"); err != nil {
		return err
	}
	type wire SkillDependencies
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*d = SkillDependencies(decoded)
	return nil
}

// SkillMetadata represents metadata for a single skill
type SkillMetadata struct {
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Path             string             `json:"path"`
	Enabled          bool               `json:"enabled"`
	Scope            SkillScope         `json:"scope"`
	Dependencies     *SkillDependencies `json:"dependencies,omitempty"`
	Interface        *SkillInterface    `json:"interface,omitempty"`
	ShortDescription *string            `json:"shortDescription,omitempty"`
}

func (m *SkillMetadata) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "description", "enabled", "name", "path", "scope"); err != nil {
		return err
	}
	type wire SkillMetadata
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedPath, err := validateInboundAbsolutePathField("skill.path", decoded.Path)
	if err != nil {
		return err
	}
	decoded.Path = validatedPath
	*m = SkillMetadata(decoded)
	return nil
}

// SkillErrorInfo represents an error encountered when loading a skill
type SkillErrorInfo struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (e *SkillErrorInfo) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "message", "path"); err != nil {
		return err
	}
	type wire SkillErrorInfo
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedPath, err := validateInboundAbsolutePathField("skill.error.path", decoded.Path)
	if err != nil {
		return err
	}
	decoded.Path = validatedPath
	*e = SkillErrorInfo(decoded)
	return nil
}

// SkillsListExtraRootsForCwd specifies extra user roots for a specific cwd
type SkillsListExtraRootsForCwd struct {
	Cwd            string   `json:"cwd"`
	ExtraUserRoots []string `json:"extraUserRoots"`
}

// SkillsListParams defines parameters for listing skills
type SkillsListParams struct {
	Cwds                 []string                     `json:"cwds,omitempty"`
	ForceReload          *bool                        `json:"forceReload,omitempty"`
	PerCwdExtraUserRoots []SkillsListExtraRootsForCwd `json:"perCwdExtraUserRoots,omitempty"`
}

func (p SkillsListParams) prepareRequest() (interface{}, error) {
	var err error
	p.Cwds, err = normalizeAbsolutePathSliceField("cwds", p.Cwds)
	if err != nil {
		return nil, err
	}

	for i := range p.PerCwdExtraUserRoots {
		p.PerCwdExtraUserRoots[i].Cwd, err = normalizeAbsolutePathField(
			fmt.Sprintf("perCwdExtraUserRoots[%d].cwd", i),
			p.PerCwdExtraUserRoots[i].Cwd,
		)
		if err != nil {
			return nil, err
		}
		p.PerCwdExtraUserRoots[i].ExtraUserRoots, err = normalizeAbsolutePathSliceField(
			fmt.Sprintf("perCwdExtraUserRoots[%d].extraUserRoots", i),
			p.PerCwdExtraUserRoots[i].ExtraUserRoots,
		)
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

// SkillsListEntry represents skills and errors for a single cwd
type SkillsListEntry struct {
	Cwd    string           `json:"cwd"`
	Errors []SkillErrorInfo `json:"errors"`
	Skills []SkillMetadata  `json:"skills"`
}

func (e *SkillsListEntry) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "cwd", "errors", "skills"); err != nil {
		return err
	}
	type wire SkillsListEntry
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("skills.cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	*e = SkillsListEntry(decoded)
	return nil
}

// SkillsListResponse contains the list of skills grouped by cwd
type SkillsListResponse struct {
	Data []SkillsListEntry `json:"data"`
}

func (r *SkillsListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire SkillsListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = SkillsListResponse(decoded)
	return nil
}

// SkillsConfigWriteParams defines parameters for enabling/disabling a skill
type SkillsConfigWriteParams struct {
	Enabled bool    `json:"enabled"`
	Name    *string `json:"name,omitempty"`
	Path    string  `json:"path,omitempty"`
}

func (p SkillsConfigWriteParams) prepareRequest() (interface{}, error) {
	if p.Path == "" {
		return p, nil
	}
	var err error
	p.Path, err = normalizeAbsolutePathField("path", p.Path)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// SkillsConfigWriteResponse contains the effective enabled state after write
type SkillsConfigWriteResponse struct {
	EffectiveEnabled bool `json:"effectiveEnabled"`
}

func (r *SkillsConfigWriteResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "effectiveEnabled"); err != nil {
		return err
	}
	type wire SkillsConfigWriteResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = SkillsConfigWriteResponse(decoded)
	return nil
}

// SkillsService provides access to skills-related operations
type SkillsService struct {
	client *Client
}

func newSkillsService(c *Client) *SkillsService {
	return &SkillsService{client: c}
}

// List retrieves all skills for the specified working directories
func (s *SkillsService) List(ctx context.Context, params SkillsListParams) (SkillsListResponse, error) {
	var resp SkillsListResponse
	if err := s.client.sendRequest(ctx, methodSkillsList, params, &resp); err != nil {
		return SkillsListResponse{}, err
	}
	return resp, nil
}

// ConfigWrite enables or disables a skill
func (s *SkillsService) ConfigWrite(ctx context.Context, params SkillsConfigWriteParams) (SkillsConfigWriteResponse, error) {
	var resp SkillsConfigWriteResponse
	if err := s.client.sendRequest(ctx, methodSkillsConfigWrite, params, &resp); err != nil {
		return SkillsConfigWriteResponse{}, err
	}
	return resp, nil
}

// SkillsChangedNotification is emitted when local skill files change.
type SkillsChangedNotification struct{}

// OnSkillsChanged registers a listener for skills invalidation notifications.
func (c *Client) OnSkillsChanged(handler func(SkillsChangedNotification)) {
	if handler == nil {
		c.OnNotification(notifySkillsChanged, nil)
		return
	}
	c.OnNotification(notifySkillsChanged, func(ctx context.Context, notif Notification) {
		var params SkillsChangedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifySkillsChanged, fmt.Errorf("unmarshal %s: %w", notifySkillsChanged, err))
			return
		}
		handler(params)
	})
}
