package codex

import (
	"context"
	"encoding/json"
)

// AppsListParams contains parameters for the apps/list request.
type AppsListParams struct {
	Cursor       *string `json:"cursor,omitempty"`
	ForceRefetch bool    `json:"forceRefetch,omitempty"`
	Limit        *uint32 `json:"limit,omitempty"`
	ThreadID     *string `json:"threadId,omitempty"`
}

// AppsListResponse contains the response from apps/list.
type AppsListResponse struct {
	Data       []AppInfo `json:"data"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}

// AppInfo represents metadata for an app/connector.
type AppInfo struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         *string           `json:"description,omitempty"`
	DistributionChannel *string           `json:"distributionChannel,omitempty"`
	InstallURL          *string           `json:"installUrl,omitempty"`
	IsAccessible        bool              `json:"isAccessible"`
	IsEnabled           bool              `json:"isEnabled"`
	Labels              map[string]string `json:"labels,omitempty"`
	LogoURL             *string           `json:"logoUrl,omitempty"`
	LogoURLDark         *string           `json:"logoUrlDark,omitempty"`
	Branding            *AppBranding      `json:"branding,omitempty"`
	AppMetadata         *AppMetadata      `json:"appMetadata,omitempty"`
}

// AppBranding contains branding information for an app.
type AppBranding struct {
	IsDiscoverableApp bool    `json:"isDiscoverableApp"`
	Category          *string `json:"category,omitempty"`
	Developer         *string `json:"developer,omitempty"`
	PrivacyPolicy     *string `json:"privacyPolicy,omitempty"`
	TermsOfService    *string `json:"termsOfService,omitempty"`
	Website           *string `json:"website,omitempty"`
}

// AppMetadata contains extended metadata for an app.
type AppMetadata struct {
	Categories                 *[]string        `json:"categories,omitempty"`
	SubCategories              *[]string        `json:"subCategories,omitempty"`
	Developer                  *string          `json:"developer,omitempty"`
	FirstPartyRequiresInstall  *bool            `json:"firstPartyRequiresInstall,omitempty"`
	FirstPartyType             *string          `json:"firstPartyType,omitempty"`
	Review                     *AppReview       `json:"review,omitempty"`
	Screenshots                *[]AppScreenshot `json:"screenshots,omitempty"`
	SEODescription             *string          `json:"seoDescription,omitempty"`
	ShowInComposerWhenUnlinked *bool            `json:"showInComposerWhenUnlinked,omitempty"`
	Version                    *string          `json:"version,omitempty"`
	VersionID                  *string          `json:"versionId,omitempty"`
	VersionNotes               *string          `json:"versionNotes,omitempty"`
}

// AppReview contains review status information.
type AppReview struct {
	Status string `json:"status"`
}

// AppScreenshot represents a screenshot for an app.
type AppScreenshot struct {
	UserPrompt string  `json:"userPrompt"`
	FileID     *string `json:"fileId,omitempty"`
	URL        *string `json:"url,omitempty"`
}

// AppListUpdatedNotification is sent when the app list changes.
type AppListUpdatedNotification struct {
	Data []AppInfo `json:"data"`
}

// AppsService provides access to app-related operations.
type AppsService struct {
	client *Client
}

func newAppsService(client *Client) *AppsService {
	return &AppsService{client: client}
}

// List retrieves the list of available apps/connectors.
func (s *AppsService) List(ctx context.Context, params AppsListParams) (AppsListResponse, error) {
	var resp AppsListResponse
	if err := s.client.sendRequest(ctx, methodAppList, params, &resp); err != nil {
		return AppsListResponse{}, err
	}
	return resp, nil
}

// OnAppListUpdated registers a listener for app/listUpdated notifications.
func (c *Client) OnAppListUpdated(handler func(AppListUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyAppListUpdated, nil)
		return
	}
	c.OnNotification(notifyAppListUpdated, func(ctx context.Context, notif Notification) {
		var params AppListUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}
