package codex_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/sdk"
)

const (
	deviceKeySignPayloadTypeConnection = "remoteControlClientConnection"
	deviceKeySignPayloadTypeEnrollment = "remoteControlClientEnrollment"
	deviceKeyConnectionScope           = "remote_control_controller_websocket"
	deviceKeyExpiresAt                 = int64(1)
	deviceKeyValidBase64               = "AQID"
)

func TestDeviceKeyCreateRejectsEmptyIdentityFieldsBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		params  codex.DeviceKeyCreateParams
		wantErr string
	}{
		{
			name: "empty account user id",
			params: codex.DeviceKeyCreateParams{
				ClientID: "client-1",
			},
			wantErr: "accountUserId must not be empty",
		},
		{
			name: "empty client id",
			params: codex.DeviceKeyCreateParams{
				AccountUserID: "user-1",
			},
			wantErr: "clientId must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_, err := client.DeviceKey.Create(context.Background(), tt.params)
			if err == nil {
				t.Fatal("expected invalid params error")
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

func TestDeviceKeyPublicRejectsEmptyKeyIDBeforeSending(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	_, err := client.DeviceKey.Public(context.Background(), codex.DeviceKeyPublicParams{})
	if err == nil {
		t.Fatal("expected invalid params error")
	}
	if !strings.Contains(err.Error(), "keyId must not be empty") {
		t.Fatalf("error = %q, want keyId validation error", err.Error())
	}
	if got := mock.CallCount(); got != 0 {
		t.Fatalf("transport recorded %d requests, want 0", got)
	}
}

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
			name: "connection payload invalid token digest base64url",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.TokenSha256Base64url = "not+base64url=="
					return payload
				}(),
			},
			wantErr: "payload.tokenSha256Base64url must be valid unpadded base64url",
		},
		{
			name: "connection payload short token digest",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validConnectionDeviceKeySignPayload()
					payload.TokenSha256Base64url = deviceKeyValidBase64
					return payload
				}(),
			},
			wantErr: "payload.tokenSha256Base64url must decode to a SHA-256 digest",
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
		{
			name: "enrollment payload invalid identity digest base64url",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validEnrollmentDeviceKeySignPayload()
					payload.DeviceIdentitySha256Base64url = "not+base64url=="
					return payload
				}(),
			},
			wantErr: "payload.deviceIdentitySha256Base64url must be valid unpadded base64url",
		},
		{
			name: "enrollment payload short identity digest",
			params: codex.DeviceKeySignParams{
				KeyID: "key-1",
				Payload: func() codex.DeviceKeySignPayload {
					payload := validEnrollmentDeviceKeySignPayload()
					payload.DeviceIdentitySha256Base64url = deviceKeyValidBase64
					return payload
				}(),
			},
			wantErr: "payload.deviceIdentitySha256Base64url must decode to a SHA-256 digest",
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

func TestDeviceKeyResponsesRejectMalformedBase64(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		response  map[string]interface{}
		call      func(*codex.Client) error
		wantError string
	}{
		{
			name:   "create public key is empty",
			method: "device/key/create",
			response: map[string]interface{}{
				"algorithm":              codex.DeviceKeyAlgorithmECDSAP256SHA256,
				"keyId":                  "key-1",
				"protectionClass":        codex.DeviceKeyProtectionClassOSProtectedNonextractable,
				"publicKeySpkiDerBase64": "",
			},
			call: func(client *codex.Client) error {
				_, err := client.DeviceKey.Create(context.Background(), codex.DeviceKeyCreateParams{
					AccountUserID: "user-1",
					ClientID:      "client-1",
				})
				return err
			},
			wantError: "publicKeySpkiDerBase64 must not be empty",
		},
		{
			name:   "public response key is invalid",
			method: "device/key/public",
			response: map[string]interface{}{
				"algorithm":              codex.DeviceKeyAlgorithmECDSAP256SHA256,
				"keyId":                  "key-1",
				"protectionClass":        codex.DeviceKeyProtectionClassOSProtectedNonextractable,
				"publicKeySpkiDerBase64": "not base64",
			},
			call: func(client *codex.Client) error {
				_, err := client.DeviceKey.Public(context.Background(), codex.DeviceKeyPublicParams{KeyID: "key-1"})
				return err
			},
			wantError: "publicKeySpkiDerBase64 must be valid base64",
		},
		{
			name:   "public response key decodes empty",
			method: "device/key/public",
			response: map[string]interface{}{
				"algorithm":              codex.DeviceKeyAlgorithmECDSAP256SHA256,
				"keyId":                  "key-1",
				"protectionClass":        codex.DeviceKeyProtectionClassOSProtectedNonextractable,
				"publicKeySpkiDerBase64": "\n",
			},
			call: func(client *codex.Client) error {
				_, err := client.DeviceKey.Public(context.Background(), codex.DeviceKeyPublicParams{KeyID: "key-1"})
				return err
			},
			wantError: "publicKeySpkiDerBase64 must decode to non-empty bytes",
		},
		{
			name:   "sign signature is invalid",
			method: "device/key/sign",
			response: map[string]interface{}{
				"algorithm":           codex.DeviceKeyAlgorithmECDSAP256SHA256,
				"signatureDerBase64":  "not base64",
				"signedPayloadBase64": deviceKeyValidBase64,
			},
			call: func(client *codex.Client) error {
				_, err := client.DeviceKey.Sign(context.Background(), codex.DeviceKeySignParams{
					KeyID:   "key-1",
					Payload: validConnectionDeviceKeySignPayload(),
				})
				return err
			},
			wantError: "signatureDerBase64 must be valid base64",
		},
		{
			name:   "sign signed payload is empty",
			method: "device/key/sign",
			response: map[string]interface{}{
				"algorithm":           codex.DeviceKeyAlgorithmECDSAP256SHA256,
				"signatureDerBase64":  deviceKeyValidBase64,
				"signedPayloadBase64": "",
			},
			call: func(client *codex.Client) error {
				_, err := client.DeviceKey.Sign(context.Background(), codex.DeviceKeySignParams{
					KeyID:   "key-1",
					Payload: validConnectionDeviceKeySignPayload(),
				})
				return err
			},
			wantError: "signedPayloadBase64 must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			if err := mock.SetResponseData(tt.method, tt.response); err != nil {
				t.Fatalf("SetResponseData(%q): %v", tt.method, err)
			}
			client := codex.NewClient(mock)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected malformed base64 response error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantError)
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
		TokenSha256Base64url: validDeviceKeyHashBase64URL(),
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
		DeviceIdentitySha256Base64url: validDeviceKeyHashBase64URL(),
		Nonce:                         "nonce-1",
		TargetOrigin:                  "https://example.com",
		TargetPath:                    "/client/enroll",
	}
}

func validDeviceKeyHashBase64URL() string {
	digest := make([]byte, sha256.Size)
	return base64.RawURLEncoding.EncodeToString(digest)
}
