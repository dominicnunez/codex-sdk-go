package codex

import "context"

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

// PluginSource identifies where a plugin came from.
type PluginSource struct {
	Path string `json:"path"`
	Type string `json:"type"`
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

// SkillSummary describes a skill bundled with a plugin.
type SkillSummary struct {
	Description      string          `json:"description"`
	Interface        *SkillInterface `json:"interface,omitempty"`
	Name             string          `json:"name"`
	Path             string          `json:"path"`
	ShortDescription *string         `json:"shortDescription,omitempty"`
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

// PluginReadResponse contains a single plugin detail payload.
type PluginReadResponse struct {
	Plugin PluginDetail `json:"plugin"`
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
	if err := s.client.sendRequest(ctx, methodPluginUninstall, params, nil); err != nil {
		return PluginUninstallResponse{}, err
	}
	return PluginUninstallResponse{}, nil
}
