package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExternalAgentConfigMigrationItemType represents the type of external agent config migration item.
type ExternalAgentConfigMigrationItemType string

const (
	MigrationItemTypeAgentsMd        ExternalAgentConfigMigrationItemType = "AGENTS_MD"
	MigrationItemTypeConfig          ExternalAgentConfigMigrationItemType = "CONFIG"
	MigrationItemTypeSkills          ExternalAgentConfigMigrationItemType = "SKILLS"
	MigrationItemTypeMcpServerConfig ExternalAgentConfigMigrationItemType = "MCP_SERVER_CONFIG"
)

var validExternalAgentConfigMigrationItemTypes = map[ExternalAgentConfigMigrationItemType]struct{}{
	MigrationItemTypeAgentsMd:        {},
	MigrationItemTypeConfig:          {},
	MigrationItemTypeSkills:          {},
	MigrationItemTypeMcpServerConfig: {},
}

func (t *ExternalAgentConfigMigrationItemType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "externalAgentConfig.itemType", validExternalAgentConfigMigrationItemTypes, t)
}

// ExternalAgentConfigMigrationItem represents a detected or imported migration item.
// Null or empty Cwd means home-scoped migration; non-empty means repo-scoped migration.
type ExternalAgentConfigMigrationItem struct {
	Cwd         *string                              `json:"cwd,omitempty"`
	Description string                               `json:"description"`
	ItemType    ExternalAgentConfigMigrationItemType `json:"itemType"`
}

func (i *ExternalAgentConfigMigrationItem) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "description", "itemType"); err != nil {
		return err
	}
	type wire ExternalAgentConfigMigrationItem
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*i = ExternalAgentConfigMigrationItem(decoded)
	return nil
}

// ExternalAgentConfigDetectParams contains parameters for detecting external agent configurations.
type ExternalAgentConfigDetectParams struct {
	Cwds        *[]string `json:"cwds,omitempty"`
	IncludeHome *bool     `json:"includeHome,omitempty"`
}

func (p ExternalAgentConfigDetectParams) prepareRequest() (interface{}, error) {
	if p.Cwds == nil {
		return p, nil
	}

	normalized, err := normalizeAbsolutePathSliceField("cwds", *p.Cwds)
	if err != nil {
		return nil, err
	}
	p.Cwds = &normalized
	return p, nil
}

// ExternalAgentConfigDetectResponse contains the result of config detection.
type ExternalAgentConfigDetectResponse struct {
	Items []ExternalAgentConfigMigrationItem `json:"items"`
}

func (r *ExternalAgentConfigDetectResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "items"); err != nil {
		return err
	}
	type wire ExternalAgentConfigDetectResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ExternalAgentConfigDetectResponse(decoded)
	return nil
}

// ExternalAgentConfigImportParams contains parameters for importing external agent configurations.
type ExternalAgentConfigImportParams struct {
	MigrationItems []ExternalAgentConfigMigrationItem `json:"migrationItems"`
}

func (p ExternalAgentConfigImportParams) prepareRequest() (interface{}, error) {
	if p.MigrationItems == nil {
		return nil, invalidParamsError("migrationItems must not be null")
	}

	for i := range p.MigrationItems {
		cwd := p.MigrationItems[i].Cwd
		if cwd == nil || *cwd == "" {
			continue
		}

		normalized, err := normalizeAbsolutePathField(
			fmt.Sprintf("migrationItems[%d].cwd", i),
			*cwd,
		)
		if err != nil {
			return nil, err
		}
		p.MigrationItems[i].Cwd = &normalized
	}

	return p, nil
}

// ExternalAgentConfigImportResponse is an empty response from config import.
type ExternalAgentConfigImportResponse struct{}

// ExternalAgentService handles external agent configuration detection and import.
type ExternalAgentService struct {
	client *Client
}

func newExternalAgentService(client *Client) *ExternalAgentService {
	return &ExternalAgentService{client: client}
}

// ConfigDetect detects external agent configurations in specified directories.
func (s *ExternalAgentService) ConfigDetect(ctx context.Context, params ExternalAgentConfigDetectParams) (ExternalAgentConfigDetectResponse, error) {
	var resp ExternalAgentConfigDetectResponse
	if err := s.client.sendRequest(ctx, methodExternalAgentConfigDetect, params, &resp); err != nil {
		return ExternalAgentConfigDetectResponse{}, err
	}
	return resp, nil
}

// ConfigImport imports detected external agent configurations.
func (s *ExternalAgentService) ConfigImport(ctx context.Context, params ExternalAgentConfigImportParams) (ExternalAgentConfigImportResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodExternalAgentConfigImport, params); err != nil {
		return ExternalAgentConfigImportResponse{}, err
	}
	return ExternalAgentConfigImportResponse{}, nil
}
