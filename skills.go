package codex

import (
	"context"
)

// SkillScope defines the scope of a skill (user, repo, system, admin)
type SkillScope string

const (
	SkillScopeUser   SkillScope = "user"
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeSystem SkillScope = "system"
	SkillScopeAdmin  SkillScope = "admin"
)

// HazelnutScope defines the scope for remote skill queries
type HazelnutScope string

const (
	HazelnutScopeExample          HazelnutScope = "example"
	HazelnutScopeWorkspaceShared  HazelnutScope = "workspace-shared"
	HazelnutScopeAllShared        HazelnutScope = "all-shared"
	HazelnutScopePersonal         HazelnutScope = "personal"
)

// ProductSurface defines the product surface for remote skills
type ProductSurface string

const (
	ProductSurfaceChatGPT ProductSurface = "chatgpt"
	ProductSurfaceCodex   ProductSurface = "codex"
	ProductSurfaceAPI     ProductSurface = "api"
	ProductSurfaceAtlas   ProductSurface = "atlas"
)

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

// SkillDependencies defines dependencies for a skill
type SkillDependencies struct {
	Tools []SkillToolDependency `json:"tools"`
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

// SkillErrorInfo represents an error encountered when loading a skill
type SkillErrorInfo struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SkillsListExtraRootsForCwd specifies extra user roots for a specific cwd
type SkillsListExtraRootsForCwd struct {
	Cwd            string   `json:"cwd"`
	ExtraUserRoots []string `json:"extraUserRoots"`
}

// SkillsListParams defines parameters for listing skills
type SkillsListParams struct {
	Cwds                []string                      `json:"cwds,omitempty"`
	ForceReload         *bool                         `json:"forceReload,omitempty"`
	PerCwdExtraUserRoots []SkillsListExtraRootsForCwd `json:"perCwdExtraUserRoots,omitempty"`
}

// SkillsListEntry represents skills and errors for a single cwd
type SkillsListEntry struct {
	Cwd    string          `json:"cwd"`
	Errors []SkillErrorInfo `json:"errors"`
	Skills []SkillMetadata `json:"skills"`
}

// SkillsListResponse contains the list of skills grouped by cwd
type SkillsListResponse struct {
	Data []SkillsListEntry `json:"data"`
}

// SkillsConfigWriteParams defines parameters for enabling/disabling a skill
type SkillsConfigWriteParams struct {
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// SkillsConfigWriteResponse contains the effective enabled state after write
type SkillsConfigWriteResponse struct {
	EffectiveEnabled bool `json:"effectiveEnabled"`
}

// SkillsRemoteReadParams defines parameters for reading remote skills
type SkillsRemoteReadParams struct {
	Enabled        *bool           `json:"enabled,omitempty"`
	HazelnutScope  *HazelnutScope  `json:"hazelnutScope,omitempty"`
	ProductSurface *ProductSurface `json:"productSurface,omitempty"`
}

// RemoteSkillSummary represents a remote skill available for installation
type RemoteSkillSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SkillsRemoteReadResponse contains the list of remote skills
type SkillsRemoteReadResponse struct {
	Data []RemoteSkillSummary `json:"data"`
}

// SkillsRemoteWriteParams defines parameters for installing a remote skill
type SkillsRemoteWriteParams struct {
	HazelnutID string `json:"hazelnutId"`
}

// SkillsRemoteWriteResponse contains the installed skill's local ID and path
type SkillsRemoteWriteResponse struct {
	ID   string `json:"id"`
	Path string `json:"path"`
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

// RemoteRead retrieves available remote skills from the skill library
func (s *SkillsService) RemoteRead(ctx context.Context, params SkillsRemoteReadParams) (SkillsRemoteReadResponse, error) {
	var resp SkillsRemoteReadResponse
	if err := s.client.sendRequest(ctx, methodSkillsRemoteList, params, &resp); err != nil {
		return SkillsRemoteReadResponse{}, err
	}
	return resp, nil
}

// RemoteWrite installs a remote skill to the local system
func (s *SkillsService) RemoteWrite(ctx context.Context, params SkillsRemoteWriteParams) (SkillsRemoteWriteResponse, error) {
	var resp SkillsRemoteWriteResponse
	if err := s.client.sendRequest(ctx, methodSkillsRemoteExport, params, &resp); err != nil {
		return SkillsRemoteWriteResponse{}, err
	}
	return resp, nil
}
