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
	Error        *string             `json:"error,omitempty"`
	ProviderID   string              `json:"providerId"`
	ProviderName string              `json:"providerName"`
	Required     bool                `json:"required"`
	Status       *GatewayOAuthStatus `json:"status,omitempty"`
}

func (r *GatewayOAuthReadResponse) UnmarshalJSON(data []byte) error {
	type wire GatewayOAuthReadResponse
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded,
		[]string{"providerId", "providerName", "required"},
		[]string{"providerId", "providerName", "required"}); err != nil {
		return err
	}
	*r = GatewayOAuthReadResponse(decoded)
	return nil
}

type GatewayOAuthLoginResponse struct{}
type GatewayOAuthCancelResponse struct{}

type GatewayOAuthChangedNotification struct {
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
	var response GatewayOAuthReadResponse
	if err := s.client.sendRequest(ctx, methodGatewayOAuthRead, nil, &response); err != nil {
		return GatewayOAuthReadResponse{}, err
	}
	return response, nil
}

func (s *AccountService) GatewayOAuthLogin(ctx context.Context) (GatewayOAuthLoginResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodGatewayOAuthLogin, nil); err != nil {
		return GatewayOAuthLoginResponse{}, err
	}
	return GatewayOAuthLoginResponse{}, nil
}

func (s *AccountService) GatewayOAuthCancel(ctx context.Context) (GatewayOAuthCancelResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodGatewayOAuthCancel, nil); err != nil {
		return GatewayOAuthCancelResponse{}, err
	}
	return GatewayOAuthCancelResponse{}, nil
}

func (c *Client) OnGatewayOAuthChanged(handler func(GatewayOAuthChangedNotification)) {
	if handler == nil {
		c.OnNotification(notifyGatewayOAuthChanged, nil)
		return
	}
	c.OnNotification(notifyGatewayOAuthChanged, func(_ context.Context, notif Notification) {
		var value GatewayOAuthChangedNotification
		if err := jsondecode.Unmarshal(notif.Params, &value); err != nil {
			c.reportHandlerError(notifyGatewayOAuthChanged, fmt.Errorf("unmarshal %s: %w", notifyGatewayOAuthChanged, err))
			return
		}
		handler(value)
	})
}
