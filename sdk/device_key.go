package codex

import (
	"context"
	"encoding/json"
)

// DeviceKeyAlgorithm is a device-key signing algorithm.
type DeviceKeyAlgorithm string

const (
	DeviceKeyAlgorithmECDSAP256SHA256 DeviceKeyAlgorithm = "ecdsa_p256_sha256"
)

var validDeviceKeyAlgorithms = map[DeviceKeyAlgorithm]struct{}{
	DeviceKeyAlgorithmECDSAP256SHA256: {},
}

func (a *DeviceKeyAlgorithm) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "deviceKey.algorithm", validDeviceKeyAlgorithms, a)
}

// DeviceKeyProtectionClass describes how a device key is protected locally.
type DeviceKeyProtectionClass string

const (
	DeviceKeyProtectionClassHardwareSecureEnclave     DeviceKeyProtectionClass = "hardware_secure_enclave"
	DeviceKeyProtectionClassHardwareTPM               DeviceKeyProtectionClass = "hardware_tpm"
	DeviceKeyProtectionClassOSProtectedNonextractable DeviceKeyProtectionClass = "os_protected_nonextractable"
)

var validDeviceKeyProtectionClasses = map[DeviceKeyProtectionClass]struct{}{
	DeviceKeyProtectionClassHardwareSecureEnclave:     {},
	DeviceKeyProtectionClassHardwareTPM:               {},
	DeviceKeyProtectionClassOSProtectedNonextractable: {},
}

func (c *DeviceKeyProtectionClass) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "deviceKey.protectionClass", validDeviceKeyProtectionClasses, c)
}

// DeviceKeyProtectionPolicy selects the protection policy used when creating a key.
type DeviceKeyProtectionPolicy string

const (
	DeviceKeyProtectionPolicyHardwareOnly                   DeviceKeyProtectionPolicy = "hardware_only"
	DeviceKeyProtectionPolicyAllowOSProtectedNonextractable DeviceKeyProtectionPolicy = "allow_os_protected_nonextractable"
)

// DeviceKeyCreateParams creates a controller-local device key.
type DeviceKeyCreateParams struct {
	AccountUserID    string                     `json:"accountUserId"`
	ClientID         string                     `json:"clientId"`
	ProtectionPolicy *DeviceKeyProtectionPolicy `json:"protectionPolicy,omitempty"`
}

// DeviceKeyCreateResponse contains device-key metadata and public key material.
type DeviceKeyCreateResponse struct {
	Algorithm              DeviceKeyAlgorithm       `json:"algorithm"`
	KeyID                  string                   `json:"keyId"`
	ProtectionClass        DeviceKeyProtectionClass `json:"protectionClass"`
	PublicKeySpkiDerBase64 string                   `json:"publicKeySpkiDerBase64"`
}

func (r *DeviceKeyCreateResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "algorithm", "keyId", "protectionClass", "publicKeySpkiDerBase64"); err != nil {
		return err
	}
	type wire DeviceKeyCreateResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = DeviceKeyCreateResponse(decoded)
	return nil
}

// DeviceKeyPublicParams requests public metadata for an existing device key.
type DeviceKeyPublicParams struct {
	KeyID string `json:"keyId"`
}

// DeviceKeyPublicResponse contains device-key metadata and public key material.
type DeviceKeyPublicResponse struct {
	Algorithm              DeviceKeyAlgorithm       `json:"algorithm"`
	KeyID                  string                   `json:"keyId"`
	ProtectionClass        DeviceKeyProtectionClass `json:"protectionClass"`
	PublicKeySpkiDerBase64 string                   `json:"publicKeySpkiDerBase64"`
}

func (r *DeviceKeyPublicResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "algorithm", "keyId", "protectionClass", "publicKeySpkiDerBase64"); err != nil {
		return err
	}
	type wire DeviceKeyPublicResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = DeviceKeyPublicResponse(decoded)
	return nil
}

// RemoteControlClientConnectionAudience is the device-key proof audience for controller websocket access.
type RemoteControlClientConnectionAudience string

const (
	RemoteControlClientConnectionAudienceWebsocket RemoteControlClientConnectionAudience = "remote_control_client_websocket"
)

// RemoteControlClientEnrollmentAudience is the device-key proof audience for enrollment.
type RemoteControlClientEnrollmentAudience string

const (
	RemoteControlClientEnrollmentAudienceEnrollment RemoteControlClientEnrollmentAudience = "remote_control_client_enrollment"
)

// DeviceKeySignPayload is a structured payload accepted by device/key/sign.
type DeviceKeySignPayload interface {
	isDeviceKeySignPayload()
}

// RemoteControlClientConnectionDeviceKeySignPayload signs a websocket connection challenge.
type RemoteControlClientConnectionDeviceKeySignPayload struct {
	Type                 string                                `json:"type"`
	AccountUserID        string                                `json:"accountUserId"`
	Audience             RemoteControlClientConnectionAudience `json:"audience"`
	ClientID             string                                `json:"clientId"`
	Nonce                string                                `json:"nonce"`
	Scopes               []string                              `json:"scopes"`
	SessionID            string                                `json:"sessionId"`
	TargetOrigin         string                                `json:"targetOrigin"`
	TargetPath           string                                `json:"targetPath"`
	TokenExpiresAt       int64                                 `json:"tokenExpiresAt"`
	TokenSha256Base64url string                                `json:"tokenSha256Base64url"`
}

func (*RemoteControlClientConnectionDeviceKeySignPayload) isDeviceKeySignPayload() {}

// RemoteControlClientEnrollmentDeviceKeySignPayload signs an enrollment challenge.
type RemoteControlClientEnrollmentDeviceKeySignPayload struct {
	Type                          string                                `json:"type"`
	AccountUserID                 string                                `json:"accountUserId"`
	Audience                      RemoteControlClientEnrollmentAudience `json:"audience"`
	ChallengeExpiresAt            int64                                 `json:"challengeExpiresAt"`
	ChallengeID                   string                                `json:"challengeId"`
	ClientID                      string                                `json:"clientId"`
	DeviceIdentitySha256Base64url string                                `json:"deviceIdentitySha256Base64url"`
	Nonce                         string                                `json:"nonce"`
	TargetOrigin                  string                                `json:"targetOrigin"`
	TargetPath                    string                                `json:"targetPath"`
}

func (*RemoteControlClientEnrollmentDeviceKeySignPayload) isDeviceKeySignPayload() {}

// DeviceKeySignParams signs a structured payload with a device key.
type DeviceKeySignParams struct {
	KeyID   string               `json:"keyId"`
	Payload DeviceKeySignPayload `json:"payload"`
}

// DeviceKeySignResponse contains a signature over the canonicalized payload.
type DeviceKeySignResponse struct {
	Algorithm           DeviceKeyAlgorithm `json:"algorithm"`
	SignatureDerBase64  string             `json:"signatureDerBase64"`
	SignedPayloadBase64 string             `json:"signedPayloadBase64"`
}

func (r *DeviceKeySignResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "algorithm", "signatureDerBase64", "signedPayloadBase64"); err != nil {
		return err
	}
	type wire DeviceKeySignResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = DeviceKeySignResponse(decoded)
	return nil
}

// DeviceKeyService provides device-key operations.
type DeviceKeyService struct {
	client *Client
}

func newDeviceKeyService(client *Client) *DeviceKeyService {
	return &DeviceKeyService{client: client}
}

// Create creates a controller-local device key.
func (s *DeviceKeyService) Create(ctx context.Context, params DeviceKeyCreateParams) (DeviceKeyCreateResponse, error) {
	var resp DeviceKeyCreateResponse
	if err := s.client.sendRequest(ctx, methodDeviceKeyCreate, params, &resp); err != nil {
		return DeviceKeyCreateResponse{}, err
	}
	return resp, nil
}

// Public reads public metadata for an existing device key.
func (s *DeviceKeyService) Public(ctx context.Context, params DeviceKeyPublicParams) (DeviceKeyPublicResponse, error) {
	var resp DeviceKeyPublicResponse
	if err := s.client.sendRequest(ctx, methodDeviceKeyPublic, params, &resp); err != nil {
		return DeviceKeyPublicResponse{}, err
	}
	return resp, nil
}

// Sign signs a structured payload with a device key.
func (s *DeviceKeyService) Sign(ctx context.Context, params DeviceKeySignParams) (DeviceKeySignResponse, error) {
	var resp DeviceKeySignResponse
	if err := s.client.sendRequest(ctx, methodDeviceKeySign, params, &resp); err != nil {
		return DeviceKeySignResponse{}, err
	}
	return resp, nil
}
