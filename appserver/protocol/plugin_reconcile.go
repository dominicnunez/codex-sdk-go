package protocol

import (
	"context"
	"encoding/json"
)

const methodPluginReconcile = "plugin/reconcile"

// PluginReconcileParams optionally records a reason for reconciling plugins.
type PluginReconcileParams struct {
	Reason *string `json:"reason,omitempty"`
}

// PluginReconcileChangedPlugin identifies runtime categories affected by a change.
type PluginReconcileChangedPlugin struct {
	ID        string `json:"id"`
	HasApps   bool   `json:"hasApps"`
	HasHooks  bool   `json:"hasHooks"`
	HasMcps   bool   `json:"hasMcps"`
	HasSkills bool   `json:"hasSkills"`
}

func (p *PluginReconcileChangedPlugin) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "id", "hasApps", "hasHooks", "hasMcps", "hasSkills"); err != nil {
		return err
	}
	type wire PluginReconcileChangedPlugin
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = PluginReconcileChangedPlugin(decoded)
	return nil
}

// PluginReconcileResponse describes changes observed in this reconciliation pass.
// It is not an acknowledgement that affected runtimes are ready.
type PluginReconcileResponse struct {
	ChangedPlugins                       []PluginReconcileChangedPlugin `json:"changedPlugins"`
	FailedMaterializationRemotePluginIDs []string                       `json:"failedMaterializationRemotePluginIds"`
	FailedRemotePluginIDs                []string                       `json:"failedRemotePluginIds"`
}

func (r *PluginReconcileResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "changedPlugins", "failedMaterializationRemotePluginIds", "failedRemotePluginIds"); err != nil {
		return err
	}
	type wire PluginReconcileResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = PluginReconcileResponse(decoded)
	return nil
}

// Reconcile refreshes plugin bundles and installed state.
func (s *PluginService) Reconcile(ctx context.Context, params PluginReconcileParams) (PluginReconcileResponse, error) {
	var response PluginReconcileResponse
	err := s.client.sendRequest(ctx, methodPluginReconcile, params, &response)
	return response, err
}
