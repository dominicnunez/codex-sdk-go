package protocol

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

type GatewayOAuthStatus string

const (
	GatewayOAuthStatusNotReady  GatewayOAuthStatus = "notReady"
	GatewayOAuthStatusStarted   GatewayOAuthStatus = "started"
	GatewayOAuthStatusSucceeded GatewayOAuthStatus = "succeeded"
	GatewayOAuthStatusFailed    GatewayOAuthStatus = "failed"
)

var validGatewayOAuthStatuses = map[GatewayOAuthStatus]struct{}{
	GatewayOAuthStatusNotReady: {}, GatewayOAuthStatusStarted: {},
	GatewayOAuthStatusSucceeded: {}, GatewayOAuthStatusFailed: {},
}

func (s GatewayOAuthStatus) MarshalJSON() ([]byte, error) {
	return marshalEnumString("GatewayOAuthStatus", s, validGatewayOAuthStatuses)
}
func (s *GatewayOAuthStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "GatewayOAuthStatus", validGatewayOAuthStatuses, s)
}

type GatewayOAuthReadResponse struct {
	SupportsDeviceAuth bool                `json:"supportsDeviceAuth"`
	Error              *string             `json:"error,omitempty"`
	ProviderID         string              `json:"providerId"`
	ProviderName       string              `json:"providerName"`
	Required           bool                `json:"required"`
	Status             *GatewayOAuthStatus `json:"status,omitempty"`
}

func (r *GatewayOAuthReadResponse) UnmarshalJSON(data []byte) error {
	type wire GatewayOAuthReadResponse
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded,
		[]string{"providerId", "providerName", "required"},
		[]string{"providerId", "providerName", "required"}); err != nil {
		return err
	}
	if err := validateOptionalBooleans(data, "supportsDeviceAuth"); err != nil {
		return err
	}
	*r = GatewayOAuthReadResponse(decoded)
	return nil
}

type GatewayOAuthLoginResponse struct {
	Prompt *GatewayOAuthPrompt `json:"prompt,omitempty"`
}
type GatewayOAuthCancelResponse struct{}

type GatewayOAuthChangedNotification struct {
	LoginID    *string            `json:"loginId,omitempty"`
	AuthURL    *string            `json:"authUrl,omitempty"`
	Error      *string            `json:"error,omitempty"`
	ProviderID string             `json:"providerId"`
	Status     GatewayOAuthStatus `json:"status"`
}

func (n *GatewayOAuthChangedNotification) UnmarshalJSON(data []byte) error {
	type wire GatewayOAuthChangedNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded,
		[]string{"providerId", "status"}, []string{"providerId", "status"}); err != nil {
		return err
	}
	*n = GatewayOAuthChangedNotification(decoded)
	return nil
}

func (s *AccountService) GatewayOAuthRead(ctx context.Context) (GatewayOAuthReadResponse, error) {
	return s.GatewayOAuthReadWithParams(ctx, nil)
}

// GatewayOAuthReadWithParams reads readiness for a selected provider.
func (s *AccountService) GatewayOAuthReadWithParams(ctx context.Context, params *GatewayOAuthReadParams) (GatewayOAuthReadResponse, error) {
	var response GatewayOAuthReadResponse
	if err := s.client.sendRequest(ctx, methodGatewayOAuthRead, params, &response); err != nil {
		return GatewayOAuthReadResponse{}, err
	}
	return response, nil
}

func (s *AccountService) GatewayOAuthLogin(ctx context.Context) (GatewayOAuthLoginResponse, error) {
	return s.GatewayOAuthLoginWithParams(ctx, nil)
}

// GatewayOAuthLoginWithParams starts provider login using the requested flow.
func (s *AccountService) GatewayOAuthLoginWithParams(ctx context.Context, params *GatewayOAuthLoginParams) (GatewayOAuthLoginResponse, error) {
	var response gatewayLoginAdmission
	if err := s.client.sendRequest(ctx, methodGatewayOAuthLogin, params, &response); err != nil {
		return GatewayOAuthLoginResponse{}, err
	}
	return response.GatewayOAuthLoginResponse, nil
}

func (s *AccountService) GatewayOAuthCancel(ctx context.Context) (GatewayOAuthCancelResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodGatewayOAuthCancel, nil); err != nil {
		return GatewayOAuthCancelResponse{}, err
	}
	return GatewayOAuthCancelResponse{}, nil
}

func (c *Client) OnGatewayOAuthChanged(handler func(GatewayOAuthChangedNotification)) {
	if handler == nil {
		c.onNotification(notifyGatewayOAuthChanged, nil)
		return
	}
	c.onNotification(notifyGatewayOAuthChanged, func(_ context.Context, notif Notification) {
		var value GatewayOAuthChangedNotification
		if err := jsondecode.Unmarshal(notif.Params, &value); err != nil {
			c.reportHandlerError(notifyGatewayOAuthChanged, fmt.Errorf("unmarshal %s: %w", notifyGatewayOAuthChanged, err))
			return
		}
		handler(value)
	})
}

type GatewayOAuthReadParams struct {
	ModelProvider *string `json:"modelProvider,omitempty"`
}
type GatewayOAuthLoginParams struct {
	DeviceAuth    *bool   `json:"deviceAuth,omitempty"`
	ModelProvider *string `json:"modelProvider,omitempty"`
}

// NullableGatewayOAuthReadParams includes the schema's null request alternative.
type NullableGatewayOAuthReadParams = *GatewayOAuthReadParams

// NullableGatewayOAuthLoginParams includes the schema's null request alternative.
type NullableGatewayOAuthLoginParams = *GatewayOAuthLoginParams

type GatewayOAuthPrompt struct {
	LoginID         string `json:"loginId"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
}

// gatewayLoginAdmission owns service validation without adding JSON methods to
// the public response or its nested records.
type gatewayLoginAdmission struct{ GatewayOAuthLoginResponse }

func (r *gatewayLoginAdmission) UnmarshalJSON(data []byte) error {
	type wire GatewayOAuthLoginResponse
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	if err := validateNestedRecord(data, "prompt", recordRule{required: []string{"loginId", "userCode", "verificationUrl"}}); err != nil {
		return err
	}
	r.GatewayOAuthLoginResponse = GatewayOAuthLoginResponse(decoded)
	return nil
}
