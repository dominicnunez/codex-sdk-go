package codex

import (
	"context"
	"encoding/json"
)

// HookTrustStatus is the trust state of a configured hook.
type HookTrustStatus string

const (
	HookTrustStatusManaged   HookTrustStatus = "managed"
	HookTrustStatusUntrusted HookTrustStatus = "untrusted"
	HookTrustStatusTrusted   HookTrustStatus = "trusted"
	HookTrustStatusModified  HookTrustStatus = "modified"
)

var validHookTrustStatuses = map[HookTrustStatus]struct{}{
	HookTrustStatusManaged:   {},
	HookTrustStatusUntrusted: {},
	HookTrustStatusTrusted:   {},
	HookTrustStatusModified:  {},
}

func (s *HookTrustStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "hook.trustStatus", validHookTrustStatuses, s)
}

// HooksListParams lists configured hooks for working directories.
type HooksListParams struct {
	Cwds []string `json:"cwds,omitempty"`
}

// HookErrorInfo describes a hook configuration error.
type HookErrorInfo struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

// HookMetadata describes a configured hook.
type HookMetadata struct {
	Command       *string         `json:"command,omitempty"`
	CurrentHash   string          `json:"currentHash"`
	DisplayOrder  int64           `json:"displayOrder"`
	Enabled       bool            `json:"enabled"`
	EventName     HookEventName   `json:"eventName"`
	HandlerType   HookHandlerType `json:"handlerType"`
	IsManaged     bool            `json:"isManaged"`
	Key           string          `json:"key"`
	Matcher       *string         `json:"matcher,omitempty"`
	PluginID      *string         `json:"pluginId,omitempty"`
	Source        HookSource      `json:"source"`
	SourcePath    string          `json:"sourcePath"`
	StatusMessage *string         `json:"statusMessage,omitempty"`
	TimeoutSec    uint64          `json:"timeoutSec"`
	TrustStatus   HookTrustStatus `json:"trustStatus"`
}

func (m *HookMetadata) UnmarshalJSON(data []byte) error {
	type wire HookMetadata
	var decoded wire
	required := []string{
		"currentHash",
		"displayOrder",
		"enabled",
		"eventName",
		"handlerType",
		"isManaged",
		"key",
		"source",
		"sourcePath",
		"timeoutSec",
		"trustStatus",
	}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	validatedSourcePath, err := validateInboundAbsolutePathField("hook.sourcePath", decoded.SourcePath)
	if err != nil {
		return err
	}
	decoded.SourcePath = validatedSourcePath
	*m = HookMetadata(decoded)
	return nil
}

// HooksListEntry groups hooks, warnings, and errors for a cwd.
type HooksListEntry struct {
	Cwd      string          `json:"cwd"`
	Errors   []HookErrorInfo `json:"errors"`
	Hooks    []HookMetadata  `json:"hooks"`
	Warnings []string        `json:"warnings"`
}

func (e *HooksListEntry) UnmarshalJSON(data []byte) error {
	type wire HooksListEntry
	var decoded wire
	required := []string{"cwd", "errors", "hooks", "warnings"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*e = HooksListEntry(decoded)
	return nil
}

// HooksListResponse contains configured hooks by working directory.
type HooksListResponse struct {
	Data []HooksListEntry `json:"data"`
}

func (r *HooksListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire HooksListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = HooksListResponse(decoded)
	return nil
}

// HooksService provides hook inspection operations.
type HooksService struct {
	client *Client
}

func newHooksService(client *Client) *HooksService {
	return &HooksService{client: client}
}

// List lists configured hooks.
func (s *HooksService) List(ctx context.Context, params HooksListParams) (HooksListResponse, error) {
	var resp HooksListResponse
	if err := s.client.sendRequest(ctx, methodHooksList, params, &resp); err != nil {
		return HooksListResponse{}, err
	}
	return resp, nil
}
