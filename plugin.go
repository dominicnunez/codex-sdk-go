package codex

import (
	"context"
	"encoding/json"
	"errors"
)

// PluginAuthPolicy controls when plugin auth is requested.
type PluginAuthPolicy string

const (
	PluginAuthPolicyOnInstall PluginAuthPolicy = "ON_INSTALL"
	PluginAuthPolicyOnUse     PluginAuthPolicy = "ON_USE"
)

// PluginInstallPolicy controls marketplace install availability.
type PluginInstallPolicy string

const (
	PluginInstallPolicyNotAvailable       PluginInstallPolicy = "NOT_AVAILABLE"
	PluginInstallPolicyAvailable          PluginInstallPolicy = "AVAILABLE"
	PluginInstallPolicyInstalledByDefault PluginInstallPolicy = "INSTALLED_BY_DEFAULT"
)

// MarketplaceInterface contains marketplace display metadata.
type MarketplaceInterface struct {
	DisplayName *string `json:"displayName,omitempty"`
}

// PluginInterface contains plugin UI metadata.
type PluginInterface struct {
	BrandColor        *string  `json:"brandColor,omitempty"`
	Capabilities      []string `json:"capabilities"`
	Category          *string  `json:"category,omitempty"`
	ComposerIcon      *string  `json:"composerIcon,omitempty"`
	DefaultPrompt     []string `json:"defaultPrompt,omitempty"`
	DeveloperName     *string  `json:"developerName,omitempty"`
	DisplayName       *string  `json:"displayName,omitempty"`
	Logo              *string  `json:"logo,omitempty"`
	LongDescription   *string  `json:"longDescription,omitempty"`
	PrivacyPolicyURL  *string  `json:"privacyPolicyUrl,omitempty"`
	Screenshots       []string `json:"screenshots"`
	ShortDescription  *string  `json:"shortDescription,omitempty"`
	TermsOfServiceURL *string  `json:"termsOfServiceUrl,omitempty"`
	WebsiteURL        *string  `json:"websiteUrl,omitempty"`
}

func (p *PluginInterface) UnmarshalJSON(data []byte) error {
	type pluginInterfaceWire struct {
		BrandColor        *string   `json:"brandColor"`
		Capabilities      *[]string `json:"capabilities"`
		Category          *string   `json:"category"`
		ComposerIcon      *string   `json:"composerIcon"`
		DefaultPrompt     []string  `json:"defaultPrompt"`
		DeveloperName     *string   `json:"developerName"`
		DisplayName       *string   `json:"displayName"`
		Logo              *string   `json:"logo"`
		LongDescription   *string   `json:"longDescription"`
		PrivacyPolicyURL  *string   `json:"privacyPolicyUrl"`
		Screenshots       *[]string `json:"screenshots"`
		ShortDescription  *string   `json:"shortDescription"`
		TermsOfServiceURL *string   `json:"termsOfServiceUrl"`
		WebsiteURL        *string   `json:"websiteUrl"`
	}

	var wire pluginInterfaceWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Capabilities == nil {
		return errors.New("missing plugin.interface.capabilities")
	}
	if wire.Screenshots == nil {
		return errors.New("missing plugin.interface.screenshots")
	}

	p.BrandColor = wire.BrandColor
	p.Capabilities = *wire.Capabilities
	p.Category = wire.Category
	p.ComposerIcon = wire.ComposerIcon
	p.DefaultPrompt = wire.DefaultPrompt
	p.DeveloperName = wire.DeveloperName
	p.DisplayName = wire.DisplayName
	p.Logo = wire.Logo
	p.LongDescription = wire.LongDescription
	p.PrivacyPolicyURL = wire.PrivacyPolicyURL
	p.Screenshots = *wire.Screenshots
	p.ShortDescription = wire.ShortDescription
	p.TermsOfServiceURL = wire.TermsOfServiceURL
	p.WebsiteURL = wire.WebsiteURL
	return nil
}

// PluginSource identifies where a plugin came from.
type PluginSource struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (p *PluginSource) UnmarshalJSON(data []byte) error {
	type pluginSourceWire struct {
		Path *string `json:"path"`
		Type *string `json:"type"`
	}

	var wire pluginSourceWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch {
	case wire.Path == nil:
		return errors.New("missing plugin.source.path")
	case wire.Type == nil:
		return errors.New("missing plugin.source.type")
	}

	p.Path = *wire.Path
	p.Type = *wire.Type
	return nil
}

// PluginSummary contains marketplace plugin summary metadata.
type PluginSummary struct {
	AuthPolicy    PluginAuthPolicy    `json:"authPolicy"`
	Enabled       bool                `json:"enabled"`
	ID            string              `json:"id"`
	InstallPolicy PluginInstallPolicy `json:"installPolicy"`
	Installed     bool                `json:"installed"`
	Interface     *PluginInterface    `json:"interface,omitempty"`
	Name          string              `json:"name"`
	Source        PluginSource        `json:"source"`
}

func (p *PluginSummary) UnmarshalJSON(data []byte) error {
	type pluginSummaryWire struct {
		AuthPolicy    *PluginAuthPolicy    `json:"authPolicy"`
		Enabled       *bool                `json:"enabled"`
		ID            *string              `json:"id"`
		InstallPolicy *PluginInstallPolicy `json:"installPolicy"`
		Installed     *bool                `json:"installed"`
		Interface     *PluginInterface     `json:"interface"`
		Name          *string              `json:"name"`
		Source        *PluginSource        `json:"source"`
	}

	var wire pluginSummaryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.AuthPolicy == nil:
		return errors.New("missing plugin.summary.authPolicy")
	case wire.Enabled == nil:
		return errors.New("missing plugin.summary.enabled")
	case wire.ID == nil:
		return errors.New("missing plugin.summary.id")
	case wire.InstallPolicy == nil:
		return errors.New("missing plugin.summary.installPolicy")
	case wire.Installed == nil:
		return errors.New("missing plugin.summary.installed")
	case wire.Name == nil:
		return errors.New("missing plugin.summary.name")
	case wire.Source == nil:
		return errors.New("missing plugin.summary.source")
	}

	p.AuthPolicy = *wire.AuthPolicy
	p.Enabled = *wire.Enabled
	p.ID = *wire.ID
	p.InstallPolicy = *wire.InstallPolicy
	p.Installed = *wire.Installed
	p.Interface = wire.Interface
	p.Name = *wire.Name
	p.Source = *wire.Source
	return nil
}

// PluginMarketplaceEntry groups plugins within a marketplace.
type PluginMarketplaceEntry struct {
	Interface *MarketplaceInterface `json:"interface,omitempty"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Plugins   []PluginSummary       `json:"plugins"`
}

// PluginListParams lists plugins across marketplaces.
type PluginListParams struct {
	Cwds            []string `json:"cwds,omitempty"`
	ForceRemoteSync *bool    `json:"forceRemoteSync,omitempty"`
}

// PluginListResponse contains marketplace plugin listings.
type PluginListResponse struct {
	Marketplaces    []PluginMarketplaceEntry `json:"marketplaces"`
	RemoteSyncError *string                  `json:"remoteSyncError,omitempty"`
}

// PluginReadParams reads a single plugin from a marketplace.
type PluginReadParams struct {
	MarketplacePath string `json:"marketplacePath"`
	PluginName      string `json:"pluginName"`
}

// AppSummary is experimental app metadata included with plugin responses.
type AppSummary struct {
	Description *string `json:"description,omitempty"`
	ID          string  `json:"id"`
	InstallURL  *string `json:"installUrl,omitempty"`
	Name        string  `json:"name"`
}

func (a *AppSummary) UnmarshalJSON(data []byte) error {
	type appSummaryWire struct {
		Description *string `json:"description"`
		ID          *string `json:"id"`
		InstallURL  *string `json:"installUrl"`
		Name        *string `json:"name"`
	}

	var wire appSummaryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch {
	case wire.ID == nil:
		return errors.New("missing plugin.app.id")
	case wire.Name == nil:
		return errors.New("missing plugin.app.name")
	}

	a.Description = wire.Description
	a.ID = *wire.ID
	a.InstallURL = wire.InstallURL
	a.Name = *wire.Name
	return nil
}

// SkillSummary describes a skill bundled with a plugin.
type SkillSummary struct {
	Description      string          `json:"description"`
	Interface        *SkillInterface `json:"interface,omitempty"`
	Name             string          `json:"name"`
	Path             string          `json:"path"`
	ShortDescription *string         `json:"shortDescription,omitempty"`
}

func (s *SkillSummary) UnmarshalJSON(data []byte) error {
	type skillSummaryWire struct {
		Description      *string         `json:"description"`
		Interface        *SkillInterface `json:"interface"`
		Name             *string         `json:"name"`
		Path             *string         `json:"path"`
		ShortDescription *string         `json:"shortDescription"`
	}

	var wire skillSummaryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.Description == nil:
		return errors.New("missing plugin.skill.description")
	case wire.Name == nil:
		return errors.New("missing plugin.skill.name")
	case wire.Path == nil:
		return errors.New("missing plugin.skill.path")
	}

	s.Description = *wire.Description
	s.Interface = wire.Interface
	s.Name = *wire.Name
	s.Path = *wire.Path
	s.ShortDescription = wire.ShortDescription
	return nil
}

// PluginDetail contains full plugin details.
type PluginDetail struct {
	Apps            []AppSummary   `json:"apps"`
	Description     *string        `json:"description,omitempty"`
	MarketplaceName string         `json:"marketplaceName"`
	MarketplacePath string         `json:"marketplacePath"`
	McpServers      []string       `json:"mcpServers"`
	Skills          []SkillSummary `json:"skills"`
	Summary         PluginSummary  `json:"summary"`
}

func (p *PluginDetail) UnmarshalJSON(data []byte) error {
	type pluginDetailWire struct {
		Apps            *[]AppSummary   `json:"apps"`
		Description     *string         `json:"description"`
		MarketplaceName *string         `json:"marketplaceName"`
		MarketplacePath *string         `json:"marketplacePath"`
		McpServers      *[]string       `json:"mcpServers"`
		Skills          *[]SkillSummary `json:"skills"`
		Summary         *PluginSummary  `json:"summary"`
	}

	var wire pluginDetailWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch {
	case wire.Apps == nil:
		return errors.New("missing plugin.apps")
	case wire.MarketplaceName == nil:
		return errors.New("missing plugin.marketplaceName")
	case wire.MarketplacePath == nil:
		return errors.New("missing plugin.marketplacePath")
	case wire.McpServers == nil:
		return errors.New("missing plugin.mcpServers")
	case wire.Skills == nil:
		return errors.New("missing plugin.skills")
	case wire.Summary == nil:
		return errors.New("missing plugin.summary")
	}

	p.Apps = *wire.Apps
	p.Description = wire.Description
	p.MarketplaceName = *wire.MarketplaceName
	p.MarketplacePath = *wire.MarketplacePath
	p.McpServers = *wire.McpServers
	p.Skills = *wire.Skills
	p.Summary = *wire.Summary
	return nil
}

// PluginReadResponse contains a single plugin detail payload.
type PluginReadResponse struct {
	Plugin PluginDetail `json:"plugin"`
}

func (r PluginReadResponse) validate() error {
	if r.Plugin.Summary.ID == "" {
		return errors.New("missing plugin.summary.id")
	}
	return nil
}

// PluginInstallParams installs a plugin from a marketplace.
type PluginInstallParams struct {
	ForceRemoteSync *bool  `json:"forceRemoteSync,omitempty"`
	MarketplacePath string `json:"marketplacePath"`
	PluginName      string `json:"pluginName"`
}

// PluginInstallResponse contains plugin auth follow-up requirements.
type PluginInstallResponse struct {
	AppsNeedingAuth []AppSummary     `json:"appsNeedingAuth"`
	AuthPolicy      PluginAuthPolicy `json:"authPolicy"`
}

func (r PluginInstallResponse) validate() error {
	switch {
	case r.AppsNeedingAuth == nil:
		return errors.New("missing appsNeedingAuth")
	case r.AuthPolicy == "":
		return errors.New("missing authPolicy")
	}
	return nil
}

// PluginUninstallParams removes an installed plugin.
type PluginUninstallParams struct {
	ForceRemoteSync *bool  `json:"forceRemoteSync,omitempty"`
	PluginID        string `json:"pluginId"`
}

// PluginUninstallResponse is the empty response from plugin/uninstall.
type PluginUninstallResponse struct{}

// PluginService provides plugin management operations.
type PluginService struct {
	client *Client
}

func newPluginService(client *Client) *PluginService {
	return &PluginService{client: client}
}

// List lists available plugins across marketplaces.
func (s *PluginService) List(ctx context.Context, params PluginListParams) (PluginListResponse, error) {
	var resp PluginListResponse
	if err := s.client.sendRequest(ctx, methodPluginList, params, &resp); err != nil {
		return PluginListResponse{}, err
	}
	return resp, nil
}

// Read reads a single plugin from a marketplace.
func (s *PluginService) Read(ctx context.Context, params PluginReadParams) (PluginReadResponse, error) {
	var resp PluginReadResponse
	if err := s.client.sendRequest(ctx, methodPluginRead, params, &resp); err != nil {
		return PluginReadResponse{}, err
	}
	if err := resp.validate(); err != nil {
		return PluginReadResponse{}, err
	}
	return resp, nil
}

// Install installs a plugin from a marketplace.
func (s *PluginService) Install(ctx context.Context, params PluginInstallParams) (PluginInstallResponse, error) {
	var resp PluginInstallResponse
	if err := s.client.sendRequest(ctx, methodPluginInstall, params, &resp); err != nil {
		return PluginInstallResponse{}, err
	}
	if err := resp.validate(); err != nil {
		return PluginInstallResponse{}, err
	}
	return resp, nil
}

// Uninstall removes an installed plugin.
func (s *PluginService) Uninstall(ctx context.Context, params PluginUninstallParams) (PluginUninstallResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodPluginUninstall, params); err != nil {
		return PluginUninstallResponse{}, err
	}
	return PluginUninstallResponse{}, nil
}
