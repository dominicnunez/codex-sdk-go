package codex

import "context"

// ExperimentalFeatureStage represents the lifecycle stage of an experimental feature flag
type ExperimentalFeatureStage string

const (
	ExperimentalFeatureStageBeta             ExperimentalFeatureStage = "beta"
	ExperimentalFeatureStageUnderDevelopment ExperimentalFeatureStage = "underDevelopment"
	ExperimentalFeatureStageStable           ExperimentalFeatureStage = "stable"
	ExperimentalFeatureStageDeprecated       ExperimentalFeatureStage = "deprecated"
	ExperimentalFeatureStageRemoved          ExperimentalFeatureStage = "removed"
)

// ExperimentalFeature represents a single experimental feature flag
type ExperimentalFeature struct {
	// Stable key used in config.toml and CLI flag toggles
	Name string `json:"name"`

	// Whether this feature is enabled by default
	DefaultEnabled bool `json:"defaultEnabled"`

	// Whether this feature is currently enabled in the loaded config
	Enabled bool `json:"enabled"`

	// Lifecycle stage of this feature flag
	Stage ExperimentalFeatureStage `json:"stage"`

	// User-facing display name shown in the experimental features UI (null when not in beta)
	DisplayName *string `json:"displayName,omitempty"`

	// Short summary describing what the feature does (null when not in beta)
	Description *string `json:"description,omitempty"`

	// Announcement copy shown to users when the feature is introduced (null when not in beta)
	Announcement *string `json:"announcement,omitempty"`
}

// ExperimentalFeatureListParams contains parameters for listing experimental features
type ExperimentalFeatureListParams struct {
	// Opaque pagination cursor returned by a previous call
	Cursor *string `json:"cursor,omitempty"`

	// Optional page size; defaults to a reasonable server-side value
	Limit *uint32 `json:"limit,omitempty"`
}

// ExperimentalFeatureListResponse contains the response from listing experimental features
type ExperimentalFeatureListResponse struct {
	// Array of experimental features
	Data []ExperimentalFeature `json:"data"`

	// Opaque cursor to pass to the next call to continue after the last item (null if no more items)
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ExperimentalService provides methods for managing experimental features
type ExperimentalService struct {
	client *Client
}

func newExperimentalService(c *Client) *ExperimentalService {
	return &ExperimentalService{client: c}
}

// FeatureList retrieves the list of experimental features
func (s *ExperimentalService) FeatureList(ctx context.Context, params ExperimentalFeatureListParams) (ExperimentalFeatureListResponse, error) {
	var resp ExperimentalFeatureListResponse
	if err := s.client.sendRequest(ctx, methodExperimentalFeatureList, params, &resp); err != nil {
		return ExperimentalFeatureListResponse{}, err
	}
	return resp, nil
}
