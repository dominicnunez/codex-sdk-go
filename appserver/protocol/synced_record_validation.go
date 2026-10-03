package protocol

// Synced records validate at their shared decoding owner, before services or
// typed notification handlers can publish a value. Failed decodes leave the
// receiver unchanged; successful decodes replace it, including omitted fields.

func (v *AppsReadResponse) UnmarshalJSON(data []byte) error {
	type wire AppsReadResponse
	var decoded struct {
		wire
		MissingAppIDs nonNullStringList `json:"missingAppIds"`
	}
	if err := unmarshalSyncedResponse(data, &decoded, []string{"apps", "missingAppIds"}, []string{"apps", "missingAppIds"}); err != nil {
		return err
	}
	decoded.wire.MissingAppIDs = []string(decoded.MissingAppIDs)
	*v = AppsReadResponse(decoded.wire)
	return nil
}

func (v *AppsInstalledResponse) UnmarshalJSON(data []byte) error {
	type wire AppsInstalledResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"apps"}, []string{"apps"}); err != nil {
		return err
	}
	*v = AppsInstalledResponse(decoded)
	return nil
}

func (v *GetWorkspaceMessagesResponse) UnmarshalJSON(data []byte) error {
	type wire GetWorkspaceMessagesResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"featureEnabled", "messages"}, []string{"featureEnabled", "messages"}); err != nil {
		return err
	}
	*v = GetWorkspaceMessagesResponse(decoded)
	return nil
}

func (v *ThreadGoalGetResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadGoalGetResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, nil, nil); err != nil {
		return err
	}
	*v = ThreadGoalGetResponse(decoded)
	return nil
}

func (v *ThreadGoalSetResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadGoalSetResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"goal"}, []string{"goal"}); err != nil {
		return err
	}
	*v = ThreadGoalSetResponse(decoded)
	return nil
}

func (v *ThreadGoalClearResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadGoalClearResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"cleared"}, []string{"cleared"}); err != nil {
		return err
	}
	*v = ThreadGoalClearResponse(decoded)
	return nil
}

func (v *ThreadSectionCreateResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadSectionCreateResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"section"}, []string{"section"}); err != nil {
		return err
	}
	*v = ThreadSectionCreateResponse(decoded)
	return nil
}

func (v *ThreadSectionListResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadSectionListResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"data"}, []string{"data"}); err != nil {
		return err
	}
	*v = ThreadSectionListResponse(decoded)
	return nil
}

func (v *ThreadSectionUpdateResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadSectionUpdateResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"section"}, []string{"section"}); err != nil {
		return err
	}
	*v = ThreadSectionUpdateResponse(decoded)
	return nil
}

func (v *ExternalAgentConfigImportHistoryRecordResponse) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportHistoryRecordResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"importId"}, []string{"importId"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportHistoryRecordResponse(decoded)
	return nil
}

func (v *ExternalAgentConfigImportHistoriesReadResponse) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportHistoriesReadResponse
	var decoded wire
	if err := unmarshalSyncedResponse(data, &decoded, []string{"connectors", "data"}, []string{"connectors", "data"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportHistoriesReadResponse(decoded)
	return nil
}

func (v *ConnectorMetadata) UnmarshalJSON(data []byte) error {
	type wire ConnectorMetadata
	var decoded struct {
		wire
		PluginDisplayNames nonNullStringList `json:"pluginDisplayNames"`
	}
	if err := unmarshalSyncedInbound(data, &decoded, []string{"id", "name"}, []string{"id", "name", "pluginDisplayNames"}); err != nil {
		return err
	}
	decoded.wire.PluginDisplayNames = []string(decoded.PluginDisplayNames)
	*v = ConnectorMetadata(decoded.wire)
	return nil
}

func (v *AppToolSummary) UnmarshalJSON(data []byte) error {
	type wire AppToolSummary
	decoded := wire{IsEnabled: true}
	if err := unmarshalSyncedInbound(data, &decoded, []string{"description", "name"}, []string{"description", "name", "isEnabled", "isReadOnly"}); err != nil {
		return err
	}
	*v = AppToolSummary(decoded)
	return nil
}

func (v *InstalledApp) UnmarshalJSON(data []byte) error {
	type wire InstalledApp
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"callable", "enabled", "id"}, []string{"callable", "enabled", "id"}); err != nil {
		return err
	}
	*v = InstalledApp(decoded)
	return nil
}

func (v *WorkspaceMessage) UnmarshalJSON(data []byte) error {
	type wire WorkspaceMessage
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"messageBody", "messageId", "messageType"}, []string{"messageBody", "messageId", "messageType"}); err != nil {
		return err
	}
	*v = WorkspaceMessage(decoded)
	return nil
}

func (v *AccountTokenUsageDailyBucket) UnmarshalJSON(data []byte) error {
	type wire AccountTokenUsageDailyBucket
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"startDate", "tokens"}, []string{"startDate", "tokens"}); err != nil {
		return err
	}
	*v = AccountTokenUsageDailyBucket(decoded)
	return nil
}

func (v *ThreadUsage) UnmarshalJSON(data []byte) error {
	type wire ThreadUsage
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"estimatedUsageCreditsMicros", "groups", "threadId"}, []string{"estimatedUsageCreditsMicros", "groups", "threadId"}); err != nil {
		return err
	}
	*v = ThreadUsage(decoded)
	return nil
}

func (v *ThreadUsageBreakdownGroup) UnmarshalJSON(data []byte) error {
	type wire ThreadUsageBreakdownGroup
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"estimatedUsageCreditsMicros"}, []string{"estimatedUsageCreditsMicros"}); err != nil {
		return err
	}
	*v = ThreadUsageBreakdownGroup(decoded)
	return nil
}

func (v *ThreadGoal) UnmarshalJSON(data []byte) error {
	type wire ThreadGoal
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"createdAt", "objective", "status", "threadId", "timeUsedSeconds", "tokensUsed", "updatedAt"}, []string{"createdAt", "objective", "status", "threadId", "timeUsedSeconds", "tokensUsed", "updatedAt"}); err != nil {
		return err
	}
	*v = ThreadGoal(decoded)
	return nil
}

func (v *ExternalAgentConfigImportItemTypeFailure) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportItemTypeFailure
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"failureStage", "itemType", "message"}, []string{"failureStage", "itemType", "message"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportItemTypeFailure(decoded)
	return nil
}

func (v *ExternalAgentConfigImportItemTypeSuccess) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportItemTypeSuccess
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"itemType"}, []string{"itemType"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportItemTypeSuccess(decoded)
	return nil
}

func (v *ExternalAgentConfigImportTypeResult) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportTypeResult
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"failures", "itemType", "successes"}, []string{"failures", "itemType", "successes"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportTypeResult(decoded)
	return nil
}

func (v *ExternalAgentConfigImportHistory) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportHistory
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"completedAtMs", "failures", "importId", "successes"}, []string{"completedAtMs", "failures", "importId", "successes"}); err != nil {
		return err
	}
	*v = ExternalAgentConfigImportHistory(decoded)
	return nil
}

func (v *ExternalAgentImportedConnectorCandidate) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentImportedConnectorCandidate
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"name", "sessionCount", "source"}, []string{"name", "sessionCount", "source"}); err != nil {
		return err
	}
	*v = ExternalAgentImportedConnectorCandidate(decoded)
	return nil
}

func (v *McpToolCallAppContext) UnmarshalJSON(data []byte) error {
	type wire McpToolCallAppContext
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"connectorId"}, []string{"connectorId"}); err != nil {
		return err
	}
	*v = McpToolCallAppContext(decoded)
	return nil
}

func (v *RawResponseCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire RawResponseCompletedNotification
	var decoded wire
	if err := unmarshalSyncedInbound(data, &decoded, []string{"responseId", "threadId", "turnId"}, []string{"responseId", "threadId", "turnId"}); err != nil {
		return err
	}
	*v = RawResponseCompletedNotification(decoded)
	return nil
}

func (v *WorkspaceMessageType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "workspaceMessage.messageType", map[WorkspaceMessageType]struct{}{WorkspaceMessageTypeHeadline: {}, WorkspaceMessageTypeAnnouncement: {}, WorkspaceMessageTypeUnknown: {}}, v)
}

func (v *ThreadGoalStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "threadGoal.status", map[ThreadGoalStatus]struct{}{ThreadGoalStatusActive: {}, ThreadGoalStatusPaused: {}, ThreadGoalStatusBlocked: {}, ThreadGoalStatusUsageLimited: {}, ThreadGoalStatusBudgetLimited: {}, ThreadGoalStatusComplete: {}}, v)
}

func (v *ExternalAgentImportedConnectorSource) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "importedConnector.source", map[ExternalAgentImportedConnectorSource]struct{}{ExternalAgentImportedConnectorSourceRemoteMcpServersConfig: {}}, v)
}

func (v *ProjectChangeType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "project.changeType", map[ProjectChangeType]struct{}{ProjectChangeTypeCreated: {}, ProjectChangeTypeUpdated: {}, ProjectChangeTypeDeleted: {}}, v)
}
