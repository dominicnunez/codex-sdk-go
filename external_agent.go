package codex

import "context"

// ExternalAgentConfigMigrationItemType represents the type of external agent config migration item.
type ExternalAgentConfigMigrationItemType string

const (
	MigrationItemTypeAgentsMd        ExternalAgentConfigMigrationItemType = "AGENTS_MD"
	MigrationItemTypeConfig          ExternalAgentConfigMigrationItemType = "CONFIG"
	MigrationItemTypeSkills          ExternalAgentConfigMigrationItemType = "SKILLS"
	MigrationItemTypeMcpServerConfig ExternalAgentConfigMigrationItemType = "MCP_SERVER_CONFIG"
)

// ExternalAgentConfigMigrationItem represents a detected or imported migration item.
// Null or empty Cwd means home-scoped migration; non-empty means repo-scoped migration.
type ExternalAgentConfigMigrationItem struct {
	Cwd         *string                                  `json:"cwd,omitempty"`
	Description string                                   `json:"description"`
	ItemType    ExternalAgentConfigMigrationItemType     `json:"itemType"`
}

// ExternalAgentConfigDetectParams contains parameters for detecting external agent configurations.
type ExternalAgentConfigDetectParams struct {
	Cwds        *[]string `json:"cwds,omitempty"`
	IncludeHome *bool     `json:"includeHome,omitempty"`
}

// ExternalAgentConfigDetectResponse contains the result of config detection.
type ExternalAgentConfigDetectResponse struct {
	Items []ExternalAgentConfigMigrationItem `json:"items"`
}

// ExternalAgentConfigImportParams contains parameters for importing external agent configurations.
type ExternalAgentConfigImportParams struct {
	MigrationItems []ExternalAgentConfigMigrationItem `json:"migrationItems"`
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
	if err := s.client.sendRequest(ctx, "externalAgentConfig/detect", params, &resp); err != nil {
		return ExternalAgentConfigDetectResponse{}, err
	}
	return resp, nil
}

// ConfigImport imports detected external agent configurations.
func (s *ExternalAgentService) ConfigImport(ctx context.Context, params ExternalAgentConfigImportParams) (ExternalAgentConfigImportResponse, error) {
	if err := s.client.sendRequest(ctx, "externalAgentConfig/import", params, nil); err != nil {
		return ExternalAgentConfigImportResponse{}, err
	}
	return ExternalAgentConfigImportResponse{}, nil
}
