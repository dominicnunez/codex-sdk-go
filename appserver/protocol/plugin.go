package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
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

// PluginAvailability describes whether a remote plugin is available to install/use.
type PluginAvailability string

const (
	PluginAvailabilityDisabledByAdmin PluginAvailability = "DISABLED_BY_ADMIN"
	PluginAvailabilityAvailable       PluginAvailability = "AVAILABLE"
)

var validPluginAvailabilities = map[PluginAvailability]struct{}{
	PluginAvailabilityDisabledByAdmin: {},
	PluginAvailabilityAvailable:       {},
}

func validateOptionalPluginAvailabilityField(field string, value *PluginAvailability) error {
	return validateOptionalEnumValue(field, value, validPluginAvailabilities)
}

// PluginDisabledReason explains why a remote plugin is unavailable.
type PluginDisabledReason string

const (
	PluginDisabledReasonDisabledByAdmin        PluginDisabledReason = "disabled_by_admin"
	PluginDisabledReasonPlanNotEligible        PluginDisabledReason = "plan_not_eligible"
	PluginDisabledReasonRequiredAppUnavailable PluginDisabledReason = "required_app_unavailable"
	PluginDisabledReasonUnknown                PluginDisabledReason = "unknown"
)

var validPluginDisabledReasons = map[PluginDisabledReason]struct{}{
	PluginDisabledReasonDisabledByAdmin: {}, PluginDisabledReasonPlanNotEligible: {},
	PluginDisabledReasonRequiredAppUnavailable: {}, PluginDisabledReasonUnknown: {},
}

func (r *PluginDisabledReason) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "plugin.disabledReason", validPluginDisabledReasons, r)
}

func (r PluginDisabledReason) MarshalJSON() ([]byte, error) {
	return marshalEnumString("plugin.disabledReason", r, validPluginDisabledReasons)
}

// PluginInstallPolicySource identifies the source of a plugin install policy.
type PluginInstallPolicySource string

const (
	PluginInstallPolicySourceWorkspaceSetting     PluginInstallPolicySource = "WORKSPACE_SETTING"
	PluginInstallPolicySourceImplicitCanonicalApp PluginInstallPolicySource = "IMPLICIT_CANONICAL_APP"
)

var validPluginInstallPolicySources = map[PluginInstallPolicySource]struct{}{
	PluginInstallPolicySourceWorkspaceSetting: {}, PluginInstallPolicySourceImplicitCanonicalApp: {},
}

func (s *PluginInstallPolicySource) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "plugin.installPolicySource", validPluginInstallPolicySources, s)
}

func (s PluginInstallPolicySource) MarshalJSON() ([]byte, error) {
	return marshalEnumString("plugin.installPolicySource", s, validPluginInstallPolicySources)
}

// PluginListMarketplaceKind filters plugin/list marketplaces.
type PluginListMarketplaceKind string

const (
	PluginListMarketplaceKindLocal              PluginListMarketplaceKind = "local"
	PluginListMarketplaceKindWorkspaceDirectory PluginListMarketplaceKind = "workspace-directory"
	PluginListMarketplaceKindSharedWithMe       PluginListMarketplaceKind = "shared-with-me"
	PluginListMarketplaceKindCreatedByMeRemote  PluginListMarketplaceKind = "created-by-me-remote"
)

var validPluginListMarketplaceKinds = map[PluginListMarketplaceKind]struct{}{
	PluginListMarketplaceKindLocal:              {},
	PluginListMarketplaceKindWorkspaceDirectory: {},
	PluginListMarketplaceKindSharedWithMe:       {},
	PluginListMarketplaceKindCreatedByMeRemote:  {},
}

const (
	pluginSourceTypeLocal  = "local"
	pluginSourceTypeGit    = "git"
	pluginSourceTypeRemote = "remote"
)

var validPluginSourceTypes = map[string]struct{}{
	pluginSourceTypeLocal:  {},
	pluginSourceTypeGit:    {},
	pluginSourceTypeRemote: {},
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
	ComposerIconURL   *string  `json:"composerIconUrl,omitempty"`
	DefaultPrompt     []string `json:"defaultPrompt,omitempty"`
	DeveloperName     *string  `json:"developerName,omitempty"`
	DisplayName       *string  `json:"displayName,omitempty"`
	Logo              *string  `json:"logo,omitempty"`
	LogoDark          *string  `json:"logoDark,omitempty"`
	LogoURL           *string  `json:"logoUrl,omitempty"`
	LogoURLDark       *string  `json:"logoUrlDark,omitempty"`
	LongDescription   *string  `json:"longDescription,omitempty"`
	PrivacyPolicyURL  *string  `json:"privacyPolicyUrl,omitempty"`
	ScreenshotURLs    []string `json:"screenshotUrls"`
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
		ComposerIconURL   *string   `json:"composerIconUrl"`
		DefaultPrompt     []string  `json:"defaultPrompt"`
		DeveloperName     *string   `json:"developerName"`
		DisplayName       *string   `json:"displayName"`
		Logo              *string   `json:"logo"`
		LogoDark          *string   `json:"logoDark"`
		LogoURL           *string   `json:"logoUrl"`
		LogoURLDark       *string   `json:"logoUrlDark"`
		LongDescription   *string   `json:"longDescription"`
		PrivacyPolicyURL  *string   `json:"privacyPolicyUrl"`
		ScreenshotURLs    *[]string `json:"screenshotUrls"`
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
	if wire.ScreenshotURLs == nil {
		return errors.New("missing plugin.interface.screenshotUrls")
	}
	if wire.Screenshots == nil {
		return errors.New("missing plugin.interface.screenshots")
	}

	p.BrandColor = wire.BrandColor
	p.Capabilities = *wire.Capabilities
	p.Category = wire.Category
	p.ComposerIconURL = wire.ComposerIconURL
	validatedComposerIcon, err := validateInboundAbsolutePathPointerField("plugin.interface.composerIcon", wire.ComposerIcon)
	if err != nil {
		return err
	}
	p.ComposerIcon = validatedComposerIcon
	p.DefaultPrompt = wire.DefaultPrompt
	p.DeveloperName = wire.DeveloperName
	p.DisplayName = wire.DisplayName
	p.LogoURL = wire.LogoURL
	validatedLogo, err := validateInboundAbsolutePathPointerField("plugin.interface.logo", wire.Logo)
	if err != nil {
		return err
	}
	p.Logo = validatedLogo
	validatedLogoDark, err := validateInboundAbsolutePathPointerField("plugin.interface.logoDark", wire.LogoDark)
	if err != nil {
		return err
	}
	p.LogoDark = validatedLogoDark
	p.LogoURLDark = wire.LogoURLDark
	p.LongDescription = wire.LongDescription
	p.PrivacyPolicyURL = wire.PrivacyPolicyURL
	p.ScreenshotURLs = *wire.ScreenshotURLs
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
	Path    *string `json:"path,omitempty"`
	RefName *string `json:"refName,omitempty"`
	SHA     *string `json:"sha,omitempty"`
	Type    string  `json:"type"`
	URL     *string `json:"url,omitempty"`
}

func (p *PluginSource) UnmarshalJSON(data []byte) error {
	type pluginSourceWire struct {
		Path    *string `json:"path"`
		RefName *string `json:"refName"`
		SHA     *string `json:"sha"`
		Type    *string `json:"type"`
		URL     *string `json:"url"`
	}

	var wire pluginSourceWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Type == nil {
		return errors.New("missing plugin.source.type")
	}
	if err := validatePluginSourceTypeField("plugin.source.type", *wire.Type); err != nil {
		return err
	}

	if *wire.Type == pluginSourceTypeLocal {
		if wire.Path == nil {
			return errors.New("missing plugin.source.path")
		}
		validatedPath, err := validateInboundAbsolutePathField("plugin.source.path", *wire.Path)
		if err != nil {
			return err
		}
		wire.Path = &validatedPath
	}
	if *wire.Type == pluginSourceTypeGit && wire.URL == nil {
		return errors.New("missing plugin.source.url")
	}
	p.Path = wire.Path
	p.RefName = wire.RefName
	p.SHA = wire.SHA
	p.Type = *wire.Type
	p.URL = wire.URL
	return nil
}

// PluginSummary contains marketplace plugin summary metadata.
type PluginSummary struct {
	DisabledReason                   *PluginDisabledReason      `json:"disabledReason,omitempty"`
	EligiblePlanTypes                OptionalNullable[[]string] `json:"eligiblePlanTypes,omitzero"`
	InstallPolicySource              *PluginInstallPolicySource `json:"installPolicySource,omitempty"`
	InstalledAt                      *int64                     `json:"installedAt,omitempty"`
	MustShowInstallationInterstitial *bool                      `json:"mustShowInstallationInterstitial,omitempty"`
	Version                          *string                    `json:"version,omitempty"`
	AuthPolicy                       PluginAuthPolicy           `json:"authPolicy"`
	Availability                     *PluginAvailability        `json:"availability,omitempty"`
	Enabled                          bool                       `json:"enabled"`
	ID                               string                     `json:"id"`
	InstallPolicy                    PluginInstallPolicy        `json:"installPolicy"`
	Installed                        bool                       `json:"installed"`
	Interface                        *PluginInterface           `json:"interface,omitempty"`
	Keywords                         []string                   `json:"keywords,omitempty"`
	LocalVersion                     *string                    `json:"localVersion,omitempty"`
	Name                             string                     `json:"name"`
	RemotePluginID                   *string                    `json:"remotePluginId,omitempty"`
	ShareContext                     *PluginShareContext        `json:"shareContext,omitempty"`
	Source                           PluginSource               `json:"source"`
}

func (p *PluginSummary) UnmarshalJSON(data []byte) error {
	type pluginSummaryWire struct {
		DisabledReason                   *PluginDisabledReason      `json:"disabledReason"`
		EligiblePlanTypes                OptionalNullable[[]string] `json:"eligiblePlanTypes"`
		InstallPolicySource              *PluginInstallPolicySource `json:"installPolicySource"`
		InstalledAt                      *int64                     `json:"installedAt"`
		MustShowInstallationInterstitial *bool                      `json:"mustShowInstallationInterstitial"`
		Version                          *string                    `json:"version"`
		AuthPolicy                       *PluginAuthPolicy          `json:"authPolicy"`
		Availability                     *PluginAvailability        `json:"availability"`
		Enabled                          *bool                      `json:"enabled"`
		ID                               *string                    `json:"id"`
		InstallPolicy                    *PluginInstallPolicy       `json:"installPolicy"`
		Installed                        *bool                      `json:"installed"`
		Interface                        *PluginInterface           `json:"interface"`
		Keywords                         []string                   `json:"keywords"`
		LocalVersion                     *string                    `json:"localVersion"`
		Name                             *string                    `json:"name"`
		RemotePluginID                   *string                    `json:"remotePluginId"`
		ShareContext                     *PluginShareContext        `json:"shareContext"`
		Source                           *PluginSource              `json:"source"`
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
	previous := *p
	p.AuthPolicy = *wire.AuthPolicy
	p.DisabledReason = wire.DisabledReason
	p.EligiblePlanTypes = wire.EligiblePlanTypes
	p.InstallPolicySource = wire.InstallPolicySource
	p.InstalledAt = wire.InstalledAt
	p.MustShowInstallationInterstitial = wire.MustShowInstallationInterstitial
	p.Version = wire.Version
	p.Availability = wire.Availability
	p.Enabled = *wire.Enabled
	p.ID = *wire.ID
	p.InstallPolicy = *wire.InstallPolicy
	p.Installed = *wire.Installed
	p.Interface = wire.Interface
	p.Keywords = wire.Keywords
	p.LocalVersion = wire.LocalVersion
	p.Name = *wire.Name
	p.RemotePluginID = wire.RemotePluginID
	p.ShareContext = wire.ShareContext
	p.Source = *wire.Source
	if err := validatePluginAuthPolicyField("plugin.summary.authPolicy", p.AuthPolicy); err != nil {
		return err
	}
	if err := validatePluginInstallPolicyField("plugin.summary.installPolicy", p.InstallPolicy); err != nil {
		return err
	}
	if err := validateOptionalPluginAvailabilityField("plugin.summary.availability", p.Availability); err != nil {
		return err
	}
	if err := validateOptionalStringArrays(data, "keywords"); err != nil {
		// Established semantic errors retain their original partial updates;
		// only this new admission failure restores the previous receiver.
		*p = previous
		return err
	}
	return nil
}

// PluginMarketplaceEntry groups plugins within a marketplace.
type PluginMarketplaceEntry struct {
	Interface *MarketplaceInterface `json:"interface,omitempty"`
	Name      string                `json:"name"`
	Path      *string               `json:"path,omitempty"`
	Plugins   []PluginSummary       `json:"plugins"`
}

func (p *PluginMarketplaceEntry) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "name", "plugins"); err != nil {
		return err
	}
	type wire PluginMarketplaceEntry
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedPath, err := validateInboundAbsolutePathPointerField("plugin.marketplace.path", decoded.Path)
	if err != nil {
		return err
	}
	decoded.Path = validatedPath
	*p = PluginMarketplaceEntry(decoded)
	return nil
}

// PluginListParams lists plugins across marketplaces.
type PluginListParams struct {
	Cwds             []string                    `json:"cwds,omitempty"`
	MarketplaceKinds []PluginListMarketplaceKind `json:"marketplaceKinds,omitempty"`
	ForceRefetch     bool                        `json:"forceRefetch,omitempty"`
}

// PluginInstalledParams lists installed plugins across marketplaces.
type PluginInstalledParams struct {
	Cwds                         []string `json:"cwds,omitempty"`
	InstallSuggestionPluginNames []string `json:"installSuggestionPluginNames,omitempty"`
}

// PluginListResponse contains marketplace plugin listings.
type PluginListResponse struct {
	FeaturedPluginIDs     []string                   `json:"featuredPluginIds,omitempty"`
	MarketplaceLoadErrors []MarketplaceLoadErrorInfo `json:"marketplaceLoadErrors,omitempty"`
	Marketplaces          []PluginMarketplaceEntry   `json:"marketplaces"`
	RemoteSyncError       *string                    `json:"remoteSyncError,omitempty"`
}

// PluginInstalledResponse contains installed plugin listings.
type PluginInstalledResponse struct {
	MarketplaceLoadErrors []MarketplaceLoadErrorInfo `json:"marketplaceLoadErrors,omitempty"`
	Marketplaces          []PluginMarketplaceEntry   `json:"marketplaces"`
}

// MarketplaceLoadErrorInfo describes a marketplace loading failure.
type MarketplaceLoadErrorInfo struct {
	MarketplacePath string `json:"marketplacePath"`
	Message         string `json:"message"`
}

// Validate records at their existing response owners without adding JSON
// methods to the public representation. Ordinary wire decoding runs first so
// its saved type errors retain their original context and precedence.
func validateMarketplaceLoadErrors(data []byte) error {
	return validateOptionalArrays(data, func(raw []byte) error {
		return validateArrayElements(raw, func(i int, record []byte) error {
			if err := validateMarketplaceLoadError(record); err != nil {
				return fmt.Errorf("record %d: %w", i, err)
			}
			return nil
		})
	}, "marketplaceLoadErrors")
}

// The response wire has already checked JSON syntax and ordinary field types.
// Keep canonical schema requiredness, but inspect nulls and select the final
// path using the same folded names as that wire. Borrow spans instead of
// allocating another record graph merely to validate its path.
func validateMarketplaceLoadError(record []byte) error {
	validation := inboundObjectValidationErrors()
	if len(record) == 0 || record[0] != '{' {
		return validation.notObject(fmt.Errorf("expected JSON object"))
	}
	var pathPresent, messagePresent bool
	var pathRaw []byte
	var err error
	jsonobject.WalkFields(record, true, func(key, raw []byte) {
		if err != nil {
			return
		}
		pathPresent = pathPresent || jsonobject.FieldMatches(key, "marketplacePath")
		messagePresent = messagePresent || jsonobject.FieldMatches(key, "message")
		if jsonobject.FieldMatchesFolded(key, "marketplacepath") {
			if isNullJSONValue(raw) {
				err = validation.null("marketplacePath")
				return
			}
			pathRaw = raw
		} else if jsonobject.FieldMatchesFolded(key, "message") && isNullJSONValue(raw) {
			err = validation.null("message")
		}
	})
	if err != nil {
		return err
	}
	if !pathPresent {
		return validation.missing("marketplacePath")
	}
	if !messagePresent {
		return validation.missing("message")
	}
	var path string
	if err := json.Unmarshal(pathRaw, &path); err != nil {
		return err
	}
	_, err = validateInboundAbsolutePathField("marketplacePath", path)
	return err
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
	if err := validateOptionalStringArrays(data, "featuredPluginIds"); err != nil {
		return err
	}
	if err := validateMarketplaceLoadErrors(data); err != nil {
		return err
	}
	*r = PluginListResponse(decoded)
	return nil
}

func (r *PluginInstalledResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "marketplaces"); err != nil {
		return err
	}
	type wire PluginInstalledResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateMarketplaceLoadErrors(data); err != nil {
		return err
	}
	*r = PluginInstalledResponse(decoded)
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
	Category    *string `json:"category,omitempty"`
	Description *string `json:"description,omitempty"`
	ID          string  `json:"id"`
	InstallURL  *string `json:"installUrl,omitempty"`
	Name        string  `json:"name"`
	NeedsAuth   bool    `json:"needsAuth"`
}

func (a *AppSummary) UnmarshalJSON(data []byte) error {
	type appSummaryWire struct {
		Category    *string `json:"category"`
		Description *string `json:"description"`
		ID          *string `json:"id"`
		InstallURL  *string `json:"installUrl"`
		Name        *string `json:"name"`
		NeedsAuth   *bool   `json:"needsAuth"`
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
	a.Category = wire.Category
	a.ID = *wire.ID
	a.InstallURL = wire.InstallURL
	a.Name = *wire.Name
	a.NeedsAuth = wire.NeedsAuth != nil && *wire.NeedsAuth
	return nil
}

// AppTemplateUnavailableReason explains why an app template cannot be used.
type AppTemplateUnavailableReason string

const (
	AppTemplateUnavailableReasonNotConfiguredForWorkspace AppTemplateUnavailableReason = "NOT_CONFIGURED_FOR_WORKSPACE"
	AppTemplateUnavailableReasonNoActiveWorkspace         AppTemplateUnavailableReason = "NO_ACTIVE_WORKSPACE"
)

var validAppTemplateUnavailableReasons = map[AppTemplateUnavailableReason]struct{}{
	AppTemplateUnavailableReasonNotConfiguredForWorkspace: {}, AppTemplateUnavailableReasonNoActiveWorkspace: {},
}

func (r *AppTemplateUnavailableReason) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "plugin.appTemplate.reason", validAppTemplateUnavailableReasons, r)
}

func (r AppTemplateUnavailableReason) MarshalJSON() ([]byte, error) {
	return marshalEnumString("plugin.appTemplate.reason", r, validAppTemplateUnavailableReasons)
}

// AppTemplateSummary describes an app template bundled with a plugin.
type AppTemplateSummary struct {
	CanonicalConnectorID *string                       `json:"canonicalConnectorId,omitempty"`
	Category             *string                       `json:"category,omitempty"`
	Description          *string                       `json:"description,omitempty"`
	LogoURL              *string                       `json:"logoUrl,omitempty"`
	LogoURLDark          *string                       `json:"logoUrlDark,omitempty"`
	MaterializedAppIDs   []string                      `json:"materializedAppIds"`
	Name                 string                        `json:"name"`
	Reason               *AppTemplateUnavailableReason `json:"reason,omitempty"`
	TemplateID           string                        `json:"templateId"`
}

func (s AppTemplateSummary) MarshalJSON() ([]byte, error) {
	if s.MaterializedAppIDs == nil {
		return nil, errors.New("missing plugin.appTemplate.materializedAppIds")
	}
	type wire AppTemplateSummary
	return json.Marshal(wire(s))
}

func (s *AppTemplateSummary) UnmarshalJSON(data []byte) error {
	type wire AppTemplateSummary
	var decoded struct {
		wire
		MaterializedAppIDs nonNullStringList `json:"materializedAppIds"`
	}
	required := []string{"materializedAppIds", "name", "templateId"}
	if err := unmarshalResponseObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.wire.MaterializedAppIDs = []string(decoded.MaterializedAppIDs)
	*s = AppTemplateSummary(decoded.wire)
	return nil
}

// ScheduledTaskWeekday is an iCalendar weekday identifier.
type ScheduledTaskWeekday string

const (
	ScheduledTaskWeekdayMonday    ScheduledTaskWeekday = "MO"
	ScheduledTaskWeekdayTuesday   ScheduledTaskWeekday = "TU"
	ScheduledTaskWeekdayWednesday ScheduledTaskWeekday = "WE"
	ScheduledTaskWeekdayThursday  ScheduledTaskWeekday = "TH"
	ScheduledTaskWeekdayFriday    ScheduledTaskWeekday = "FR"
	ScheduledTaskWeekdaySaturday  ScheduledTaskWeekday = "SA"
	ScheduledTaskWeekdaySunday    ScheduledTaskWeekday = "SU"
)

var validScheduledTaskWeekdays = map[ScheduledTaskWeekday]struct{}{
	ScheduledTaskWeekdayMonday: {}, ScheduledTaskWeekdayTuesday: {}, ScheduledTaskWeekdayWednesday: {},
	ScheduledTaskWeekdayThursday: {}, ScheduledTaskWeekdayFriday: {}, ScheduledTaskWeekdaySaturday: {}, ScheduledTaskWeekdaySunday: {},
}

func (d *ScheduledTaskWeekday) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "plugin.scheduledTask.schedule.days", validScheduledTaskWeekdays, d)
}

func (d ScheduledTaskWeekday) MarshalJSON() ([]byte, error) {
	return marshalEnumString("plugin.scheduledTask.schedule.days", d, validScheduledTaskWeekdays)
}

// ScheduledTaskSchedule is the schedule union for a plugin task.
type ScheduledTaskSchedule interface{ isScheduledTaskSchedule() }

// HourlyScheduledTaskSchedule runs at a fixed hourly interval, optionally on selected days.
type HourlyScheduledTaskSchedule struct {
	Days          OptionalNullable[[]ScheduledTaskWeekday] `json:"days,omitzero"`
	IntervalHours uint32                                   `json:"intervalHours"`
}

func (HourlyScheduledTaskSchedule) isScheduledTaskSchedule() {}

func (s HourlyScheduledTaskSchedule) MarshalJSON() ([]byte, error) {
	type wire HourlyScheduledTaskSchedule
	return json.Marshal(struct {
		Type string `json:"type"`
		wire
	}{"hourly", wire(s)})
}

func (s *HourlyScheduledTaskSchedule) UnmarshalJSON(data []byte) error {
	type wire HourlyScheduledTaskSchedule
	var decoded wire
	if err := validateScheduledTaskScheduleType(data, "hourly"); err != nil {
		return err
	}
	if err := validateRequiredTaggedObjectFields(data, "intervalHours"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = HourlyScheduledTaskSchedule(decoded)
	return nil
}

// DailyScheduledTaskSchedule runs daily at the specified time.
type DailyScheduledTaskSchedule struct {
	Time string `json:"time"`
}

func (DailyScheduledTaskSchedule) isScheduledTaskSchedule() {}

func (s DailyScheduledTaskSchedule) MarshalJSON() ([]byte, error) {
	type wire DailyScheduledTaskSchedule
	return json.Marshal(struct {
		Type string `json:"type"`
		wire
	}{"daily", wire(s)})
}

func (s *DailyScheduledTaskSchedule) UnmarshalJSON(data []byte) error {
	type wire DailyScheduledTaskSchedule
	var decoded wire
	if err := validateScheduledTaskScheduleType(data, "daily"); err != nil {
		return err
	}
	if err := validateRequiredTaggedObjectFields(data, "time"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = DailyScheduledTaskSchedule(decoded)
	return nil
}

// WeekdaysScheduledTaskSchedule runs each weekday at the specified time.
type WeekdaysScheduledTaskSchedule struct {
	Time string `json:"time"`
}

func (WeekdaysScheduledTaskSchedule) isScheduledTaskSchedule() {}

func (s WeekdaysScheduledTaskSchedule) MarshalJSON() ([]byte, error) {
	type wire WeekdaysScheduledTaskSchedule
	return json.Marshal(struct {
		Type string `json:"type"`
		wire
	}{"weekdays", wire(s)})
}

func (s *WeekdaysScheduledTaskSchedule) UnmarshalJSON(data []byte) error {
	type wire WeekdaysScheduledTaskSchedule
	var decoded wire
	if err := validateScheduledTaskScheduleType(data, "weekdays"); err != nil {
		return err
	}
	if err := validateRequiredTaggedObjectFields(data, "time"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = WeekdaysScheduledTaskSchedule(decoded)
	return nil
}

// WeeklyScheduledTaskSchedule runs on the specified days and time.
type WeeklyScheduledTaskSchedule struct {
	Days []ScheduledTaskWeekday `json:"days"`
	Time string                 `json:"time"`
}

func (WeeklyScheduledTaskSchedule) isScheduledTaskSchedule() {}

func (s WeeklyScheduledTaskSchedule) MarshalJSON() ([]byte, error) {
	if s.Days == nil {
		return nil, errors.New("missing plugin.scheduledTask.schedule.days")
	}
	type wire WeeklyScheduledTaskSchedule
	return json.Marshal(struct {
		Type string `json:"type"`
		wire
	}{"weekly", wire(s)})
}

func (s *WeeklyScheduledTaskSchedule) UnmarshalJSON(data []byte) error {
	type wire WeeklyScheduledTaskSchedule
	var decoded wire
	if err := validateScheduledTaskScheduleType(data, "weekly"); err != nil {
		return err
	}
	if err := validateRequiredTaggedObjectFields(data, "days", "time"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = WeeklyScheduledTaskSchedule(decoded)
	return nil
}

func validateScheduledTaskScheduleType(data []byte, expected string) error {
	typeName, err := decodeRequiredObjectTypeField(data, "scheduled task schedule")
	if err != nil {
		return err
	}
	if typeName != expected {
		return fmt.Errorf("invalid scheduled task schedule type %q; want %q", typeName, expected)
	}
	return nil
}

func unmarshalScheduledTaskSchedule(data []byte) (ScheduledTaskSchedule, error) {
	typeName, err := decodeRequiredObjectTypeField(data, "scheduled task schedule")
	if err != nil {
		return nil, err
	}
	var schedule ScheduledTaskSchedule
	switch typeName {
	case "hourly":
		schedule = &HourlyScheduledTaskSchedule{}
	case "daily":
		schedule = &DailyScheduledTaskSchedule{}
	case "weekdays":
		schedule = &WeekdaysScheduledTaskSchedule{}
	case "weekly":
		schedule = &WeeklyScheduledTaskSchedule{}
	default:
		return nil, fmt.Errorf("unknown scheduled task schedule type: %q", typeName)
	}
	if err := json.Unmarshal(data, schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

// ScheduledTaskSummary describes a scheduled task bundled with a plugin.
type ScheduledTaskSummary struct {
	Key      string                `json:"key"`
	Name     string                `json:"name"`
	Prompt   string                `json:"prompt"`
	Schedule ScheduledTaskSchedule `json:"schedule"`
}

func (s ScheduledTaskSummary) MarshalJSON() ([]byte, error) {
	if isNilInterfaceValue(s.Schedule) {
		return nil, errors.New("missing plugin.scheduledTask.schedule")
	}
	type wire ScheduledTaskSummary
	return json.Marshal(wire(s))
}

func (s *ScheduledTaskSummary) UnmarshalJSON(data []byte) error {
	var wire struct {
		Key      string          `json:"key"`
		Name     string          `json:"name"`
		Prompt   string          `json:"prompt"`
		Schedule json.RawMessage `json:"schedule"`
	}
	required := []string{"key", "name", "prompt", "schedule"}
	if err := unmarshalResponseObject(data, &wire, required, required); err != nil {
		return err
	}
	schedule, err := unmarshalScheduledTaskSchedule(wire.Schedule)
	if err != nil {
		return err
	}
	*s = ScheduledTaskSummary{Key: wire.Key, Name: wire.Name, Prompt: wire.Prompt, Schedule: schedule}
	return nil
}

// SkillSummary describes a skill bundled with a plugin.
type SkillSummary struct {
	Description      string          `json:"description"`
	Enabled          bool            `json:"enabled"`
	Interface        *SkillInterface `json:"interface,omitempty"`
	Name             string          `json:"name"`
	Path             *string         `json:"path,omitempty"`
	ShortDescription *string         `json:"shortDescription,omitempty"`
}

func (s *SkillSummary) UnmarshalJSON(data []byte) error {
	type skillSummaryWire struct {
		Description      *string         `json:"description"`
		Enabled          *bool           `json:"enabled"`
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
	case wire.Enabled == nil:
		return errors.New("missing plugin.skill.enabled")
	case wire.Name == nil:
		return errors.New("missing plugin.skill.name")
	}

	s.Description = *wire.Description
	s.Enabled = *wire.Enabled
	s.Interface = wire.Interface
	s.Name = *wire.Name
	validatedPath, err := validateInboundAbsolutePathPointerField("plugin.skill.path", wire.Path)
	if err != nil {
		return err
	}
	s.Path = validatedPath
	s.ShortDescription = wire.ShortDescription
	return nil
}

// PluginDetail contains full plugin details.
type PluginDetail struct {
	AppTemplates    []AppTemplateSummary                     `json:"appTemplates"`
	ScheduledTasks  OptionalNullable[[]ScheduledTaskSummary] `json:"scheduledTasks,omitzero"`
	ShareURL        *string                                  `json:"shareUrl,omitempty"`
	Apps            []AppSummary                             `json:"apps"`
	Description     *string                                  `json:"description,omitempty"`
	Hooks           []PluginHookSummary                      `json:"hooks"`
	MarketplaceName string                                   `json:"marketplaceName"`
	MarketplacePath *string                                  `json:"marketplacePath,omitempty"`
	McpServers      []string                                 `json:"mcpServers"`
	Skills          []SkillSummary                           `json:"skills"`
	Summary         PluginSummary                            `json:"summary"`
	OnboardingSkill *SkillSummary                            `json:"onboardingSkill,omitempty"`
}

func (p *PluginDetail) UnmarshalJSON(data []byte) error {
	type pluginDetailWire struct {
		AppTemplates    *[]AppTemplateSummary                    `json:"appTemplates"`
		ScheduledTasks  OptionalNullable[[]ScheduledTaskSummary] `json:"scheduledTasks"`
		ShareURL        *string                                  `json:"shareUrl"`
		Apps            *[]AppSummary                            `json:"apps"`
		Description     *string                                  `json:"description"`
		Hooks           *[]PluginHookSummary                     `json:"hooks"`
		MarketplaceName *string                                  `json:"marketplaceName"`
		MarketplacePath *string                                  `json:"marketplacePath"`
		McpServers      *[]string                                `json:"mcpServers"`
		Skills          *[]SkillSummary                          `json:"skills"`
		Summary         *PluginSummary                           `json:"summary"`
		OnboardingSkill *SkillSummary                            `json:"onboardingSkill"`
	}

	var wire pluginDetailWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch {
	case wire.Apps == nil:
		return errors.New("missing plugin.apps")
	case wire.Hooks == nil:
		return errors.New("missing plugin.hooks")
	case wire.MarketplaceName == nil:
		return errors.New("missing plugin.marketplaceName")
	case wire.McpServers == nil:
		return errors.New("missing plugin.mcpServers")
	case wire.Skills == nil:
		return errors.New("missing plugin.skills")
	case wire.Summary == nil:
		return errors.New("missing plugin.summary")
	case wire.AppTemplates == nil:
		return errors.New("missing plugin.appTemplates")
	}

	p.Apps = *wire.Apps
	p.AppTemplates = *wire.AppTemplates
	p.ScheduledTasks = wire.ScheduledTasks
	p.ShareURL = wire.ShareURL
	p.Description = wire.Description
	p.Hooks = *wire.Hooks
	p.MarketplaceName = *wire.MarketplaceName
	validatedMarketplacePath, err := validateInboundAbsolutePathPointerField("plugin.marketplacePath", wire.MarketplacePath)
	if err != nil {
		return err
	}
	p.MarketplacePath = validatedMarketplacePath
	p.McpServers = *wire.McpServers
	p.Skills = *wire.Skills
	p.Summary = *wire.Summary
	p.OnboardingSkill = wire.OnboardingSkill
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
	MarketplacePath       string  `json:"marketplacePath,omitempty"`
	PluginName            string  `json:"pluginName"`
	RemoteMarketplaceName *string `json:"remoteMarketplaceName,omitempty"`
	InstallAttemptID      *string `json:"installAttemptId,omitempty"`
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
	PluginID string `json:"pluginId"`
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

// Installed lists installed plugins across marketplaces.
func (s *PluginService) Installed(ctx context.Context, params PluginInstalledParams) (PluginInstalledResponse, error) {
	var resp PluginInstalledResponse
	if err := s.client.sendRequest(ctx, methodPluginInstalled, params, &resp); err != nil {
		return PluginInstalledResponse{}, err
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
