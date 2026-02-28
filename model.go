package codex

import (
	"context"
	"encoding/json"
)

// ModelListParams are parameters for listing available models.
type ModelListParams struct {
	// Opaque pagination cursor returned by a previous call.
	Cursor *string `json:"cursor,omitempty"`
	// When true, include models that are hidden from the default picker list.
	IncludeHidden *bool `json:"includeHidden,omitempty"`
	// Optional page size; defaults to a reasonable server-side value.
	Limit *uint32 `json:"limit,omitempty"`
}

// ModelListResponse contains the list of available models.
type ModelListResponse struct {
	// Array of model definitions.
	Data []Model `json:"data"`
	// Opaque cursor to pass to the next call to continue after the last item.
	// If nil, there are no more items to return.
	NextCursor *string `json:"nextCursor,omitempty"`
}

// Model represents a language model available in Codex.
type Model struct {
	// Unique identifier for the model.
	ID string `json:"id"`
	// Model identifier string (e.g., "claude-opus-4-6").
	Model string `json:"model"`
	// Human-readable display name.
	DisplayName string `json:"displayName"`
	// Description of the model's capabilities.
	Description string `json:"description"`
	// Whether the model is hidden from the default picker list.
	Hidden bool `json:"hidden"`
	// Whether this is the default model.
	IsDefault bool `json:"isDefault"`
	// Default reasoning effort level for this model.
	DefaultReasoningEffort ReasoningEffort `json:"defaultReasoningEffort"`
	// Supported reasoning effort options for this model.
	SupportedReasoningEfforts []ReasoningEffortOption `json:"supportedReasoningEfforts"`
	// Input modalities supported by this model (e.g., "text", "image").
	InputModalities []InputModality `json:"inputModalities,omitempty"`
	// Whether the model supports personality customization.
	SupportsPersonality bool `json:"supportsPersonality"`
	// Optional model ID to upgrade to.
	Upgrade *string `json:"upgrade,omitempty"`
}

// ReasoningEffortOption describes a supported reasoning effort level.
type ReasoningEffortOption struct {
	// The reasoning effort level.
	ReasoningEffort ReasoningEffort `json:"reasoningEffort"`
	// Human-readable description of this effort level.
	Description string `json:"description"`
}

// ModelReroutedNotification is sent when a model is rerouted to a different model.
type ModelReroutedNotification struct {
	// Thread ID where the reroute occurred.
	ThreadID string `json:"threadId"`
	// Turn ID where the reroute occurred.
	TurnID string `json:"turnId"`
	// Original model that was requested.
	FromModel string `json:"fromModel"`
	// Model that was used instead.
	ToModel string `json:"toModel"`
	// Reason for the reroute.
	Reason ModelRerouteReason `json:"reason"`
}

// ModelRerouteReason represents the reason why a model was rerouted.
type ModelRerouteReason string

const (
	// Model was rerouted due to high-risk cyber activity detection.
	ModelRerouteReasonHighRiskCyberActivity ModelRerouteReason = "highRiskCyberActivity"
)

// ModelService provides access to model listing and notifications.
type ModelService struct {
	client *Client
}

func newModelService(client *Client) *ModelService {
	return &ModelService{client: client}
}

// List retrieves the list of available models.
func (s *ModelService) List(ctx context.Context, params ModelListParams) (ModelListResponse, error) {
	var resp ModelListResponse
	if err := s.client.sendRequest(ctx, methodModelList, params, &resp); err != nil {
		return ModelListResponse{}, err
	}
	return resp, nil
}

// OnModelRerouted registers a listener for model reroute notifications.
func (c *Client) OnModelRerouted(handler func(ModelReroutedNotification)) {
	if handler == nil {
		c.OnNotification(notifyModelRerouted, nil)
		return
	}
	c.OnNotification(notifyModelRerouted, func(ctx context.Context, notif Notification) {
		var n ModelReroutedNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			return
		}
		handler(n)
	})
}
