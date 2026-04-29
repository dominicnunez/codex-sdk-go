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

var validPluginAuthPolicies = map[PluginAuthPolicy]struct{}{
	PluginAuthPolicyOnInstall: {},
	PluginAuthPolicyOnUse:     {},
}

func validatePluginAuthPolicyField(field string, value PluginAuthPolicy) error {
	return validateEnumValue(field, value, validPluginAuthPolicies)
}

// PluginInstallPolicy controls marketplace install availability.
type PluginInstallPolicy string

const (
	PluginInstallPolicyNotAvailable       PluginInstallPolicy = "NOT_AVAILABLE"
	PluginInstallPolicyAvailable          PluginInstallPolicy = "AVAILABLE"
	PluginInstallPolicyInstalledByDefault PluginInstallPolicy = "INSTALLED_BY_DEFAULT"
)

var validPluginInstallPolicies = map[PluginInstallPolicy]struct{}{
	PluginInstallPolicyNotAvailable:       {},
	PluginInstallPolicyAvailable:          {},
	PluginInstallPolicyInstalledByDefault: {},
}

func validatePluginInstallPolicyField(field string, value PluginInstallPolicy) error {
	return validateEnumValue(field, value, validPluginInstallPolicies)
}

const pluginSourceTypeLocal = "local"

var validPluginSourceTypes = map[string]struct{}{
	pluginSourceTypeLocal: {},
}

func validatePluginSourceTypeField(field string, value string) error {
	return validateStringEnumValue(field, value, validPluginSourceTypes)
}

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
	validatedComposerIcon, err := validateInboundAbsolutePathPointerField("plugin.interface.composerIcon", wire.ComposerIcon)
	if err != nil {
		return err
	}
	p.ComposerIcon = validatedComposerIcon
	p.DefaultPrompt = wire.DefaultPrompt
	p.DeveloperName = wire.DeveloperName
	p.DisplayName = wire.DisplayName
	validatedLogo, err := validateInboundAbsolutePathPointerField("plugin.interface.logo", wire.Logo)
	if err != nil {
		return err
	}
	p.Logo = validatedLogo
	p.LongDescription = wire.LongDescription
	p.PrivacyPolicyURL = wire.PrivacyPolicyURL
	validatedScreenshots, err := validateInboundAbsolutePathSliceField("plugin.interface.screenshots", *wire.Screenshots)
	if err != nil {
		return err
	}
	p.Screenshots = validatedScreenshots
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

	validatedPath, err := validateInboundAbsolutePathField("plugin.source.path", *wire.Path)
	if err != nil {
		return err
	}
	p.Path = validatedPath
	p.Type = *wire.Type
	if err := validatePluginSourceTypeField("plugin.source.type", p.Type); err != nil {
		return err
	}
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
	if err := validatePluginAuthPolicyField("plugin.summary.authPolicy", p.AuthPolicy); err != nil {
		return err
	}
	if err := validatePluginInstallPolicyField("plugin.summary.installPolicy", p.InstallPolicy); err != nil {
		return err
	}
	return nil
}

// PluginMarketplaceEntry groups plugins within a marketplace.
type PluginMarketplaceEntry struct {
	Interface *MarketplaceInterface `json:"interface,omitempty"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Plugins   []PluginSummary       `json:"plugins"`
}

func (p *PluginMarketplaceEntry) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "name", "path", "plugins"); err != nil {
		return err
	}
	type wire PluginMarketplaceEntry
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedPath, err := validateInboundAbsolutePathField("plugin.marketplace.path", decoded.Path)
	if err != nil {
		return err
	}
	decoded.Path = validatedPath
	*p = PluginMarketplaceEntry(decoded)
	return nil
}

// PluginListParams lists plugins across marketplaces.
type PluginListParams struct {
	Cwds            []string `json:"cwds,omitempty"`
	ForceRemoteSync *bool    `json:"forceRemoteSync,omitempty"`
}

// PluginListResponse contains marketplace plugin listings.
type PluginListResponse struct {
	FeaturedPluginIDs     []string                   `json:"featuredPluginIds,omitempty"`
	MarketplaceLoadErrors []MarketplaceLoadErrorInfo `json:"marketplaceLoadErrors,omitempty"`
	Marketplaces          []PluginMarketplaceEntry   `json:"marketplaces"`
	RemoteSyncError       *string                    `json:"remoteSyncError,omitempty"`
}

// MarketplaceLoadErrorInfo describes a marketplace loading failure.
type MarketplaceLoadErrorInfo struct {
	MarketplacePath string `json:"marketplacePath"`
	Message         string `json:"message"`
}

func (r *PluginListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "marketplaces"); err != nil {
		return err
	}
	type wire PluginListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = PluginListResponse(decoded)
	return nil
}

// PluginReadParams reads a single plugin from a marketplace.
type PluginReadParams struct {
	MarketplacePath       string  `json:"marketplacePath,omitempty"`
	PluginName            string  `json:"pluginName"`
	RemoteMarketplaceName *string `json:"remoteMarketplaceName,omitempty"`
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
	validatedPath, err := validateInboundAbsolutePathField("plugin.skill.path", *wire.Path)
	if err != nil {
		return err
	}
	s.Path = validatedPath
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
	validatedMarketplacePath, err := validateInboundAbsolutePathField("plugin.marketplacePath", *wire.MarketplacePath)
	if err != nil {
		return err
	}
	p.MarketplacePath = validatedMarketplacePath
	p.McpServers = *wire.McpServers
	p.Skills = *wire.Skills
	p.Summary = *wire.Summary
	return nil
}

// PluginReadResponse contains a single plugin detail payload.
type PluginReadResponse struct {
	Plugin PluginDetail `json:"plugin"`
}

func (r *PluginReadResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "plugin"); err != nil {
		return err
	}
	type wire PluginReadResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = PluginReadResponse(decoded)
	return nil
}

func (r PluginReadResponse) validate() error {
	if r.Plugin.Summary.ID == "" {
		return errors.New("missing plugin.summary.id")
	}
	return nil
}

// PluginInstallParams installs a plugin from a marketplace.
type PluginInstallParams struct {
	ForceRemoteSync       *bool   `json:"forceRemoteSync,omitempty"`
	MarketplacePath       string  `json:"marketplacePath,omitempty"`
	PluginName            string  `json:"pluginName"`
	RemoteMarketplaceName *string `json:"remoteMarketplaceName,omitempty"`
}

// PluginInstallResponse contains plugin auth follow-up requirements.
type PluginInstallResponse struct {
	AppsNeedingAuth []AppSummary     `json:"appsNeedingAuth"`
	AuthPolicy      PluginAuthPolicy `json:"authPolicy"`
}

func (r *PluginInstallResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "appsNeedingAuth", "authPolicy"); err != nil {
		return err
	}
	type wire PluginInstallResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validatePluginAuthPolicyField("plugin.install.authPolicy", decoded.AuthPolicy); err != nil {
		return err
	}
	*r = PluginInstallResponse(decoded)
	return nil
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
	return resp, nil
}

// Install installs a plugin from a marketplace.
func (s *PluginService) Install(ctx context.Context, params PluginInstallParams) (PluginInstallResponse, error) {
	var resp PluginInstallResponse
	if err := s.client.sendRequest(ctx, methodPluginInstall, params, &resp); err != nil {
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
