package codex

import (
	"context"
	"encoding/json"
	"fmt"
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

func (r *ModelListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ModelListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ModelListResponse(decoded)
	return nil
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
	// Availability nux message for the model.
	AvailabilityNux *ModelAvailabilityNux `json:"availabilityNux,omitempty"`
	// Upgrade information for the model.
	UpgradeInfo *ModelUpgradeInfo `json:"upgradeInfo,omitempty"`
}

func (m *Model) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(
		data,
		"defaultReasoningEffort",
		"description",
		"displayName",
		"hidden",
		"id",
		"isDefault",
		"model",
		"supportedReasoningEfforts",
	); err != nil {
		return err
	}
	type wire Model
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*m = Model(decoded)
	return nil
}

// ModelAvailabilityNux contains an availability nux message for a model.
type ModelAvailabilityNux struct {
	Message string `json:"message"`
}

func (n *ModelAvailabilityNux) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "message"); err != nil {
		return err
	}
	type wire ModelAvailabilityNux
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*n = ModelAvailabilityNux(decoded)
	return nil
}

// ModelUpgradeInfo contains upgrade information for a model.
type ModelUpgradeInfo struct {
	Model             string  `json:"model"`
	MigrationMarkdown *string `json:"migrationMarkdown,omitempty"`
	ModelLink         *string `json:"modelLink,omitempty"`
	UpgradeCopy       *string `json:"upgradeCopy,omitempty"`
}

func (i *ModelUpgradeInfo) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "model"); err != nil {
		return err
	}
	type wire ModelUpgradeInfo
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*i = ModelUpgradeInfo(decoded)
	return nil
}

// ReasoningEffortOption describes a supported reasoning effort level.
type ReasoningEffortOption struct {
	// The reasoning effort level.
	ReasoningEffort ReasoningEffort `json:"reasoningEffort"`
	// Human-readable description of this effort level.
	Description string `json:"description"`
}

func (o *ReasoningEffortOption) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "description", "reasoningEffort"); err != nil {
		return err
	}
	type wire ReasoningEffortOption
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*o = ReasoningEffortOption(decoded)
	return nil
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

func (n *ModelReroutedNotification) UnmarshalJSON(data []byte) error {
	type wire ModelReroutedNotification
	var decoded wire
	required := []string{"fromModel", "reason", "threadId", "toModel", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ModelReroutedNotification(decoded)
	return nil
}

// ModelRerouteReason represents the reason why a model was rerouted.
type ModelRerouteReason string

const (
	// Model was rerouted due to high-risk cyber activity detection.
	ModelRerouteReasonHighRiskCyberActivity ModelRerouteReason = "highRiskCyberActivity"
)

var validModelRerouteReasons = map[ModelRerouteReason]struct{}{
	ModelRerouteReasonHighRiskCyberActivity: {},
}

func (r *ModelRerouteReason) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "model.rerouted.reason", validModelRerouteReasons, r)
}

// ModelVerification describes a verification attached to a model response.
type ModelVerification string

const (
	ModelVerificationTrustedAccessForCyber ModelVerification = "trustedAccessForCyber"
)

// ModelVerificationNotification is sent when model verifications are available.
type ModelVerificationNotification struct {
	ThreadID      string              `json:"threadId"`
	TurnID        string              `json:"turnId"`
	Verifications []ModelVerification `json:"verifications"`
}

func (n *ModelVerificationNotification) UnmarshalJSON(data []byte) error {
	type wire ModelVerificationNotification
	var decoded wire
	required := []string{"threadId", "turnId", "verifications"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ModelVerificationNotification(decoded)
	return nil
}

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
			c.reportHandlerError(notifyModelRerouted, fmt.Errorf("unmarshal %s: %w", notifyModelRerouted, err))
			return
		}
		handler(n)
	})
}

// OnModelVerification registers a listener for model/verification notifications.
func (c *Client) OnModelVerification(handler func(ModelVerificationNotification)) {
	if handler == nil {
		c.OnNotification(notifyModelVerification, nil)
		return
	}
	c.OnNotification(notifyModelVerification, func(ctx context.Context, notif Notification) {
		var n ModelVerificationNotification
		if err := json.Unmarshal(notif.Params, &n); err != nil {
			c.reportHandlerError(notifyModelVerification, fmt.Errorf("unmarshal %s: %w", notifyModelVerification, err))
			return
		}
		handler(n)
	})
}
