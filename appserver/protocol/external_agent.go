package protocol

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
)

// ExternalAgentConfigMigrationItemType represents the type of external agent config migration item.
type ExternalAgentConfigMigrationItemType string

const (
	MigrationItemTypeAgentsMd        ExternalAgentConfigMigrationItemType = "AGENTS_MD"
	MigrationItemTypeConfig          ExternalAgentConfigMigrationItemType = "CONFIG"
	MigrationItemTypeSkills          ExternalAgentConfigMigrationItemType = "SKILLS"
	MigrationItemTypeMcpServerConfig ExternalAgentConfigMigrationItemType = "MCP_SERVER_CONFIG"
	MigrationItemTypePlugins         ExternalAgentConfigMigrationItemType = "PLUGINS"
	MigrationItemTypeSubagents       ExternalAgentConfigMigrationItemType = "SUBAGENTS"
	MigrationItemTypeHooks           ExternalAgentConfigMigrationItemType = "HOOKS"
	MigrationItemTypeCommands        ExternalAgentConfigMigrationItemType = "COMMANDS"
	MigrationItemTypeSessions        ExternalAgentConfigMigrationItemType = "SESSIONS"
	MigrationItemTypeMemory          ExternalAgentConfigMigrationItemType = "MEMORY"
)

var validExternalAgentConfigMigrationItemTypes = map[ExternalAgentConfigMigrationItemType]struct{}{
	MigrationItemTypeAgentsMd:        {},
	MigrationItemTypeConfig:          {},
	MigrationItemTypeSkills:          {},
	MigrationItemTypeMcpServerConfig: {},
	MigrationItemTypePlugins:         {},
	MigrationItemTypeSubagents:       {},
	MigrationItemTypeHooks:           {},
	MigrationItemTypeCommands:        {},
	MigrationItemTypeSessions:        {},
	MigrationItemTypeMemory:          {},
}

func (t *ExternalAgentConfigMigrationItemType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "externalAgentConfig.itemType", validExternalAgentConfigMigrationItemTypes, t)
}

// ExternalAgentConfigMigrationItem represents a detected or imported migration item.
// Null or empty Cwd means home-scoped migration; non-empty means repo-scoped migration.
type ExternalAgentConfigMigrationItem struct {
	Cwd         *string                              `json:"cwd,omitempty"`
	Description string                               `json:"description"`
	Details     *MigrationDetails                    `json:"details,omitempty"`
	ItemType    ExternalAgentConfigMigrationItemType `json:"itemType"`
}

func (i *ExternalAgentConfigMigrationItem) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "description", "itemType"); err != nil {
		return err
	}
	type wire ExternalAgentConfigMigrationItem
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Cwd != nil && *decoded.Cwd != "" {
		validatedCwd, err := validateInboundAbsolutePathPointerField("externalAgentConfig.cwd", decoded.Cwd)
		if err != nil {
			return err
		}
		decoded.Cwd = validatedCwd
	}
	*i = ExternalAgentConfigMigrationItem(decoded)
	return nil
}

// ExternalAgentConfigDetectParams contains parameters for detecting external agent configurations.
type ExternalAgentConfigDetectParams struct {
	Cwds              *[]string `json:"cwds,omitempty"`
	IncludeHome       *bool     `json:"includeHome,omitempty"`
	MaxSessionAgeDays *uint32   `json:"maxSessionAgeDays,omitempty"`
	MaxSessions       *uint32   `json:"maxSessions,omitempty"`
	MigrationSource   *string   `json:"migrationSource,omitempty"`
	Source            *string   `json:"source,omitempty"`
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
	// Connectors retains its legacy public type. Detection admits both source
	// variants from ExternalAgentDetectedConnectorSource at this envelope owner.
	Connectors []ExternalAgentImportedConnectorCandidate `json:"connectors,omitempty"`
}

func (r *ExternalAgentConfigDetectResponse) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigDetectResponse
	var decoded struct {
		wire
		Connectors []detectedConnector `json:"connectors"`
	}
	if err := unmarshalResponseObject(data, &decoded, []string{"items"}, []string{"items", "connectors"}); err != nil {
		return err
	}
	if decoded.Connectors != nil {
		decoded.wire.Connectors = make([]ExternalAgentImportedConnectorCandidate, len(decoded.Connectors))
		for i, connector := range decoded.Connectors {
			decoded.wire.Connectors[i] = ExternalAgentImportedConnectorCandidate{Name: connector.Name, SessionCount: connector.SessionCount, Source: ExternalAgentImportedConnectorSource(connector.Source)}
		}
	}
	*r = ExternalAgentConfigDetectResponse(decoded.wire)
	return nil
}

// Detection and import history have different source enums despite the legacy
// shared public field type. Do not validate detected records as imported ones.
type detectedConnector struct {
	Name         string                  `json:"name"`
	SessionCount uint32                  `json:"sessionCount"`
	Source       detectedConnectorSource `json:"source"`
}

type detectedConnectorSource string

func (v *detectedConnectorSource) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "detectedConnector.source", map[detectedConnectorSource]struct{}{"remoteMcpServersConfig": {}, "sessionToolUse": {}}, v)
}

func (v *detectedConnector) UnmarshalJSON(data []byte) error {
	type wire detectedConnector
	var decoded wire
	required := []string{"name", "sessionCount", "source"}
	if err := unmarshalSyncedInbound(data, &decoded, required, required); err != nil {
		return err
	}
	*v = detectedConnector(decoded)
	return nil
}

// ExternalAgentConfigImportParams contains parameters for importing external agent configurations.
type ExternalAgentConfigImportParams struct {
	MigrationItems  []ExternalAgentConfigMigrationItem `json:"migrationItems"`
	MigrationSource *string                            `json:"migrationSource,omitempty"`
	ProviderID      *string                            `json:"providerId,omitempty"`
	Source          *string                            `json:"source,omitempty"`
}

func (p ExternalAgentConfigImportParams) prepareRequest() (interface{}, error) {
	if p.MigrationItems == nil {
		return nil, invalidParamsError("migrationItems must not be null")
	}

	for i := range p.MigrationItems {
		if err := validateEnumValue(
			"externalAgentConfig.itemType",
			p.MigrationItems[i].ItemType,
			validExternalAgentConfigMigrationItemTypes,
		); err != nil {
			return nil, invalidParamsError("migrationItems[%d].itemType: %v", i, err)
		}

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

// ExternalAgentConfigImportResponse identifies the asynchronous import operation.
type ExternalAgentConfigImportResponse struct {
	ImportID string `json:"importId"`
}

func (r *ExternalAgentConfigImportResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "importId"); err != nil {
		return err
	}
	type wire ExternalAgentConfigImportResponse
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ExternalAgentConfigImportResponse(decoded)
	return nil
}

// ExternalAgentConfigImportCompletedNotification is sent when config import completes.
type ExternalAgentConfigImportCompletedNotification struct {
	ImportID        string                                `json:"importId"`
	ItemTypeResults []ExternalAgentConfigImportTypeResult `json:"itemTypeResults"`
}

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
	var resp ExternalAgentConfigImportResponse
	if err := s.client.sendRequest(ctx, methodExternalAgentConfigImport, params, &resp); err != nil {
		return ExternalAgentConfigImportResponse{}, err
	}
	return resp, nil
}

// OnExternalAgentConfigImportCompleted registers a listener for config import completion notifications.
func (c *Client) OnExternalAgentConfigImportCompleted(handler func(ExternalAgentConfigImportCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyExternalAgentConfigImportCompleted, nil)
		return
	}
	c.OnNotification(notifyExternalAgentConfigImportCompleted, func(ctx context.Context, notif Notification) {
		var params ExternalAgentConfigImportCompletedNotification
		if err := jsondecode.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyExternalAgentConfigImportCompleted, fmt.Errorf("unmarshal %s: %w", notifyExternalAgentConfigImportCompleted, err))
			return
		}
		handler(params)
	})
}
