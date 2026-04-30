package codex

import (
	"context"
	"encoding/json"
)

// ModelProviderCapabilitiesReadParams reads capabilities for the active model provider.
type ModelProviderCapabilitiesReadParams struct{}

// ModelProviderCapabilitiesReadResponse describes active provider capabilities.
type ModelProviderCapabilitiesReadResponse struct {
	ImageGeneration bool `json:"imageGeneration"`
	NamespaceTools  bool `json:"namespaceTools"`
	WebSearch       bool `json:"webSearch"`
}

func (r *ModelProviderCapabilitiesReadResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "imageGeneration", "namespaceTools", "webSearch"); err != nil {
		return err
	}
	type wire ModelProviderCapabilitiesReadResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ModelProviderCapabilitiesReadResponse(decoded)
	return nil
}

// ModelProviderService provides model-provider operations.
type ModelProviderService struct {
	client *Client
}

func newModelProviderService(client *Client) *ModelProviderService {
	return &ModelProviderService{client: client}
}

// CapabilitiesRead reads capabilities for the active model provider.
func (s *ModelProviderService) CapabilitiesRead(ctx context.Context, params ModelProviderCapabilitiesReadParams) (ModelProviderCapabilitiesReadResponse, error) {
	var resp ModelProviderCapabilitiesReadResponse
	if err := s.client.sendRequest(ctx, methodModelProviderCapabilitiesRead, params, &resp); err != nil {
		return ModelProviderCapabilitiesReadResponse{}, err
	}
	return resp, nil
}
