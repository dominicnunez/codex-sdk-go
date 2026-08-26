package protocol

import (
	"context"
	"encoding/json"
)

// PermissionProfileListParams lists available permission profiles.
type PermissionProfileListParams struct {
	Cursor *string `json:"cursor,omitempty"`
	Cwd    *string `json:"cwd,omitempty"`
	Limit  *uint32 `json:"limit,omitempty"`
}

// PermissionProfileSummary describes an available permission profile.
type PermissionProfileSummary struct {
	Description *string `json:"description,omitempty"`
	ID          string  `json:"id"`
	Allowed     bool    `json:"allowed"`
}

func (s *PermissionProfileSummary) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "id"); err != nil {
		return err
	}
	type wire PermissionProfileSummary
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = PermissionProfileSummary(decoded)
	return nil
}

// PermissionProfileListResponse contains available permission profiles.
type PermissionProfileListResponse struct {
	Data       []PermissionProfileSummary `json:"data"`
	NextCursor *string                    `json:"nextCursor,omitempty"`
}

func (r *PermissionProfileListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire PermissionProfileListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = PermissionProfileListResponse(decoded)
	return nil
}

func (p PermissionProfileListParams) prepareRequest() (interface{}, error) {
	var err error
	p.Cwd, err = normalizeOptionalAbsolutePathField("cwd", p.Cwd)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// PermissionProfileService provides permission profile operations.
type PermissionProfileService struct {
	client *Client
}

func newPermissionProfileService(client *Client) *PermissionProfileService {
	return &PermissionProfileService{client: client}
}

// List lists available permission profiles.
func (s *PermissionProfileService) List(ctx context.Context, params PermissionProfileListParams) (PermissionProfileListResponse, error) {
	var resp PermissionProfileListResponse
	if err := s.client.sendRequest(ctx, methodPermissionProfileList, params, &resp); err != nil {
		return PermissionProfileListResponse{}, err
	}
	return resp, nil
}
