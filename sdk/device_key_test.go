package codex_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/sdk"
)

const (
	deviceKeySignPayloadTypeConnection = "remoteControlClientConnection"
	deviceKeySignPayloadTypeEnrollment = "remoteControlClientEnrollment"
	deviceKeyConnectionScope           = "remote_control_controller_websocket"
	deviceKeyExpiresAt                 = int64(1)
)

func TestDeviceKeySignRejectsMalformedPayloadBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		params  codex.DeviceKeySignParams
		wantErr string
	}{
		{
			name: "empty key id",
			params: codex.DeviceKeySignParams{
				Payload: validConnectionDeviceKeySignPayload(),
			},
			wantErr: "keyId must not be empty",
		},
		{
			name: "nil payload",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
			},
			wantErr: "payload must not be null",
		},
		{
			name: "connection payload wrong type",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.Type = deviceKeySignPayloadTypeEnrollment
					return payload
				}(),
			},
			wantErr: `invalid payload.type "remoteControlClientEnrollment"`,
		},
		{
			name: "connection payload wrong audience",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.Audience = codex.RemoteControlClientConnectionAudience("browser")
					return payload
				}(),
			},
			wantErr: `invalid payload.audience "browser"`,
		},
		{
			name: "connection payload missing scope",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.Scopes = nil
					return payload
				}(),
			},
			wantErr: `payload.scopes must contain exactly "remote_control_controller_websocket"`,
		},
		{
			name: "connection payload empty session",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.SessionID = ""
					return payload
				}(),
			},
			wantErr: "payload.sessionId must not be empty",
		},
		{
			name: "enrollment payload wrong type",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validEnrollmentDeviceKeySignPayload()
					payload.Type = deviceKeySignPayloadTypeConnection
					return payload
				}(),
			},
			wantErr: `invalid payload.type "remoteControlClientConnection"`,
		},
		{
			name: "enrollment payload wrong audience",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validEnrollmentDeviceKeySignPayload()
					payload.Audience = codex.RemoteControlClientEnrollmentAudience("browser")
					return payload
				}(),
			},
			wantErr: `invalid payload.audience "browser"`,
		},
		{
			name: "enrollment payload empty challenge id",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validEnrollmentDeviceKeySignPayload()
					payload.ChallengeID = ""
					return payload
				}(),
			},
			wantErr: "payload.challengeId must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_, err := client.DeviceKey.Sign(context.Background(), tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if got := mock.CallCount(); got != 0 {
				t.Fatalf("transport recorded %d requests, want 0", got)
			}
		})
	}
}

func validConnectionDeviceKeySignPayload() *codex.RemoteControlClientConnectionDeviceKeySignPayload {
	return &codex.RemoteControlClientConnectionDeviceKeySignPayload{
		Type:                 deviceKeySignPayloadTypeConnection,
		AccountUserID:        "user-1",
		Audience:             codex.RemoteControlClientConnectionAudienceWebsocket,
		ClientID:             "client-1",
		Nonce:                "nonce-1",
		Scopes:               []string{deviceKeyConnectionScope},
		SessionID:            "session-1",
		TargetOrigin:         "https://example.com",
		TargetPath:           "/client",
		TokenExpiresAt:       deviceKeyExpiresAt,
		TokenSha256Base64url: "token-digest",
	}
}

func validEnrollmentDeviceKeySignPayload() *codex.RemoteControlClientEnrollmentDeviceKeySignPayload {
	return &codex.RemoteControlClientEnrollmentDeviceKeySignPayload{
		Type:                          deviceKeySignPayloadTypeEnrollment,
		AccountUserID:                 "user-1",
		Audience:                      codex.RemoteControlClientEnrollmentAudienceEnrollment,
		ChallengeExpiresAt:            deviceKeyExpiresAt,
		ChallengeID:                   "challenge-1",
		ClientID:                      "client-1",
		DeviceIdentitySha256Base64url: "identity-digest",
		Nonce:                         "nonce-1",
		TargetOrigin:                  "https://example.com",
		TargetPath:                    "/client/enroll",
	}
}
