package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ThreadService provides methods for thread lifecycle management
type ThreadService struct {
	client *Client
}

// newThreadService creates a new ThreadService
func newThreadService(client *Client) *ThreadService {
	return &ThreadService{client: client}
}

// ThreadStartParams are parameters for starting a new thread
type ThreadStartParams struct {
	ApprovalPolicy        *AskForApproval    `json:"approvalPolicy,omitempty"`
	ApprovalsReviewer     *ApprovalsReviewer `json:"approvalsReviewer,omitempty"`
	BaseInstructions      *string            `json:"baseInstructions,omitempty"`
	Config                json.RawMessage    `json:"config,omitempty"`
	Cwd                   *string            `json:"cwd,omitempty"`
	DeveloperInstructions *string            `json:"developerInstructions,omitempty"`
	Ephemeral             *bool              `json:"ephemeral,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	ModelProvider         *string            `json:"modelProvider,omitempty"`
	Personality           *Personality       `json:"personality,omitempty"`
	Sandbox               *SandboxMode       `json:"sandbox,omitempty"`
	ServiceName           *string            `json:"serviceName,omitempty"`
	ServiceTier           *ServiceTier       `json:"serviceTier,omitempty"`
	SessionStartSource    *ThreadStartSource `json:"sessionStartSource,omitempty"`
}

// ThreadStartSource identifies why a thread was started.
type ThreadStartSource string

const (
	ThreadStartSourceStartup ThreadStartSource = "startup"
	ThreadStartSourceClear   ThreadStartSource = "clear"
)

var validThreadStartSources = map[ThreadStartSource]struct{}{
	ThreadStartSourceStartup: {},
	ThreadStartSourceClear:   {},
}

func (s ThreadStartSource) MarshalJSON() ([]byte, error) {
	return marshalEnumString("sessionStartSource", s, validThreadStartSources)
}

func (s *ThreadStartSource) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "sessionStartSource", validThreadStartSources, s)
}

// ThreadStartResponse is the response from starting a thread
type ThreadStartResponse struct {
	ApprovalPolicy     AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer  ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd                string                `json:"cwd"`
	InstructionSources []string              `json:"instructionSources,omitempty"`
	Model              string                `json:"model"`
	ModelProvider      string                `json:"modelProvider"`
	ReasoningEffort    *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox            SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier        *ServiceTier          `json:"serviceTier,omitempty"`
	Thread             Thread                `json:"thread"`
}

func (r *ThreadStartResponse) UnmarshalJSON(data []byte) error {
	if err := validateThreadLifecycleResponseObject(data); err != nil {
		return err
	}
	type wire ThreadStartResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	*r = ThreadStartResponse(decoded)
	return nil
}

func validateThreadLifecycleResponseObject(data []byte) error {
	return validateRequiredObjectFields(
		data,
		"approvalPolicy",
		"approvalsReviewer",
		"cwd",
		"model",
		"modelProvider",
		"sandbox",
		"thread",
	)
}

func validateThreadLifecycleResponseFields(
	approvalPolicy AskForApprovalWrapper,
	approvalsReviewer ApprovalsReviewer,
	cwd string,
	model string,
	modelProvider string,
	sandbox SandboxPolicyWrapper,
	serviceTier *ServiceTier,
	thread Thread,
) error {
	switch {
	case approvalPolicy.Value == nil:
		return errors.New("missing approvalPolicy")
	case cwd == "":
		return errors.New("missing cwd")
	case model == "":
		return errors.New("missing model")
	case modelProvider == "":
		return errors.New("missing modelProvider")
	case sandbox.Value == nil:
		return errors.New("missing sandbox")
	case thread.ID == "":
		return errors.New("missing thread.id")
	}
	if err := validateOptionalEnumValue("serviceTier", serviceTier, validServiceTiers); err != nil {
		return err
	}
	return validateApprovalsReviewer(approvalsReviewer)
}

func validateApprovalsReviewer(reviewer ApprovalsReviewer) error {
	if reviewer == "" {
		return errors.New("missing approvalsReviewer")
	}
	return validateEnumValue("approvalsReviewer", reviewer, validApprovalsReviewers)
}

func (r ThreadStartResponse) validate() error {
	return validateThreadLifecycleResponseFields(
		r.ApprovalPolicy,
		r.ApprovalsReviewer,
		r.Cwd,
		r.Model,
		r.ModelProvider,
		r.Sandbox,
		r.ServiceTier,
		r.Thread,
	)
}

// Start initiates a new thread
func (s *ThreadService) Start(ctx context.Context, params ThreadStartParams) (ThreadStartResponse, error) {
	var response ThreadStartResponse
	if err := s.client.sendRequest(ctx, methodThreadStart, params, &response); err != nil {
		return ThreadStartResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadReadParams are parameters for reading a thread
type ThreadReadParams struct {
	ThreadID     string `json:"threadId"`
	IncludeTurns *bool  `json:"includeTurns,omitempty"`
}

// ThreadReadResponse is the response from reading a thread
type ThreadReadResponse struct {
	Thread Thread `json:"thread"`
}

func (r ThreadReadResponse) validate() error {
	if r.Thread.ID == "" {
		return errors.New("missing thread.id")
	}
	return nil
}

// Read retrieves thread details
func (s *ThreadService) Read(ctx context.Context, params ThreadReadParams) (ThreadReadResponse, error) {
	var response ThreadReadResponse
	if err := s.client.sendRequest(ctx, methodThreadRead, params, &response); err != nil {
		return ThreadReadResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadListParams are parameters for listing threads
type ThreadListParams struct {
	Archived       *bool              `json:"archived,omitempty"`
	Cursor         *string            `json:"cursor,omitempty"`
	Cwd            *string            `json:"cwd,omitempty"`
	Limit          *uint32            `json:"limit,omitempty"`
	ModelProviders []string           `json:"modelProviders,omitempty"`
	SearchTerm     *string            `json:"searchTerm,omitempty"`
	SortDirection  *SortDirection     `json:"sortDirection,omitempty"`
	SortKey        *ThreadSortKey     `json:"sortKey,omitempty"`
	SourceKinds    []ThreadSourceKind `json:"sourceKinds,omitempty"`
	UseStateDbOnly *bool              `json:"useStateDbOnly,omitempty"`
}

// ThreadListResponse is the response from listing threads
type ThreadListResponse struct {
	BackwardsCursor *string  `json:"backwardsCursor,omitempty"`
	Data            []Thread `json:"data"`
	NextCursor      *string  `json:"nextCursor,omitempty"`
}

func (r *ThreadListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ThreadListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadListResponse(decoded)
	return nil
}

// List retrieves a list of threads
func (s *ThreadService) List(ctx context.Context, params ThreadListParams) (ThreadListResponse, error) {
	var response ThreadListResponse
	if err := s.client.sendRequest(ctx, methodThreadList, params, &response); err != nil {
		return ThreadListResponse{}, err
	}
	for _, thread := range response.Data {
		s.client.cacheThreadState(thread)
	}
	return response, nil
}

// ThreadLoadedListParams are parameters for listing loaded threads
type ThreadLoadedListParams struct {
	Cursor *string `json:"cursor,omitempty"`
	Limit  *uint32 `json:"limit,omitempty"`
}

// ThreadLoadedListResponse is the response from listing loaded threads
type ThreadLoadedListResponse struct {
	Data       []string `json:"data"`
	NextCursor *string  `json:"nextCursor,omitempty"`
}

func (r *ThreadLoadedListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ThreadLoadedListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadLoadedListResponse(decoded)
	return nil
}

// LoadedList retrieves loaded threads
func (s *ThreadService) LoadedList(ctx context.Context, params ThreadLoadedListParams) (ThreadLoadedListResponse, error) {
	var response ThreadLoadedListResponse
	if err := s.client.sendRequest(ctx, methodThreadLoadedList, params, &response); err != nil {
		return ThreadLoadedListResponse{}, err
	}
	return response, nil
}

// SortDirection controls thread turn pagination direction.
type SortDirection string

const (
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

var validSortDirections = map[SortDirection]struct{}{
	SortDirectionAsc:  {},
	SortDirectionDesc: {},
}

func (d SortDirection) MarshalJSON() ([]byte, error) {
	return marshalEnumString("sortDirection", d, validSortDirections)
}

func (d *SortDirection) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "sortDirection", validSortDirections, d)
}

// ThreadTurnsListParams are parameters for listing turns in a thread.
type ThreadTurnsListParams struct {
	Cursor        *string        `json:"cursor,omitempty"`
	Limit         *uint32        `json:"limit,omitempty"`
	SortDirection *SortDirection `json:"sortDirection,omitempty"`
	ThreadID      string         `json:"threadId"`
}

// ThreadTurnsListResponse is the response from thread/turns/list.
type ThreadTurnsListResponse struct {
	BackwardsCursor *string `json:"backwardsCursor,omitempty"`
	Data            []Turn  `json:"data"`
	NextCursor      *string `json:"nextCursor,omitempty"`
}

func (r *ThreadTurnsListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ThreadTurnsListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadTurnsListResponse(decoded)
	return nil
}

// TurnsList lists turns in a thread.
func (s *ThreadService) TurnsList(ctx context.Context, params ThreadTurnsListParams) (ThreadTurnsListResponse, error) {
	var response ThreadTurnsListResponse
	if err := s.client.sendRequest(ctx, methodThreadTurnsList, params, &response); err != nil {
		return ThreadTurnsListResponse{}, err
	}
	return response, nil
}

// ThreadShellCommandParams runs a shell command in a thread context.
type ThreadShellCommandParams struct {
	Command  string `json:"command"`
	ThreadID string `json:"threadId"`
}

// ThreadShellCommandResponse is the empty response from thread/shellCommand.
type ThreadShellCommandResponse struct{}

// ShellCommand runs a shell command in a thread context.
func (s *ThreadService) ShellCommand(ctx context.Context, params ThreadShellCommandParams) (ThreadShellCommandResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadShellCommand, params); err != nil {
		return ThreadShellCommandResponse{}, err
	}
	return ThreadShellCommandResponse{}, nil
}

// ThreadApproveGuardianDeniedActionParams approves a guardian-denied action.
type ThreadApproveGuardianDeniedActionParams struct {
	Event    json.RawMessage `json:"event"`
	ThreadID string          `json:"threadId"`
}

// ThreadApproveGuardianDeniedActionResponse is the empty response from thread/approveGuardianDeniedAction.
type ThreadApproveGuardianDeniedActionResponse struct{}

// ApproveGuardianDeniedAction approves a guardian-denied action.
func (s *ThreadService) ApproveGuardianDeniedAction(ctx context.Context, params ThreadApproveGuardianDeniedActionParams) (ThreadApproveGuardianDeniedActionResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadApproveGuardianDeniedAction, params); err != nil {
		return ThreadApproveGuardianDeniedActionResponse{}, err
	}
	return ThreadApproveGuardianDeniedActionResponse{}, nil
}

// ThreadInjectItemsParams appends raw Responses API items to a thread's history.
type ThreadInjectItemsParams struct {
	Items    []json.RawMessage `json:"items"`
	ThreadID string            `json:"threadId"`
}

// ThreadInjectItemsResponse is the empty response from thread/inject_items.
type ThreadInjectItemsResponse struct{}

// InjectItems appends raw Responses API items to a thread's history.
func (s *ThreadService) InjectItems(ctx context.Context, params ThreadInjectItemsParams) (ThreadInjectItemsResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadInjectItems, params); err != nil {
		return ThreadInjectItemsResponse{}, err
	}
	return ThreadInjectItemsResponse{}, nil
}

// ThreadResumeParams are parameters for resuming a thread
type ThreadResumeParams struct {
	ThreadID              string             `json:"threadId"`
	ApprovalPolicy        *AskForApproval    `json:"approvalPolicy,omitempty"`
	ApprovalsReviewer     *ApprovalsReviewer `json:"approvalsReviewer,omitempty"`
	BaseInstructions      *string            `json:"baseInstructions,omitempty"`
	Config                json.RawMessage    `json:"config,omitempty"`
	Cwd                   *string            `json:"cwd,omitempty"`
	DeveloperInstructions *string            `json:"developerInstructions,omitempty"`
	ExcludeTurns          *bool              `json:"excludeTurns,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	ModelProvider         *string            `json:"modelProvider,omitempty"`
	Personality           *Personality       `json:"personality,omitempty"`
	Sandbox               *SandboxMode       `json:"sandbox,omitempty"`
	ServiceTier           *ServiceTier       `json:"serviceTier,omitempty"`
}

// ThreadResumeResponse is the response from resuming a thread
type ThreadResumeResponse struct {
	ApprovalPolicy     AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer  ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd                string                `json:"cwd"`
	InstructionSources []string              `json:"instructionSources,omitempty"`
	Model              string                `json:"model"`
	ModelProvider      string                `json:"modelProvider"`
	ReasoningEffort    *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox            SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier        *ServiceTier          `json:"serviceTier,omitempty"`
	Thread             Thread                `json:"thread"`
}

func (r *ThreadResumeResponse) UnmarshalJSON(data []byte) error {
	if err := validateThreadLifecycleResponseObject(data); err != nil {
		return err
	}
	type wire ThreadResumeResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	*r = ThreadResumeResponse(decoded)
	return nil
}

func (r ThreadResumeResponse) validate() error {
	return validateThreadLifecycleResponseFields(
		r.ApprovalPolicy,
		r.ApprovalsReviewer,
		r.Cwd,
		r.Model,
		r.ModelProvider,
		r.Sandbox,
		r.ServiceTier,
		r.Thread,
	)
}

// Resume resumes an existing thread
func (s *ThreadService) Resume(ctx context.Context, params ThreadResumeParams) (ThreadResumeResponse, error) {
	var response ThreadResumeResponse
	if err := s.client.sendRequest(ctx, methodThreadResume, params, &response); err != nil {
		return ThreadResumeResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadForkParams are parameters for forking a thread
type ThreadForkParams struct {
	ThreadID              string             `json:"threadId"`
	ApprovalPolicy        *AskForApproval    `json:"approvalPolicy,omitempty"`
	ApprovalsReviewer     *ApprovalsReviewer `json:"approvalsReviewer,omitempty"`
	BaseInstructions      *string            `json:"baseInstructions,omitempty"`
	Config                json.RawMessage    `json:"config,omitempty"`
	Cwd                   *string            `json:"cwd,omitempty"`
	DeveloperInstructions *string            `json:"developerInstructions,omitempty"`
	Ephemeral             *bool              `json:"ephemeral,omitempty"`
	ExcludeTurns          *bool              `json:"excludeTurns,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	ModelProvider         *string            `json:"modelProvider,omitempty"`
	Sandbox               *SandboxMode       `json:"sandbox,omitempty"`
	ServiceTier           *ServiceTier       `json:"serviceTier,omitempty"`
}

// ThreadForkResponse is the response from forking a thread
type ThreadForkResponse struct {
	ApprovalPolicy     AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer  ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd                string                `json:"cwd"`
	InstructionSources []string              `json:"instructionSources,omitempty"`
	Model              string                `json:"model"`
	ModelProvider      string                `json:"modelProvider"`
	ReasoningEffort    *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox            SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier        *ServiceTier          `json:"serviceTier,omitempty"`
	Thread             Thread                `json:"thread"`
}

func (r *ThreadForkResponse) UnmarshalJSON(data []byte) error {
	if err := validateThreadLifecycleResponseObject(data); err != nil {
		return err
	}
	type wire ThreadForkResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	*r = ThreadForkResponse(decoded)
	return nil
}

func (r ThreadForkResponse) validate() error {
	return validateThreadLifecycleResponseFields(
		r.ApprovalPolicy,
		r.ApprovalsReviewer,
		r.Cwd,
		r.Model,
		r.ModelProvider,
		r.Sandbox,
		r.ServiceTier,
		r.Thread,
	)
}

// Fork creates a fork of a thread
func (s *ThreadService) Fork(ctx context.Context, params ThreadForkParams) (ThreadForkResponse, error) {
	var response ThreadForkResponse
	if err := s.client.sendRequest(ctx, methodThreadFork, params, &response); err != nil {
		return ThreadForkResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadRollbackParams are parameters for rolling back a thread
type ThreadRollbackParams struct {
	ThreadID string `json:"threadId"`
	NumTurns uint32 `json:"numTurns"`
}

// ThreadRollbackResponse is the response from rolling back a thread
type ThreadRollbackResponse struct {
	Thread Thread `json:"thread"`
}

func (r ThreadRollbackResponse) validate() error {
	if r.Thread.ID == "" {
		return errors.New("missing thread.id")
	}
	return nil
}

// Rollback rolls back a thread by N turns
func (s *ThreadService) Rollback(ctx context.Context, params ThreadRollbackParams) (ThreadRollbackResponse, error) {
	var response ThreadRollbackResponse
	if err := s.client.sendRequest(ctx, methodThreadRollback, params, &response); err != nil {
		return ThreadRollbackResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadSetNameParams are parameters for setting thread name
type ThreadSetNameParams struct {
	ThreadID string `json:"threadId"`
	Name     string `json:"name"`
}

// ThreadSetNameResponse is the response from setting thread name
type ThreadSetNameResponse struct {
	// Empty per spec
}

// SetName updates the name of a thread
func (s *ThreadService) SetName(ctx context.Context, params ThreadSetNameParams) (ThreadSetNameResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadNameSet, params); err != nil {
		return ThreadSetNameResponse{}, err
	}
	return ThreadSetNameResponse{}, nil
}

// ThreadMetadataGitInfoUpdateParams patches stored Git metadata for a thread.
type ThreadMetadataGitInfoUpdateParams struct {
	Branch    *string `json:"branch,omitempty"`
	OriginURL *string `json:"originUrl,omitempty"`
	SHA       *string `json:"sha,omitempty"`
}

// ThreadMetadataUpdateParams updates thread metadata.
type ThreadMetadataUpdateParams struct {
	GitInfo  *ThreadMetadataGitInfoUpdateParams `json:"gitInfo,omitempty"`
	ThreadID string                             `json:"threadId"`
}

// ThreadMetadataUpdateResponse is the response from thread/metadata/update.
type ThreadMetadataUpdateResponse struct {
	Thread Thread `json:"thread"`
}

func (r ThreadMetadataUpdateResponse) validate() error {
	if r.Thread.ID == "" {
		return errors.New("missing thread.id")
	}
	return nil
}

// MetadataUpdate updates stored metadata for a thread.
func (s *ThreadService) MetadataUpdate(ctx context.Context, params ThreadMetadataUpdateParams) (ThreadMetadataUpdateResponse, error) {
	var response ThreadMetadataUpdateResponse
	if err := s.client.sendRequest(ctx, methodThreadMetadataUpdate, params, &response); err != nil {
		return ThreadMetadataUpdateResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadArchiveParams are parameters for archiving a thread
type ThreadArchiveParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadArchiveResponse is the response from archiving a thread
type ThreadArchiveResponse struct {
	// Empty per spec
}

// Archive archives a thread
func (s *ThreadService) Archive(ctx context.Context, params ThreadArchiveParams) (ThreadArchiveResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadArchive, params); err != nil {
		return ThreadArchiveResponse{}, err
	}
	return ThreadArchiveResponse{}, nil
}

// ThreadUnarchiveParams are parameters for unarchiving a thread
type ThreadUnarchiveParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadUnarchiveResponse is the response from unarchiving a thread
type ThreadUnarchiveResponse struct {
	Thread Thread `json:"thread"`
}

func (r ThreadUnarchiveResponse) validate() error {
	if r.Thread.ID == "" {
		return errors.New("missing thread.id")
	}
	return nil
}

// Unarchive unarchives a thread
func (s *ThreadService) Unarchive(ctx context.Context, params ThreadUnarchiveParams) (ThreadUnarchiveResponse, error) {
	var response ThreadUnarchiveResponse
	if err := s.client.sendRequest(ctx, methodThreadUnarchive, params, &response); err != nil {
		return ThreadUnarchiveResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}

// ThreadUnsubscribeParams are parameters for unsubscribing from a thread
type ThreadUnsubscribeParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadUnsubscribeResponse is the response from unsubscribing from a thread
type ThreadUnsubscribeResponse struct {
	Status ThreadUnsubscribeStatus `json:"status"`
}

func validateThreadUnsubscribeStatus(status ThreadUnsubscribeStatus) error {
	switch status {
	case ThreadUnsubscribeStatusNotLoaded, ThreadUnsubscribeStatusNotSubscribed, ThreadUnsubscribeStatusUnsubscribed:
		return nil
	default:
		return fmt.Errorf("invalid status %q", status)
	}
}

func (r *ThreadUnsubscribeResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "status"); err != nil {
		return err
	}
	type wire ThreadUnsubscribeResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateThreadUnsubscribeStatus(decoded.Status); err != nil {
		return err
	}
	*r = ThreadUnsubscribeResponse(decoded)
	return nil
}

// Unsubscribe unsubscribes from a thread
func (s *ThreadService) Unsubscribe(ctx context.Context, params ThreadUnsubscribeParams) (ThreadUnsubscribeResponse, error) {
	var response ThreadUnsubscribeResponse
	if err := s.client.sendRequest(ctx, methodThreadUnsubscribe, params, &response); err != nil {
		return ThreadUnsubscribeResponse{}, err
	}
	return response, nil
}

// ThreadCompactStartParams are parameters for starting thread compaction
type ThreadCompactStartParams struct {
	ThreadID string `json:"threadId"`
}

// ThreadCompactStartResponse is the response from starting thread compaction
type ThreadCompactStartResponse struct {
	// Empty per spec
}

// CompactStart initiates thread compaction
func (s *ThreadService) CompactStart(ctx context.Context, params ThreadCompactStartParams) (ThreadCompactStartResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadCompactStart, params); err != nil {
		return ThreadCompactStartResponse{}, err
	}
	return ThreadCompactStartResponse{}, nil
}
