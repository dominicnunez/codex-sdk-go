package protocol

import (
	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

// PackagedDefaultsConfigLayerSource identifies the installed package's defaults.
type PackagedDefaultsConfigLayerSource struct {
	File string `json:"file"`
}

func (PackagedDefaultsConfigLayerSource) isConfigLayerSource() {}

func (s *PackagedDefaultsConfigLayerSource) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type", "file"); err != nil {
		return err
	}
	type wire PackagedDefaultsConfigLayerSource
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if _, err := validateInboundAbsolutePathField("config.layer.packagedDefaults.file", decoded.File); err != nil {
		return err
	}
	if err := validateFields(data, func(_ string, raw []byte) error {
		var file string
		if err := jsondecode.Unmarshal(raw, &file); err != nil {
			return err
		}
		_, err := validateInboundAbsolutePathField("config.layer.packagedDefaults.file", file)
		return err
	}, false, "file"); err != nil {
		return err
	}
	*s = PackagedDefaultsConfigLayerSource(decoded)
	return nil
}

func (s PackagedDefaultsConfigLayerSource) MarshalJSON() ([]byte, error) {
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		File string `json:"file"`
	}{Type: "packagedDefaults", File: s.File})
}

// EnterpriseManagedConfigLayerSource identifies a delivered enterprise policy.
type EnterpriseManagedConfigLayerSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (EnterpriseManagedConfigLayerSource) isConfigLayerSource() {}

func (s *EnterpriseManagedConfigLayerSource) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "type", "id", "name"); err != nil {
		return err
	}
	type wire EnterpriseManagedConfigLayerSource
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = EnterpriseManagedConfigLayerSource(decoded)
	return nil
}

func (s EnterpriseManagedConfigLayerSource) MarshalJSON() ([]byte, error) {
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}{Type: "enterpriseManaged", ID: s.ID, Name: s.Name})
}
