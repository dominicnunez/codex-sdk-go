package protocol

import (
	"context"
	"encoding/json"
	"fmt"
)

// AppsReadParams contains parameters for app/read.
type AppsReadParams struct {
	AppIDs       []string `json:"appIds"`
	IncludeTools bool     `json:"includeTools,omitempty"`
	ThreadID     *string  `json:"threadId,omitempty"`
}

// AppToolSummary is display-only metadata for an app tool.
type AppToolSummary struct {
	Description    string  `json:"description"`
	DisabledReason *string `json:"disabledReason,omitempty"`
	IsEnabled      bool    `json:"isEnabled,omitempty"`
	IsReadOnly     bool    `json:"isReadOnly,omitempty"`
	Name           string  `json:"name"`
	Title          *string `json:"title,omitempty"`
}

// ConnectorMetadata is metadata returned by app/read.
type ConnectorMetadata struct {
	Description         *string           `json:"description,omitempty"`
	DistributionChannel *string           `json:"distributionChannel,omitempty"`
	IconURL             *string           `json:"iconUrl,omitempty"`
	IconURLDark         *string           `json:"iconUrlDark,omitempty"`
	ID                  string            `json:"id"`
	InstallURL          *string           `json:"installUrl,omitempty"`
	Name                string            `json:"name"`
	PluginDisplayNames  []string          `json:"pluginDisplayNames,omitempty"`
	ToolSummaries       *[]AppToolSummary `json:"toolSummaries,omitempty"`
}

// AppsReadResponse contains app/read results.
type AppsReadResponse struct {
	Apps          []ConnectorMetadata `json:"apps"`
	MissingAppIDs []string            `json:"missingAppIds"`
}

// AppsInstalledParams contains parameters for app/installed.
type AppsInstalledParams struct {
	ForceRefresh bool    `json:"forceRefresh,omitempty"`
	ThreadID     *string `json:"threadId,omitempty"`
}

// InstalledApp is the committed runtime state of an installed connector.
type InstalledApp struct {
	Callable    bool    `json:"callable"`
	Enabled     bool    `json:"enabled"`
	ID          string  `json:"id"`
	RuntimeName *string `json:"runtimeName,omitempty"`
}

// AppsInstalledResponse contains the installed connector snapshot.
type AppsInstalledResponse struct {
	Apps []InstalledApp `json:"apps"`
}

// Read returns metadata for specific apps/connectors.
func (s *AppsService) Read(ctx context.Context, params AppsReadParams) (AppsReadResponse, error) {
	var resp AppsReadResponse
	err := s.client.sendRequest(ctx, methodAppRead, params, &resp)
	return resp, err
}

// Installed returns the committed installed connector snapshot.
func (s *AppsService) Installed(ctx context.Context, params AppsInstalledParams) (AppsInstalledResponse, error) {
	var resp AppsInstalledResponse
	err := s.client.sendRequest(ctx, methodAppInstalled, params, &resp)
	return resp, err
}

// ConsumeAccountRateLimitResetCreditParams contains a reset-credit redemption request.
type ConsumeAccountRateLimitResetCreditParams struct {
	CreditID       *string `json:"creditId,omitempty"`
	IdempotencyKey string  `json:"idempotencyKey"`
}

// ConsumeAccountRateLimitResetCreditOutcome is the result of redeeming a reset credit.
type ConsumeAccountRateLimitResetCreditOutcome string

const (
	ConsumeAccountRateLimitResetCreditOutcomeReset           ConsumeAccountRateLimitResetCreditOutcome = "reset"
	ConsumeAccountRateLimitResetCreditOutcomeNothingToReset  ConsumeAccountRateLimitResetCreditOutcome = "nothingToReset"
	ConsumeAccountRateLimitResetCreditOutcomeNoCredit        ConsumeAccountRateLimitResetCreditOutcome = "noCredit"
	ConsumeAccountRateLimitResetCreditOutcomeAlreadyRedeemed ConsumeAccountRateLimitResetCreditOutcome = "alreadyRedeemed"
)

// ConsumeAccountRateLimitResetCreditResponse contains the redemption outcome.
type ConsumeAccountRateLimitResetCreditResponse struct {
	Outcome ConsumeAccountRateLimitResetCreditOutcome `json:"outcome"`
}

// GetAccountTokenUsageParams selects optional thread-scoped token usage.
type GetAccountTokenUsageParams struct {
	ThreadID *string `json:"threadId,omitempty"`
}

// NullableGetAccountTokenUsageParams is the nullable account/usage/read parameter shape.
type NullableGetAccountTokenUsageParams = GetAccountTokenUsageParams

type AccountTokenUsageDailyBucket struct {
	StartDate string `json:"startDate"`
	Tokens    int64  `json:"tokens"`
}

type AccountTokenUsageSummary struct {
	CurrentStreakDays     *int64 `json:"currentStreakDays,omitempty"`
	LifetimeTokens        *int64 `json:"lifetimeTokens,omitempty"`
	LongestRunningTurnSec *int64 `json:"longestRunningTurnSec,omitempty"`
	LongestStreakDays     *int64 `json:"longestStreakDays,omitempty"`
	PeakDailyTokens       *int64 `json:"peakDailyTokens,omitempty"`
}

type ThreadUsageBreakdownGroup struct {
	CachedInputTokens           *int64  `json:"cachedInputTokens,omitempty"`
	EstimatedUsageCreditsMicros int64   `json:"estimatedUsageCreditsMicros"`
	InputTokens                 *int64  `json:"inputTokens,omitempty"`
	Model                       *string `json:"model,omitempty"`
	NetNewInputTokens           *int64  `json:"netNewInputTokens,omitempty"`
	OutputTokens                *int64  `json:"outputTokens,omitempty"`
	ReasoningEffort             *string `json:"reasoningEffort,omitempty"`
	Speed                       *string `json:"speed,omitempty"`
	TotalTokens                 *int64  `json:"totalTokens,omitempty"`
}

type ThreadUsage struct {
	EstimatedUsageCreditsMicros int64                       `json:"estimatedUsageCreditsMicros"`
	EstimatedUsageUSDMicros     *int64                      `json:"estimatedUsageUsdMicros,omitempty"`
	Groups                      []ThreadUsageBreakdownGroup `json:"groups"`
	ThreadID                    string                      `json:"threadId"`
}

// GetAccountTokenUsageResponse contains account activity and optional thread estimates.
type GetAccountTokenUsageResponse struct {
	DailyUsageBuckets *[]AccountTokenUsageDailyBucket `json:"dailyUsageBuckets,omitempty"`
	Summary           AccountTokenUsageSummary        `json:"summary"`
	ThreadUsage       *ThreadUsage                    `json:"threadUsage,omitempty"`
}

type WorkspaceMessageType string

const (
	WorkspaceMessageTypeHeadline     WorkspaceMessageType = "headline"
	WorkspaceMessageTypeAnnouncement WorkspaceMessageType = "announcement"
	WorkspaceMessageTypeUnknown      WorkspaceMessageType = "unknown"
)

type WorkspaceMessage struct {
	ArchivedAt  *int64               `json:"archivedAt,omitempty"`
	CreatedAt   *int64               `json:"createdAt,omitempty"`
	MessageBody string               `json:"messageBody"`
	MessageID   string               `json:"messageId"`
	MessageType WorkspaceMessageType `json:"messageType"`
}

// GetWorkspaceMessagesResponse contains active workspace messages.
type GetWorkspaceMessagesResponse struct {
	FeatureEnabled bool               `json:"featureEnabled"`
	Messages       []WorkspaceMessage `json:"messages"`
}

func (s *AccountService) ConsumeRateLimitResetCredit(ctx context.Context, params ConsumeAccountRateLimitResetCreditParams) (ConsumeAccountRateLimitResetCreditResponse, error) {
	var resp ConsumeAccountRateLimitResetCreditResponse
	err := s.client.sendRequest(ctx, methodAccountRateLimitResetCreditConsume, params, &resp)
	return resp, err
}

func (s *AccountService) GetTokenUsage(ctx context.Context, params *GetAccountTokenUsageParams) (GetAccountTokenUsageResponse, error) {
	var resp GetAccountTokenUsageResponse
	err := s.client.sendRequest(ctx, methodAccountUsageRead, params, &resp)
	return resp, err
}

func (s *AccountService) GetWorkspaceMessages(ctx context.Context) (GetWorkspaceMessagesResponse, error) {
	var resp GetWorkspaceMessagesResponse
	err := s.client.sendRequest(ctx, methodAccountWorkspaceMessagesRead, nil, &resp)
	return resp, err
}

// Thread goal requests.
type ThreadGoalGetParams struct {
	ThreadID string `json:"threadId"`
}
type ThreadGoalGetResponse struct {
	Goal *ThreadGoal `json:"goal,omitempty"`
}
type ThreadGoalSetParams struct {
	Objective   *string           `json:"objective,omitempty"`
	Status      *ThreadGoalStatus `json:"status,omitempty"`
	ThreadID    string            `json:"threadId"`
	TokenBudget *int64            `json:"tokenBudget,omitempty"`
}
type ThreadGoalSetResponse struct {
	Goal ThreadGoal `json:"goal"`
}
type ThreadGoalClearParams struct {
	ThreadID string `json:"threadId"`
}
type ThreadGoalClearResponse struct {
	Cleared bool `json:"cleared"`
}

// Thread deletion requests.
type ThreadDeleteParams struct {
	ThreadID string `json:"threadId"`
}
type ThreadDeleteResponse struct{}

// Thread section models and requests.
type ThreadSectionAppearance struct {
	Color *string `json:"color,omitempty"`
	Icon  *string `json:"icon,omitempty"`
}

type ThreadSection struct {
	Appearance *ThreadSectionAppearance `json:"appearance,omitempty"`
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
}

type ThreadSectionCreateParams struct {
	Appearance *ThreadSectionAppearance `json:"appearance,omitempty"`
	Name       string                   `json:"name"`
}
type ThreadSectionCreateResponse struct {
	Section ThreadSection `json:"section"`
}
type ThreadSectionDeleteParams struct {
	SectionID string `json:"sectionId"`
}
type ThreadSectionDeleteResponse struct{}
type ThreadSectionListParams struct {
	Cursor *string `json:"cursor,omitempty"`
	Limit  *uint32 `json:"limit,omitempty"`
}
type ThreadSectionListResponse struct {
	Data       []ThreadSection `json:"data"`
	NextCursor *string         `json:"nextCursor,omitempty"`
}
type ThreadSectionUpdateParams struct {
	Appearance json.RawMessage `json:"appearance,omitempty"`
	Name       string          `json:"name"`
	SectionID  string          `json:"sectionId"`
}
type ThreadSectionUpdateResponse struct {
	Section ThreadSection `json:"section"`
}
type ThreadSectionMoveParams struct {
	BeforeThreadID *string `json:"beforeThreadId,omitempty"`
	SectionID      *string `json:"sectionId"`
	ThreadID       string  `json:"threadId"`
}
type ThreadSectionMoveResponse struct{}

func (s *ThreadService) Delete(ctx context.Context, params ThreadDeleteParams) (ThreadDeleteResponse, error) {
	err := s.client.sendEmptyObjectRequest(ctx, methodThreadDelete, params)
	return ThreadDeleteResponse{}, err
}
func (s *ThreadService) GoalGet(ctx context.Context, params ThreadGoalGetParams) (ThreadGoalGetResponse, error) {
	var resp ThreadGoalGetResponse
	err := s.client.sendRequest(ctx, methodThreadGoalGet, params, &resp)
	return resp, err
}
func (s *ThreadService) GoalSet(ctx context.Context, params ThreadGoalSetParams) (ThreadGoalSetResponse, error) {
	var resp ThreadGoalSetResponse
	err := s.client.sendRequest(ctx, methodThreadGoalSet, params, &resp)
	return resp, err
}
func (s *ThreadService) GoalClear(ctx context.Context, params ThreadGoalClearParams) (ThreadGoalClearResponse, error) {
	var resp ThreadGoalClearResponse
	err := s.client.sendRequest(ctx, methodThreadGoalClear, params, &resp)
	return resp, err
}
func (s *ThreadService) SectionCreate(ctx context.Context, params ThreadSectionCreateParams) (ThreadSectionCreateResponse, error) {
	var resp ThreadSectionCreateResponse
	err := s.client.sendRequest(ctx, methodThreadSectionCreate, params, &resp)
	return resp, err
}
func (s *ThreadService) SectionDelete(ctx context.Context, params ThreadSectionDeleteParams) (ThreadSectionDeleteResponse, error) {
	err := s.client.sendEmptyObjectRequest(ctx, methodThreadSectionDelete, params)
	return ThreadSectionDeleteResponse{}, err
}
func (s *ThreadService) SectionList(ctx context.Context, params ThreadSectionListParams) (ThreadSectionListResponse, error) {
	var resp ThreadSectionListResponse
	err := s.client.sendRequest(ctx, methodThreadSectionList, params, &resp)
	return resp, err
}
func (s *ThreadService) SectionUpdate(ctx context.Context, params ThreadSectionUpdateParams) (ThreadSectionUpdateResponse, error) {
	var resp ThreadSectionUpdateResponse
	err := s.client.sendRequest(ctx, methodThreadSectionUpdate, params, &resp)
	return resp, err
}
func (s *ThreadService) SectionMove(ctx context.Context, params ThreadSectionMoveParams) (ThreadSectionMoveResponse, error) {
	err := s.client.sendEmptyObjectRequest(ctx, methodThreadSectionMove, params)
	return ThreadSectionMoveResponse{}, err
}

// SkillsExtraRootsSetParams replaces the global extra skill roots.
type SkillsExtraRootsSetParams struct {
	ExtraRoots []string `json:"extraRoots"`
}
type SkillsExtraRootsSetResponse struct{}

func (p SkillsExtraRootsSetParams) prepareRequest() (interface{}, error) {
	roots, err := normalizeAbsolutePathSliceField("extraRoots", p.ExtraRoots)
	if err != nil {
		return nil, err
	}
	p.ExtraRoots = roots
	return p, nil
}

func (s *SkillsService) SetExtraRoots(ctx context.Context, params SkillsExtraRootsSetParams) (SkillsExtraRootsSetResponse, error) {
	err := s.client.sendEmptyObjectRequest(ctx, methodSkillsExtraRootsSet, params)
	return SkillsExtraRootsSetResponse{}, err
}

type ExternalAgentConfigImportItemTypeFailure struct {
	Cwd          *string                              `json:"cwd,omitempty"`
	ErrorType    *string                              `json:"errorType,omitempty"`
	FailureStage string                               `json:"failureStage"`
	ItemType     ExternalAgentConfigMigrationItemType `json:"itemType"`
	Message      string                               `json:"message"`
	Source       *string                              `json:"source,omitempty"`
	SubErrorType *string                              `json:"subErrorType,omitempty"`
}
type ExternalAgentConfigImportItemTypeSuccess struct {
	Cwd      *string                              `json:"cwd,omitempty"`
	ItemType ExternalAgentConfigMigrationItemType `json:"itemType"`
	Source   *string                              `json:"source,omitempty"`
	Target   *string                              `json:"target,omitempty"`
	Title    *string                              `json:"title,omitempty"`
}
type ExternalAgentConfigImportTypeResult struct {
	Failures  []ExternalAgentConfigImportItemTypeFailure `json:"failures"`
	ItemType  ExternalAgentConfigMigrationItemType       `json:"itemType"`
	Successes []ExternalAgentConfigImportItemTypeSuccess `json:"successes"`
}
type ExternalAgentConfigImportProgressNotification struct {
	ImportID        string                                `json:"importId"`
	ItemTypeResults []ExternalAgentConfigImportTypeResult `json:"itemTypeResults"`
}
type ExternalAgentConfigImportHistoryRecordSuccessParams = ExternalAgentConfigImportItemTypeSuccess
type ExternalAgentConfigImportHistoryRecordTypeResultParams struct {
	Failures  []ExternalAgentConfigImportItemTypeFailure            `json:"failures"`
	ItemType  ExternalAgentConfigMigrationItemType                  `json:"itemType"`
	Successes []ExternalAgentConfigImportHistoryRecordSuccessParams `json:"successes"`
}
type ExternalAgentConfigImportHistoryRecordParams struct {
	ItemTypeResults []ExternalAgentConfigImportHistoryRecordTypeResultParams `json:"itemTypeResults"`
	ProviderID      string                                                   `json:"providerId"`
}
type ExternalAgentConfigImportHistoryRecordResponse struct {
	ImportID string `json:"importId"`
}
type ExternalAgentConfigImportHistory struct {
	CompletedAtMs int64                                      `json:"completedAtMs"`
	Failures      []ExternalAgentConfigImportItemTypeFailure `json:"failures"`
	ImportID      string                                     `json:"importId"`
	ProviderID    *string                                    `json:"providerId,omitempty"`
	Successes     []ExternalAgentConfigImportItemTypeSuccess `json:"successes"`
}
type ExternalAgentImportedConnectorSource string

const ExternalAgentImportedConnectorSourceRemoteMcpServersConfig ExternalAgentImportedConnectorSource = "remoteMcpServersConfig"

type ExternalAgentImportedConnectorCandidate struct {
	Name         string                               `json:"name"`
	SessionCount uint32                               `json:"sessionCount"`
	Source       ExternalAgentImportedConnectorSource `json:"source"`
}
type ExternalAgentConfigImportHistoriesReadResponse struct {
	Connectors []ExternalAgentImportedConnectorCandidate `json:"connectors"`
	Data       []ExternalAgentConfigImportHistory        `json:"data"`
}

func (s *ExternalAgentService) ImportHistories(ctx context.Context) (ExternalAgentConfigImportHistoriesReadResponse, error) {
	var resp ExternalAgentConfigImportHistoriesReadResponse
	err := s.client.sendRequest(ctx, methodExternalAgentConfigImportReadHistories, nil, &resp)
	return resp, err
}
func (s *ExternalAgentService) RecordImportHistory(ctx context.Context, params ExternalAgentConfigImportHistoryRecordParams) (ExternalAgentConfigImportHistoryRecordResponse, error) {
	var resp ExternalAgentConfigImportHistoryRecordResponse
	err := s.client.sendRequest(ctx, methodExternalAgentConfigImportRecordHistory, params, &resp)
	return resp, err
}

// New notifications introduced by the synced protocol.
type ThreadDeletedNotification struct {
	ThreadID string `json:"threadId"`
}
type ThreadRevertedNotification struct {
	ThreadID string `json:"threadId"`
}
type ThreadQueueChangedNotification struct {
	ThreadID string `json:"threadId"`
}
type ThreadProjectUpdatedNotification struct {
	ProjectID *string `json:"projectId"`
	ThreadID  string  `json:"threadId"`
}
type EnvironmentConnectionNotification struct {
	EnvironmentID string `json:"environmentId"`
	ThreadID      string `json:"threadId"`
}
type StrictReviewRequiredNotification struct {
	StartedAtMs int64  `json:"startedAtMs"`
	ThreadID    string `json:"threadId"`
	TurnID      string `json:"turnId"`
}
type TurnModerationMetadataNotification struct {
	Metadata map[string]interface{} `json:"metadata"`
	ThreadID string                 `json:"threadId"`
	TurnID   string                 `json:"turnId"`
}
type ProjectChangeType string

const (
	ProjectChangeTypeCreated ProjectChangeType = "created"
	ProjectChangeTypeUpdated ProjectChangeType = "updated"
	ProjectChangeTypeDeleted ProjectChangeType = "deleted"
)

type ProjectChangedNotification struct {
	ChangeType ProjectChangeType `json:"changeType"`
	ProjectID  string            `json:"projectId"`
}
type ModelSafetyBufferingUpdatedNotification struct {
	FasterModel     *string  `json:"fasterModel,omitempty"`
	Model           string   `json:"model"`
	Reasons         []string `json:"reasons"`
	ShowBufferingUI bool     `json:"showBufferingUi"`
	ThreadID        string   `json:"threadId"`
	TurnID          string   `json:"turnId"`
	UseCases        []string `json:"useCases"`
}
type RawResponseCompletedNotification struct {
	ResponseID string               `json:"responseId"`
	ThreadID   string               `json:"threadId"`
	TurnID     string               `json:"turnId"`
	Usage      *TokenUsageBreakdown `json:"usage"`
}

type AgentMessageDelivery string
const AgentMessageDeliveryAsync AgentMessageDelivery = "async"

type McpToolCallAppContext struct {
	ActionName  *string `json:"actionName,omitempty"`
	AppName     *string `json:"appName,omitempty"`
	ConnectorID string  `json:"connectorId"`
	LinkID      *string `json:"linkId,omitempty"`
	ResourceURI *string `json:"resourceUri,omitempty"`
}

func setTypedNotificationHandler[T any](c *Client, method string, handler func(T)) {
	if handler == nil {
		c.OnNotification(method, nil)
		return
	}
	c.OnNotification(method, func(_ context.Context, notif Notification) {
		var params T
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(method, fmt.Errorf("unmarshal %s: %w", method, err))
			return
		}
		handler(params)
	})
}

func (c *Client) OnThreadDeleted(h func(ThreadDeletedNotification)) {
	setTypedNotificationHandler(c, notifyThreadDeleted, h)
}
func (c *Client) OnThreadReverted(h func(ThreadRevertedNotification)) {
	setTypedNotificationHandler(c, notifyThreadReverted, h)
}
func (c *Client) OnThreadQueueChanged(h func(ThreadQueueChangedNotification)) {
	setTypedNotificationHandler(c, notifyThreadQueueChanged, h)
}
func (c *Client) OnThreadProjectUpdated(h func(ThreadProjectUpdatedNotification)) {
	setTypedNotificationHandler(c, notifyThreadProjectUpdated, h)
}
func (c *Client) OnStrictReviewRequired(h func(StrictReviewRequiredNotification)) {
	setTypedNotificationHandler(c, notifyStrictReviewRequired, h)
}
func (c *Client) OnExternalAgentConfigImportProgress(h func(ExternalAgentConfigImportProgressNotification)) {
	setTypedNotificationHandler(c, notifyExternalAgentConfigImportProgress, h)
}
func (c *Client) OnTurnModerationMetadata(h func(TurnModerationMetadataNotification)) {
	setTypedNotificationHandler(c, notifyTurnModerationMetadata, h)
}
func (c *Client) OnProjectChanged(h func(ProjectChangedNotification)) {
	setTypedNotificationHandler(c, notifyProjectChanged, h)
}
func (c *Client) OnModelSafetyBufferingUpdated(h func(ModelSafetyBufferingUpdatedNotification)) {
	setTypedNotificationHandler(c, notifyModelSafetyBufferingUpdated, h)
}

func (c *Client) AddThreadDeletedListener(h func(ThreadDeletedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadDeleted, h)
}
func (c *Client) AddThreadRevertedListener(h func(ThreadRevertedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadReverted, h)
}
func (c *Client) AddThreadQueueChangedListener(h func(ThreadQueueChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadQueueChanged, h)
}
func (c *Client) AddThreadProjectUpdatedListener(h func(ThreadProjectUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadProjectUpdated, h)
}
func (c *Client) AddStrictReviewRequiredListener(h func(StrictReviewRequiredNotification)) func() {
	return addTypedNotificationListener(c, notifyStrictReviewRequired, h)
}
func (c *Client) AddExternalAgentConfigImportProgressListener(h func(ExternalAgentConfigImportProgressNotification)) func() {
	return addTypedNotificationListener(c, notifyExternalAgentConfigImportProgress, h)
}
func (c *Client) AddTurnModerationMetadataListener(h func(TurnModerationMetadataNotification)) func() {
	return addTypedNotificationListener(c, notifyTurnModerationMetadata, h)
}
func (c *Client) AddProjectChangedListener(h func(ProjectChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyProjectChanged, h)
}
func (c *Client) AddModelSafetyBufferingUpdatedListener(h func(ModelSafetyBufferingUpdatedNotification)) func() {
	return addTypedNotificationListener(c, notifyModelSafetyBufferingUpdated, h)
}
