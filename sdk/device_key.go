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

var validDeviceKeyProtectionPolicies = map[DeviceKeyProtectionPolicy]struct{}{
	DeviceKeyProtectionPolicyHardwareOnly:                   {},
	DeviceKeyProtectionPolicyAllowOSProtectedNonextractable: {},
}

func validateOptionalDeviceKeyProtectionPolicyField(field string, value *DeviceKeyProtectionPolicy) error {
	return validateOptionalEnumValue(field, value, validDeviceKeyProtectionPolicies)
}

// DeviceKeyCreateParams creates a controller-local device key.
type DeviceKeyCreateParams struct {
	AccountUserID    string                     `json:"accountUserId"`
	ClientID         string                     `json:"clientId"`
	ProtectionPolicy *DeviceKeyProtectionPolicy `json:"protectionPolicy,omitempty"`
}

func (p DeviceKeyCreateParams) prepareRequest() (interface{}, error) {
	if err := validateOptionalDeviceKeyProtectionPolicyField("protectionPolicy", p.ProtectionPolicy); err != nil {
		return nil, invalidParamsError("%v", err)
	}
	return p, nil
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

var validRemoteControlClientConnectionAudiences = map[RemoteControlClientConnectionAudience]struct{}{
	RemoteControlClientConnectionAudienceWebsocket: {},
}

// RemoteControlClientEnrollmentAudience is the device-key proof audience for enrollment.
type RemoteControlClientEnrollmentAudience string

const (
	RemoteControlClientEnrollmentAudienceEnrollment RemoteControlClientEnrollmentAudience = "remote_control_client_enrollment"
)

var validRemoteControlClientEnrollmentAudiences = map[RemoteControlClientEnrollmentAudience]struct{}{
	RemoteControlClientEnrollmentAudienceEnrollment: {},
}

const (
	remoteControlClientConnectionDeviceKeySignPayloadType = "remoteControlClientConnection"
	remoteControlClientEnrollmentDeviceKeySignPayloadType = "remoteControlClientEnrollment"
	remoteControlClientConnectionScope                    = "remote_control_controller_websocket"
	remoteControlClientConnectionScopeCount               = 1
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

func (p DeviceKeySignParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("keyId", p.KeyID); err != nil {
		return nil, err
	}
	if err := validateDeviceKeySignPayload(p.Payload); err != nil {
		return nil, err
	}
	return p, nil
}

func validateDeviceKeySignPayload(payload DeviceKeySignPayload) error {
	if isNilInterfaceValue(payload) {
		return invalidParamsError("payload must not be null")
	}

	switch p := payload.(type) {
	case *RemoteControlClientConnectionDeviceKeySignPayload:
		return validateRemoteControlClientConnectionDeviceKeySignPayload(p)
	case *RemoteControlClientEnrollmentDeviceKeySignPayload:
		return validateRemoteControlClientEnrollmentDeviceKeySignPayload(p)
	default:
		return invalidParamsError("payload has unsupported device-key sign payload type %T", payload)
	}
}

func validateRemoteControlClientConnectionDeviceKeySignPayload(p *RemoteControlClientConnectionDeviceKeySignPayload) error {
	if p == nil {
		return invalidParamsError("payload must not be null")
	}
	if p.Type != remoteControlClientConnectionDeviceKeySignPayloadType {
		return invalidParamsError("invalid payload.type %q", p.Type)
	}
	if err := validateEnumValue("payload.audience", p.Audience, validRemoteControlClientConnectionAudiences); err != nil {
		return invalidParamsError("%v", err)
	}
	if err := validateNonEmptyDeviceKeyPayloadStrings(map[string]string{
		"payload.accountUserId":        p.AccountUserID,
		"payload.clientId":             p.ClientID,
		"payload.nonce":                p.Nonce,
		"payload.sessionId":            p.SessionID,
		"payload.targetOrigin":         p.TargetOrigin,
		"payload.targetPath":           p.TargetPath,
		"payload.tokenSha256Base64url": p.TokenSha256Base64url,
	}); err != nil {
		return err
	}
	if len(p.Scopes) != remoteControlClientConnectionScopeCount || p.Scopes[0] != remoteControlClientConnectionScope {
		return invalidParamsError("payload.scopes must contain exactly %q", remoteControlClientConnectionScope)
	}
	return nil
}

func validateRemoteControlClientEnrollmentDeviceKeySignPayload(p *RemoteControlClientEnrollmentDeviceKeySignPayload) error {
	if p == nil {
		return invalidParamsError("payload must not be null")
	}
	if p.Type != remoteControlClientEnrollmentDeviceKeySignPayloadType {
		return invalidParamsError("invalid payload.type %q", p.Type)
	}
	if err := validateEnumValue("payload.audience", p.Audience, validRemoteControlClientEnrollmentAudiences); err != nil {
		return invalidParamsError("%v", err)
	}
	return validateNonEmptyDeviceKeyPayloadStrings(map[string]string{
		"payload.accountUserId":                 p.AccountUserID,
		"payload.challengeId":                   p.ChallengeID,
		"payload.clientId":                      p.ClientID,
		"payload.deviceIdentitySha256Base64url": p.DeviceIdentitySha256Base64url,
		"payload.nonce":                         p.Nonce,
		"payload.targetOrigin":                  p.TargetOrigin,
		"payload.targetPath":                    p.TargetPath,
	})
}

func validateNonEmptyDeviceKeyPayloadStrings(fields map[string]string) error {
	for field, value := range fields {
		if err := validateRequiredNonEmptyStringField(field, value); err != nil {
			return err
		}
	}
	return nil
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
