package protocol

import "context"

type ModelProviderDefaultSetParams struct {
	ProviderID string `json:"providerId"`
}

func (s *ModelProviderService) DefaultSet(ctx context.Context, params ModelProviderDefaultSetParams) (ConfigWriteResponse, error) {
	var response ConfigWriteResponse
	if err := s.client.sendRequest(ctx, methodModelProviderDefaultSet, params, &response); err != nil {
		return ConfigWriteResponse{}, err
	}
	return response, nil
}
