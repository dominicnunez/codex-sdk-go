package protocol

import (
	"context"
	"fmt"
	"reflect"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

type ModelProviderListParams struct{}
type ModelProviderListResponse struct {
	CanChangeDefault  bool                   `json:"canChangeDefault"`
	Data              []ModelProviderSummary `json:"data"`
	DefaultProviderID string                 `json:"defaultProviderId"`
}
type ModelProviderConfigWriteParams struct {
	Settings ProviderSettingsPatchWrapper `json:"settings"`
}
type ModelProviderConfigWriteResponse struct {
	Provider ModelProviderSummary `json:"provider"`
}

// ModelProviderSummary contains display metadata; credential paths name files on the server host.
// Validation belongs to the containing response, so this record keeps ordinary JSON behavior.
type ModelProviderSummary struct {
	BaseURL  *string                 `json:"baseUrl,omitempty"`
	CanEdit  bool                    `json:"canEdit"`
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	Settings ProviderSettingsWrapper `json:"settings"`
}

func (s *ModelProviderService) List(ctx context.Context, params ModelProviderListParams) (ModelProviderListResponse, error) {
	var response ModelProviderListResponse
	if err := s.client.sendRequest(ctx, methodModelProviderList, params, &response); err != nil {
		return ModelProviderListResponse{}, err
	}
	return response, nil
}
func (s *ModelProviderService) ConfigWrite(ctx context.Context, params ModelProviderConfigWriteParams) (ModelProviderConfigWriteResponse, error) {
	var response ModelProviderConfigWriteResponse
	if err := s.client.sendRequest(ctx, methodModelProviderConfigWrite, params, &response); err != nil {
		return ModelProviderConfigWriteResponse{}, err
	}
	return response, nil
}

// ProviderSettings is a schema-tagged union. Use ProviderSettingsWrapper in JSON carriers.
type ProviderSettings interface{ isProviderSettings() }
type ProviderSettingsWrapper struct{ Value ProviderSettings }

type OpenaiProviderSettings struct{}

func (OpenaiProviderSettings) isProviderSettings() {}

type CustomGatewayProviderSettings struct {
	Auth GatewayAuthWrapper `json:"auth"`
}

func (CustomGatewayProviderSettings) isProviderSettings() {}

type BedrockProviderSettings struct {
	Profile *string `json:"profile,omitempty"`
	Region  *string `json:"region,omitempty"`
}

func (BedrockProviderSettings) isProviderSettings() {}

type GcpProviderSettings struct {
	CredentialFile *string `json:"credentialFile,omitempty"`
	Location       *string `json:"location,omitempty"`
	ProjectID      *string `json:"projectId,omitempty"`
}

func (GcpProviderSettings) isProviderSettings() {}

func (w ProviderSettingsWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil || (reflect.ValueOf(w.Value).Kind() == reflect.Pointer && reflect.ValueOf(w.Value).IsNil()) {
		return nil, fmt.Errorf("nil ProviderSettings")
	}
	var data []byte
	var err error
	switch value := w.Value.(type) {
	case OpenaiProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*OpenaiProviderSettings
		}{"openai", &value})
	case *OpenaiProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*OpenaiProviderSettings
		}{"openai", value})
	case CustomGatewayProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*CustomGatewayProviderSettings
		}{"customGateway", &value})
	case *CustomGatewayProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*CustomGatewayProviderSettings
		}{"customGateway", value})
	case BedrockProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*BedrockProviderSettings
		}{"bedrock", &value})
	case *BedrockProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*BedrockProviderSettings
		}{"bedrock", value})
	case GcpProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*GcpProviderSettings
		}{"gcp", &value})
	case *GcpProviderSettings:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*GcpProviderSettings
		}{"gcp", value})
	default:
		data, err = jsonencode.Marshal(w.Value)
	}
	if err != nil {
		return nil, err
	}
	var checked ProviderSettingsWrapper
	if err = checked.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (w *ProviderSettingsWrapper) UnmarshalJSON(data []byte) error {
	kind, err := providerUnionKind(data, "type", "openai", "customGateway", "bedrock", "gcp")
	if err != nil {
		return err
	}
	var decoded ProviderSettings
	switch kind {
	case "openai":
		var value OpenaiProviderSettings
		if err := unmarshalResponseObject(data, &value, []string{"type"}, []string{"type"}); err != nil {
			return err
		}
		decoded = value
	case "customGateway":
		var value CustomGatewayProviderSettings
		if err := unmarshalResponseObject(data, &value, []string{"type", "auth"}, []string{"type", "auth"}); err != nil {
			return err
		}
		decoded = value
	case "bedrock":
		var value BedrockProviderSettings
		if err := unmarshalResponseObject(data, &value, []string{"type"}, []string{"type"}); err != nil {
			return err
		}
		decoded = value
	case "gcp":
		var value GcpProviderSettings
		if err := unmarshalResponseObject(data, &value, []string{"type"}, []string{"type"}); err != nil {
			return err
		}
		decoded = value
	}
	w.Value = decoded
	return nil
}

// GatewayAuth is a schema-tagged union. Use GatewayAuthWrapper in JSON carriers.
type GatewayAuth interface{ isGatewayAuth() }
type GatewayAuthWrapper struct{ Value GatewayAuth }

type NoneGatewayAuth struct{}

func (NoneGatewayAuth) isGatewayAuth() {}

type EnvironmentVariableGatewayAuth struct {
	EnvKey string `json:"envKey"`
}

func (EnvironmentVariableGatewayAuth) isGatewayAuth() {}

type ModeGatewayAuth struct {
	EnvKey *string `json:"envKey,omitempty"`
}

func (ModeGatewayAuth) isGatewayAuth() {}

func (w GatewayAuthWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil || (reflect.ValueOf(w.Value).Kind() == reflect.Pointer && reflect.ValueOf(w.Value).IsNil()) {
		return nil, fmt.Errorf("nil GatewayAuth")
	}
	var data []byte
	var err error
	switch value := w.Value.(type) {
	case NoneGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*NoneGatewayAuth
		}{"none", &value})
	case *NoneGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*NoneGatewayAuth
		}{"none", value})
	case EnvironmentVariableGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*EnvironmentVariableGatewayAuth
		}{"environmentVariable", &value})
	case *EnvironmentVariableGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*EnvironmentVariableGatewayAuth
		}{"environmentVariable", value})
	case ModeGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*ModeGatewayAuth
		}{"externallyConfigured", &value})
	case *ModeGatewayAuth:
		data, err = jsonencode.Marshal(struct {
			Mode string `json:"mode"`
			*ModeGatewayAuth
		}{"externallyConfigured", value})
	default:
		data, err = jsonencode.Marshal(w.Value)
	}
	if err != nil {
		return nil, err
	}
	var checked GatewayAuthWrapper
	if err = checked.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (w *GatewayAuthWrapper) UnmarshalJSON(data []byte) error {
	kind, err := providerUnionKind(data, "mode", "none", "environmentVariable", "externallyConfigured")
	if err != nil {
		return err
	}
	var decoded GatewayAuth
	switch kind {
	case "none":
		var value NoneGatewayAuth
		if err := unmarshalResponseObject(data, &value, []string{"mode"}, []string{"mode"}); err != nil {
			return err
		}
		decoded = value
	case "environmentVariable":
		var value EnvironmentVariableGatewayAuth
		if err := unmarshalResponseObject(data, &value, []string{"mode", "envKey"}, []string{"mode", "envKey"}); err != nil {
			return err
		}
		decoded = value
	case "externallyConfigured":
		var value ModeGatewayAuth
		if err := unmarshalResponseObject(data, &value, []string{"mode"}, []string{"mode"}); err != nil {
			return err
		}
		decoded = value
	}
	w.Value = decoded
	return nil
}

// ProviderSettingsPatch is a schema-tagged union. Use ProviderSettingsPatchWrapper in JSON carriers.
type ProviderSettingsPatch interface{ isProviderSettingsPatch() }
type ProviderSettingsPatchWrapper struct{ Value ProviderSettingsPatch }

type CustomGatewayProviderSettingsPatch struct {
	BaseURL    OptionalNullable[string] `json:"baseUrl,omitzero"`
	EnvKey     OptionalNullable[string] `json:"envKey,omitzero"`
	Name       *string                  `json:"name,omitempty"`
	ProviderID OptionalNullable[string] `json:"providerId,omitzero"`
}

func (CustomGatewayProviderSettingsPatch) isProviderSettingsPatch() {}

type BedrockProviderSettingsPatch struct {
	Profile    OptionalNullable[string] `json:"profile,omitzero"`
	ProviderID BedrockProviderID        `json:"providerId"`
	Region     OptionalNullable[string] `json:"region,omitzero"`
}

func (BedrockProviderSettingsPatch) isProviderSettingsPatch() {}

type GcpProviderSettingsPatch struct {
	CredentialFile OptionalNullable[string] `json:"credentialFile,omitzero"`
	Location       OptionalNullable[string] `json:"location,omitzero"`
	ProjectID      OptionalNullable[string] `json:"projectId,omitzero"`
	ProviderID     GcpProviderID            `json:"providerId"`
}

func (GcpProviderSettingsPatch) isProviderSettingsPatch() {}

func (w ProviderSettingsPatchWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil || (reflect.ValueOf(w.Value).Kind() == reflect.Pointer && reflect.ValueOf(w.Value).IsNil()) {
		return nil, fmt.Errorf("nil ProviderSettingsPatch")
	}
	var data []byte
	var err error
	switch value := w.Value.(type) {
	case CustomGatewayProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*CustomGatewayProviderSettingsPatch
		}{"customGateway", &value})
	case *CustomGatewayProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*CustomGatewayProviderSettingsPatch
		}{"customGateway", value})
	case BedrockProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*BedrockProviderSettingsPatch
		}{"bedrock", &value})
	case *BedrockProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*BedrockProviderSettingsPatch
		}{"bedrock", value})
	case GcpProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*GcpProviderSettingsPatch
		}{"gcp", &value})
	case *GcpProviderSettingsPatch:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*GcpProviderSettingsPatch
		}{"gcp", value})
	default:
		data, err = jsonencode.Marshal(w.Value)
	}
	if err != nil {
		return nil, err
	}
	var checked ProviderSettingsPatchWrapper
	if err = checked.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (w *ProviderSettingsPatchWrapper) UnmarshalJSON(data []byte) error {
	kind, err := providerUnionKind(data, "type", "customGateway", "bedrock", "gcp")
	if err != nil {
		return err
	}
	var decoded ProviderSettingsPatch
	switch kind {
	case "customGateway":
		var value CustomGatewayProviderSettingsPatch
		if err := unmarshalResponseObject(data, &value, []string{"type"}, []string{"type", "name"}); err != nil {
			return err
		}
		if err := validateClosedObjectFields(data, "type", "baseUrl", "envKey", "name", "providerId"); err != nil {
			return err
		}
		decoded = value
	case "bedrock":
		var value BedrockProviderSettingsPatch
		if err := unmarshalResponseObject(data, &value, []string{"type", "providerId"}, []string{"type", "providerId"}); err != nil {
			return err
		}
		if err := validateClosedObjectFields(data, "type", "profile", "providerId", "region"); err != nil {
			return err
		}
		decoded = value
	case "gcp":
		var value GcpProviderSettingsPatch
		if err := unmarshalResponseObject(data, &value, []string{"type", "providerId"}, []string{"type", "providerId"}); err != nil {
			return err
		}
		if err := validateClosedObjectFields(data, "type", "credentialFile", "location", "projectId", "providerId"); err != nil {
			return err
		}
		decoded = value
	}
	w.Value = decoded
	return nil
}

// ModelProviderError is a schema-tagged union. Use ModelProviderErrorWrapper in JSON carriers.
type ModelProviderError interface{ isModelProviderError() }
type ModelProviderErrorWrapper struct{ Value ModelProviderError }

type ModelProviderMismatchError struct {
	CurrentModelProvider string `json:"currentModelProvider"`
	ModelProvider        string `json:"modelProvider"`
}

func (ModelProviderMismatchError) isModelProviderError() {}

type ModelProviderNotConfiguredError struct {
	ModelProvider string `json:"modelProvider"`
}

func (ModelProviderNotConfiguredError) isModelProviderError() {}

type ModelProviderPolicyConflictError struct {
	ModelProvider         string `json:"modelProvider"`
	RequiredModelProvider string `json:"requiredModelProvider"`
}

func (ModelProviderPolicyConflictError) isModelProviderError() {}

func (w ModelProviderErrorWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil || (reflect.ValueOf(w.Value).Kind() == reflect.Pointer && reflect.ValueOf(w.Value).IsNil()) {
		return nil, fmt.Errorf("nil ModelProviderError")
	}
	var data []byte
	var err error
	switch value := w.Value.(type) {
	case ModelProviderMismatchError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderMismatchError
		}{"modelProviderMismatch", &value})
	case *ModelProviderMismatchError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderMismatchError
		}{"modelProviderMismatch", value})
	case ModelProviderNotConfiguredError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderNotConfiguredError
		}{"modelProviderNotConfigured", &value})
	case *ModelProviderNotConfiguredError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderNotConfiguredError
		}{"modelProviderNotConfigured", value})
	case ModelProviderPolicyConflictError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderPolicyConflictError
		}{"modelProviderPolicyConflict", &value})
	case *ModelProviderPolicyConflictError:
		data, err = jsonencode.Marshal(struct {
			Type string `json:"type"`
			*ModelProviderPolicyConflictError
		}{"modelProviderPolicyConflict", value})
	default:
		data, err = jsonencode.Marshal(w.Value)
	}
	if err != nil {
		return nil, err
	}
	var checked ModelProviderErrorWrapper
	if err = checked.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (w *ModelProviderErrorWrapper) UnmarshalJSON(data []byte) error {
	kind, err := providerUnionKind(data, "type", "modelProviderMismatch", "modelProviderNotConfigured", "modelProviderPolicyConflict")
	if err != nil {
		return err
	}
	var decoded ModelProviderError
	switch kind {
	case "modelProviderMismatch":
		var value ModelProviderMismatchError
		if err := unmarshalResponseObject(data, &value, []string{"type", "currentModelProvider", "modelProvider"}, []string{"type", "currentModelProvider", "modelProvider"}); err != nil {
			return err
		}
		decoded = value
	case "modelProviderNotConfigured":
		var value ModelProviderNotConfiguredError
		if err := unmarshalResponseObject(data, &value, []string{"type", "modelProvider"}, []string{"type", "modelProvider"}); err != nil {
			return err
		}
		decoded = value
	case "modelProviderPolicyConflict":
		var value ModelProviderPolicyConflictError
		if err := unmarshalResponseObject(data, &value, []string{"type", "modelProvider", "requiredModelProvider"}, []string{"type", "modelProvider", "requiredModelProvider"}); err != nil {
			return err
		}
		decoded = value
	}
	w.Value = decoded
	return nil
}

// Reject every invalid recognized discriminator occurrence, including escaped names
// before admitting the final branch. Unknown response properties are open.
func providerUnionKind(data []byte, field string, allowed ...string) (string, error) {
	if err := validateRequiredObjectFields(data, field); err != nil {
		return "", err
	}
	var kind string
	var firstErr error
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if firstErr != nil || !jsonobject.FieldMatches(key, field) {
			return
		}
		var value string
		if err := jsondecode.Unmarshal(raw, &value); err != nil {
			firstErr = err
			return
		}
		valid := false
		for _, candidate := range allowed {
			if candidate == value {
				valid = true
				break
			}
		}
		if !valid {
			firstErr = fmt.Errorf("invalid provider %s %s", field, quotedValueDiagnostic(value))
			return
		}
		kind = value
	})
	return kind, firstErr
}

// providerSummaryAdmission follows native ordered object/array reuse. Required
// member presence is checked after the containing response has finished merging.
// Public summary records keep ordinary JSON composition without decoder methods.
type providerSummaryAdmission struct {
	value ModelProviderSummary
	seen  [4]bool
}

var providerSummaryFields = [...]string{"id", "name", "canEdit", "settings"}

func (s *providerSummaryAdmission) UnmarshalJSON(data []byte) error {
	if err := unmarshalResponseObject(data, &s.value, nil, providerSummaryFields[:]); err != nil {
		return err
	}
	jsonobject.WalkFields(data, true, func(key, _ []byte) {
		for i, field := range providerSummaryFields {
			if jsonobject.FieldMatches(key, field) {
				s.seen[i] = true
				break
			}
		}
	})
	return nil
}
func (s *providerSummaryAdmission) validate() error {
	for i, field := range providerSummaryFields {
		if !s.seen[i] {
			return fmt.Errorf("%w: %s", ErrMissingResultField, field)
		}
	}
	return nil
}

func (r *ModelProviderListResponse) UnmarshalJSON(data []byte) error {
	var value struct {
		CanChangeDefault  bool                       `json:"canChangeDefault"`
		Data              []providerSummaryAdmission `json:"data"`
		DefaultProviderID string                     `json:"defaultProviderId"`
	}
	if err := unmarshalResponseObject(data, &value, []string{"data", "canChangeDefault", "defaultProviderId"}, []string{"data", "canChangeDefault", "defaultProviderId"}); err != nil {
		return err
	}
	output := ModelProviderListResponse{CanChangeDefault: value.CanChangeDefault, DefaultProviderID: value.DefaultProviderID, Data: make([]ModelProviderSummary, len(value.Data))}
	for i := range value.Data {
		if err := value.Data[i].validate(); err != nil {
			return err
		}
		output.Data[i] = value.Data[i].value
	}
	*r = output
	return nil
}
func (r *ModelProviderConfigWriteResponse) UnmarshalJSON(data []byte) error {
	var value struct {
		Provider providerSummaryAdmission `json:"provider"`
	}
	if err := unmarshalResponseObject(data, &value, []string{"provider"}, []string{"provider"}); err != nil {
		return err
	}
	if err := value.Provider.validate(); err != nil {
		return err
	}
	*r = ModelProviderConfigWriteResponse{Provider: value.Provider.value}
	return nil
}
func (p *ModelProviderConfigWriteParams) UnmarshalJSON(data []byte) error {
	type wire ModelProviderConfigWriteParams
	var value wire
	if err := unmarshalResponseObject(data, &value, []string{"settings"}, []string{"settings"}); err != nil {
		return err
	}
	if err := validateClosedObjectFields(data, "settings"); err != nil {
		return err
	}
	*p = ModelProviderConfigWriteParams(value)
	return nil
}
