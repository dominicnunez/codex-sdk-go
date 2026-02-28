package codex

import (
	"context"
	"encoding/json"
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

// Thread represents a conversation thread with all its metadata
type Thread struct {
	ID            string                `json:"id"`
	CLIVersion    string                `json:"cliVersion"`
	CreatedAt     int64                 `json:"createdAt"`
	Cwd           string                `json:"cwd"`
	ModelProvider string                `json:"modelProvider"`
	Preview       string                `json:"preview"`
	Source        SessionSourceWrapper  `json:"source"`
	Status        ThreadStatusWrapper   `json:"status"`
	Turns         []Turn                `json:"turns"`
	UpdatedAt     int64                 `json:"updatedAt"`
	AgentNickname *string               `json:"agentNickname,omitempty"`
	AgentRole     *string               `json:"agentRole,omitempty"`
	GitInfo       *GitInfo              `json:"gitInfo,omitempty"`
	Name          *string               `json:"name,omitempty"`
	Path          *string               `json:"path,omitempty"`
}

// GitInfo contains git repository information
type GitInfo struct {
	Branch    *string `json:"branch,omitempty"`
	OriginURL *string `json:"originUrl,omitempty"`
	SHA       *string `json:"sha,omitempty"`
}

// SessionSource represents the source of a thread session
type SessionSource interface {
	isSessionSource()
}

// Simple session source literals
type sessionSourceLiteral string

func (sessionSourceLiteral) isSessionSource() {}

const (
	SessionSourceCLI       sessionSourceLiteral = "cli"
	SessionSourceVSCode    sessionSourceLiteral = "vscode"
	SessionSourceExec      sessionSourceLiteral = "exec"
	SessionSourceAppServer sessionSourceLiteral = "appServer"
	SessionSourceUnknown   sessionSourceLiteral = "unknown"
)

// SessionSourceSubAgent represents a sub-agent session source
type SessionSourceSubAgent struct {
	SubAgent SubAgentSource `json:"subAgent"`
}

func (SessionSourceSubAgent) isSessionSource() {}

// SubAgentSource represents the type of sub-agent
type SubAgentSource interface {
	isSubAgentSource()
}

// Simple sub-agent source literals
type subAgentSourceLiteral string

func (subAgentSourceLiteral) isSubAgentSource() {}

const (
	SubAgentSourceReview              subAgentSourceLiteral = "review"
	SubAgentSourceCompact             subAgentSourceLiteral = "compact"
	SubAgentSourceMemoryConsolidation subAgentSourceLiteral = "memory_consolidation"
)

// SubAgentSourceThreadSpawn represents a thread spawn sub-agent
type SubAgentSourceThreadSpawn struct {
	ThreadSpawn struct {
		AgentNickname  string `json:"agent_nickname"`
		AgentRole      string `json:"agent_role"`
		Depth          uint32 `json:"depth"`
		ParentThreadID string `json:"parent_thread_id"`
	} `json:"thread_spawn"`
}

func (SubAgentSourceThreadSpawn) isSubAgentSource() {}

// SubAgentSourceOther represents an unknown sub-agent type
type SubAgentSourceOther struct {
	Other string `json:"other"`
}

func (SubAgentSourceOther) isSubAgentSource() {}

// UnknownSubAgentSource represents an unrecognized sub-agent source object from a newer protocol version.
type UnknownSubAgentSource struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownSubAgentSource) isSubAgentSource() {}

func (u UnknownSubAgentSource) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// UnknownSessionSource represents an unrecognized session source from a newer protocol version.
type UnknownSessionSource struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownSessionSource) isSessionSource() {}

func (u UnknownSessionSource) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// ThreadStatus represents the current status of a thread
type ThreadStatus interface {
	isThreadStatus()
}

// ThreadStatusNotLoaded represents a not-loaded thread
type ThreadStatusNotLoaded struct {
	Type string `json:"type"`
}

func (ThreadStatusNotLoaded) isThreadStatus() {}

// ThreadStatusIdle represents an idle thread
type ThreadStatusIdle struct {
	Type string `json:"type"`
}

func (ThreadStatusIdle) isThreadStatus() {}

// ThreadStatusSystemError represents a thread with a system error
type ThreadStatusSystemError struct {
	Type string `json:"type"`
}

func (ThreadStatusSystemError) isThreadStatus() {}

// ThreadStatusActive represents an active thread
type ThreadStatusActive struct {
	Type        string             `json:"type"`
	ActiveFlags []ThreadActiveFlag `json:"activeFlags"`
}

func (ThreadStatusActive) isThreadStatus() {}

// UnknownThreadStatus represents an unrecognized thread status type from a newer protocol version.
type UnknownThreadStatus struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownThreadStatus) isThreadStatus() {}

func (u UnknownThreadStatus) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// Turn represents a single turn in a conversation
type Turn struct {
	ID     string          `json:"id"`
	Status TurnStatus      `json:"status"`
	Items  json.RawMessage `json:"items"` // []ThreadItem discriminated union
	Error  *TurnError      `json:"error,omitempty"`
}

// TurnError represents an error in a turn
type TurnError struct {
	Message           string           `json:"message"`
	CodexErrorInfo    json.RawMessage  `json:"codexErrorInfo,omitempty"`
	AdditionalDetails *string          `json:"additionalDetails,omitempty"`
}

// SessionSourceWrapper wraps SessionSource for JSON marshaling
type SessionSourceWrapper struct {
	Value SessionSource
}

// UnmarshalJSON for SessionSourceWrapper handles the union type
func (s *SessionSourceWrapper) UnmarshalJSON(data []byte) error {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		s.Value = sessionSourceLiteral(literal)
		return nil
	}

	// Try object
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if subAgentRaw, hasKey := raw["subAgent"]; hasKey {
			subAgent, err := unmarshalSubAgentSource(subAgentRaw)
			if err != nil {
				return fmt.Errorf("unmarshal session source subAgent: %w", err)
			}
			s.Value = SessionSourceSubAgent{SubAgent: subAgent}
			return nil
		}
		// Unknown object variant — preserve for forward compatibility
		s.Value = UnknownSessionSource{Raw: append(json.RawMessage(nil), data...)}
		return nil
	}

	return fmt.Errorf("unable to unmarshal SessionSource from: %.200s", data)
}

// unmarshalSubAgentSource dispatches the SubAgentSource discriminated union.
func unmarshalSubAgentSource(data json.RawMessage) (SubAgentSource, error) {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		return subAgentSourceLiteral(literal), nil
	}

	// Try object variants
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("unable to unmarshal SubAgentSource: %w", err)
	}

	if _, ok := keys["thread_spawn"]; ok {
		var ts SubAgentSourceThreadSpawn
		if err := json.Unmarshal(data, &ts); err != nil {
			return nil, fmt.Errorf("unmarshal thread_spawn: %w", err)
		}
		return ts, nil
	}

	if _, ok := keys["other"]; ok {
		var other SubAgentSourceOther
		if err := json.Unmarshal(data, &other); err != nil {
			return nil, fmt.Errorf("unmarshal other: %w", err)
		}
		return other, nil
	}

	return UnknownSubAgentSource{Raw: append(json.RawMessage(nil), data...)}, nil
}

// MarshalJSON for SessionSourceWrapper
func (s SessionSourceWrapper) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	switch v := s.Value.(type) {
	case sessionSourceLiteral:
		return json.Marshal(string(v))
	case SessionSourceSubAgent:
		return json.Marshal(v)
	case UnknownSessionSource:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown SessionSource type: %T", v)
	}
}

// ThreadStatusWrapper wraps ThreadStatus for JSON marshaling
type ThreadStatusWrapper struct {
	Value ThreadStatus
}

// UnmarshalJSON for ThreadStatusWrapper handles the discriminated union
func (t *ThreadStatusWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "notLoaded":
		var status ThreadStatusNotLoaded
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		t.Value = status
	case "idle":
		var status ThreadStatusIdle
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		t.Value = status
	case "systemError":
		var status ThreadStatusSystemError
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		t.Value = status
	case "active":
		var status ThreadStatusActive
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		t.Value = status
	default:
		t.Value = UnknownThreadStatus{Type: raw.Type, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON for ThreadStatusWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (t ThreadStatusWrapper) MarshalJSON() ([]byte, error) {
	if t.Value == nil {
		return []byte("null"), nil
	}
	switch v := t.Value.(type) {
	case ThreadStatusNotLoaded:
		v.Type = "notLoaded"
		return json.Marshal(v)
	case ThreadStatusIdle:
		v.Type = "idle"
		return json.Marshal(v)
	case ThreadStatusSystemError:
		v.Type = "systemError"
		return json.Marshal(v)
	case ThreadStatusActive:
		v.Type = "active"
		return json.Marshal(v)
	case UnknownThreadStatus:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown ThreadStatus type: %T", v)
	}
}

// AskForApproval represents approval policy for operations
type AskForApproval interface {
	isAskForApproval()
}

// Approval policy literals
type approvalPolicyLiteral string

func (approvalPolicyLiteral) isAskForApproval() {}

const (
	ApprovalPolicyUntrusted approvalPolicyLiteral = "untrusted"
	ApprovalPolicyOnFailure approvalPolicyLiteral = "on-failure"
	ApprovalPolicyOnRequest approvalPolicyLiteral = "on-request"
	ApprovalPolicyNever     approvalPolicyLiteral = "never"
)

// ApprovalPolicyReject represents granular rejection policy
type ApprovalPolicyReject struct {
	Reject struct {
		MCPElicitations bool `json:"mcp_elicitations"`
		Rules           bool `json:"rules"`
		SandboxApproval bool `json:"sandbox_approval"`
	} `json:"reject"`
}

func (ApprovalPolicyReject) isAskForApproval() {}

// UnknownAskForApproval represents an unrecognized approval policy shape from a newer protocol version.
type UnknownAskForApproval struct {
	Raw json.RawMessage `json:"-"`
}

func (UnknownAskForApproval) isAskForApproval() {}

func (u UnknownAskForApproval) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// AskForApprovalWrapper wraps AskForApproval for JSON marshaling
type AskForApprovalWrapper struct {
	Value AskForApproval
}

// UnmarshalJSON for AskForApprovalWrapper handles the union type
func (a *AskForApprovalWrapper) UnmarshalJSON(data []byte) error {
	// Try string literal first
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		a.Value = approvalPolicyLiteral(literal)
		return nil
	}

	// Try reject object — validate that the discriminating "reject" key
	// is present, otherwise any JSON object would silently match
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawObj); err == nil {
		if _, hasKey := rawObj["reject"]; hasKey {
			var reject ApprovalPolicyReject
			if err := json.Unmarshal(data, &reject); err != nil {
				return fmt.Errorf("unmarshal approval policy reject: %w", err)
			}
			a.Value = reject
			return nil
		}
	}

	// Unknown shape — preserve raw JSON for forward compatibility
	a.Value = UnknownAskForApproval{Raw: append(json.RawMessage(nil), data...)}
	return nil
}

// MarshalJSON for AskForApprovalWrapper
func (a AskForApprovalWrapper) MarshalJSON() ([]byte, error) {
	if a.Value == nil {
		return []byte("null"), nil
	}
	switch v := a.Value.(type) {
	case approvalPolicyLiteral:
		return json.Marshal(string(v))
	case ApprovalPolicyReject:
		return json.Marshal(v)
	case UnknownAskForApproval:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown AskForApproval type: %T", v)
	}
}

// SandboxPolicy represents sandbox access policy
type SandboxPolicy interface {
	isSandboxPolicy()
}

// SandboxPolicyDangerFullAccess allows full system access
type SandboxPolicyDangerFullAccess struct {
	Type string `json:"type"`
}

func (SandboxPolicyDangerFullAccess) isSandboxPolicy() {}

// SandboxPolicyReadOnly allows read-only access
type SandboxPolicyReadOnly struct {
	Type   string                `json:"type"`
	Access *ReadOnlyAccessWrapper `json:"access,omitempty"`
}

func (SandboxPolicyReadOnly) isSandboxPolicy() {}

// SandboxPolicyExternalSandbox uses external sandbox
type SandboxPolicyExternalSandbox struct {
	Type          string         `json:"type"`
	NetworkAccess *NetworkAccess `json:"networkAccess,omitempty"` // "restricted" or "enabled"
}

func (SandboxPolicyExternalSandbox) isSandboxPolicy() {}

// SandboxPolicyWorkspaceWrite allows workspace writes
type SandboxPolicyWorkspaceWrite struct {
	Type                string                 `json:"type"`
	ExcludeSlashTmp     *bool                  `json:"excludeSlashTmp,omitempty"`
	ExcludeTmpdirEnvVar *bool                  `json:"excludeTmpdirEnvVar,omitempty"`
	NetworkAccess       *bool                  `json:"networkAccess,omitempty"`
	ReadOnlyAccess      *ReadOnlyAccessWrapper `json:"readOnlyAccess,omitempty"`
	WritableRoots       []string               `json:"writableRoots,omitempty"`
}

func (SandboxPolicyWorkspaceWrite) isSandboxPolicy() {}

// UnknownSandboxPolicy represents an unrecognized sandbox policy type from a newer protocol version.
type UnknownSandboxPolicy struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownSandboxPolicy) isSandboxPolicy() {}

func (u UnknownSandboxPolicy) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// ReadOnlyAccess is a discriminated union for read-only access configuration
type ReadOnlyAccess interface {
	isReadOnlyAccess()
}

// ReadOnlyAccessRestricted restricts read access to specific roots
type ReadOnlyAccessRestricted struct {
	Type                    string   `json:"type"`
	IncludePlatformDefaults *bool    `json:"includePlatformDefaults,omitempty"`
	ReadableRoots           []string `json:"readableRoots,omitempty"`
}

func (ReadOnlyAccessRestricted) isReadOnlyAccess() {}

// ReadOnlyAccessFullAccess allows full read access
type ReadOnlyAccessFullAccess struct {
	Type string `json:"type"`
}

func (ReadOnlyAccessFullAccess) isReadOnlyAccess() {}

// UnknownReadOnlyAccess represents an unrecognized read-only access type from a newer protocol version.
type UnknownReadOnlyAccess struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (UnknownReadOnlyAccess) isReadOnlyAccess() {}

func (u UnknownReadOnlyAccess) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// ReadOnlyAccessWrapper wraps the ReadOnlyAccess discriminated union for JSON
type ReadOnlyAccessWrapper struct {
	Value ReadOnlyAccess
}

// UnmarshalJSON for ReadOnlyAccessWrapper
func (w *ReadOnlyAccessWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "restricted":
		var v ReadOnlyAccessRestricted
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "fullAccess":
		w.Value = ReadOnlyAccessFullAccess{Type: "fullAccess"}
	default:
		w.Value = UnknownReadOnlyAccess{Type: raw.Type, Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}

// MarshalJSON for ReadOnlyAccessWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (w ReadOnlyAccessWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil {
		return []byte("null"), nil
	}
	switch v := w.Value.(type) {
	case ReadOnlyAccessRestricted:
		v.Type = "restricted"
		return json.Marshal(v)
	case ReadOnlyAccessFullAccess:
		v.Type = "fullAccess"
		return json.Marshal(v)
	case UnknownReadOnlyAccess:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown ReadOnlyAccess type: %T", v)
	}
}

// NetworkAccess represents network access control
type NetworkAccess string

const (
	NetworkAccessRestricted NetworkAccess = "restricted"
	NetworkAccessEnabled    NetworkAccess = "enabled"
)

// SandboxPolicyWrapper wraps SandboxPolicy for JSON marshaling
type SandboxPolicyWrapper struct {
	Value SandboxPolicy
}

// UnmarshalJSON for SandboxPolicyWrapper handles the discriminated union
func (s *SandboxPolicyWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw.Type {
	case "dangerFullAccess":
		var policy SandboxPolicyDangerFullAccess
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "readOnly":
		var policy SandboxPolicyReadOnly
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "externalSandbox":
		var policy SandboxPolicyExternalSandbox
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "workspaceWrite":
		var policy SandboxPolicyWorkspaceWrite
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	default:
		s.Value = UnknownSandboxPolicy{Type: raw.Type, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON for SandboxPolicyWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (s SandboxPolicyWrapper) MarshalJSON() ([]byte, error) {
	if s.Value == nil {
		return []byte("null"), nil
	}
	switch v := s.Value.(type) {
	case SandboxPolicyDangerFullAccess:
		v.Type = "dangerFullAccess"
		return json.Marshal(v)
	case SandboxPolicyReadOnly:
		v.Type = "readOnly"
		return json.Marshal(v)
	case SandboxPolicyExternalSandbox:
		v.Type = "externalSandbox"
		return json.Marshal(v)
	case SandboxPolicyWorkspaceWrite:
		v.Type = "workspaceWrite"
		return json.Marshal(v)
	case UnknownSandboxPolicy:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown SandboxPolicy type: %T", v)
	}
}

// ThreadStartParams are parameters for starting a new thread
type ThreadStartParams struct {
	ApprovalPolicy        *AskForApproval `json:"approvalPolicy,omitempty"`
	BaseInstructions      *string         `json:"baseInstructions,omitempty"`
	Config                json.RawMessage `json:"config,omitempty"`
	Cwd                   *string         `json:"cwd,omitempty"`
	DeveloperInstructions *string         `json:"developerInstructions,omitempty"`
	Ephemeral             *bool           `json:"ephemeral,omitempty"`
	Model                 *string         `json:"model,omitempty"`
	ModelProvider         *string         `json:"modelProvider,omitempty"`
	Personality           *Personality    `json:"personality,omitempty"`
	Sandbox               *SandboxMode    `json:"sandbox,omitempty"`
	ServiceName           *string         `json:"serviceName,omitempty"`
}

// ThreadStartResponse is the response from starting a thread
type ThreadStartResponse struct {
	ApprovalPolicy  AskForApprovalWrapper `json:"approvalPolicy"`
	Cwd             string                `json:"cwd"`
	Model           string                `json:"model"`
	ModelProvider   string                `json:"modelProvider"`
	ReasoningEffort *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox         SandboxPolicyWrapper  `json:"sandbox"`
	Thread          Thread                `json:"thread"`
}

// Start initiates a new thread
func (s *ThreadService) Start(ctx context.Context, params ThreadStartParams) (ThreadStartResponse, error) {
	var response ThreadStartResponse
	if err := s.client.sendRequest(ctx, "thread/start", params, &response); err != nil {
		return ThreadStartResponse{}, err
	}
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

// Read retrieves thread details
func (s *ThreadService) Read(ctx context.Context, params ThreadReadParams) (ThreadReadResponse, error) {
	var response ThreadReadResponse
	if err := s.client.sendRequest(ctx, "thread/read", params, &response); err != nil {
		return ThreadReadResponse{}, err
	}
	return response, nil
}

// ThreadListParams are parameters for listing threads
type ThreadListParams struct {
	Archived       *bool    `json:"archived,omitempty"`
	Cursor         *string  `json:"cursor,omitempty"`
	Cwd            *string  `json:"cwd,omitempty"`
	Limit          *uint32  `json:"limit,omitempty"`
	ModelProviders []string `json:"modelProviders,omitempty"`
	SearchTerm     *string  `json:"searchTerm,omitempty"`
	SortKey        *ThreadSortKey    `json:"sortKey,omitempty"`
	SourceKinds    []ThreadSourceKind `json:"sourceKinds,omitempty"`
}

// ThreadListResponse is the response from listing threads
type ThreadListResponse struct {
	Data       []Thread `json:"data"`
	NextCursor *string  `json:"nextCursor,omitempty"`
}

// List retrieves a list of threads
func (s *ThreadService) List(ctx context.Context, params ThreadListParams) (ThreadListResponse, error) {
	var response ThreadListResponse
	if err := s.client.sendRequest(ctx, "thread/list", params, &response); err != nil {
		return ThreadListResponse{}, err
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

// LoadedList retrieves loaded threads
func (s *ThreadService) LoadedList(ctx context.Context, params ThreadLoadedListParams) (ThreadLoadedListResponse, error) {
	var response ThreadLoadedListResponse
	if err := s.client.sendRequest(ctx, "thread/loaded/list", params, &response); err != nil {
		return ThreadLoadedListResponse{}, err
	}
	return response, nil
}

// ThreadResumeParams are parameters for resuming a thread
type ThreadResumeParams struct {
	ThreadID              string          `json:"threadId"`
	ApprovalPolicy        *AskForApproval `json:"approvalPolicy,omitempty"`
	BaseInstructions      *string         `json:"baseInstructions,omitempty"`
	Config                json.RawMessage `json:"config,omitempty"`
	Cwd                   *string         `json:"cwd,omitempty"`
	DeveloperInstructions *string         `json:"developerInstructions,omitempty"`
	Model                 *string         `json:"model,omitempty"`
	ModelProvider         *string         `json:"modelProvider,omitempty"`
	Personality           *Personality    `json:"personality,omitempty"`
	Sandbox               *SandboxMode    `json:"sandbox,omitempty"`
}

// ThreadResumeResponse is the response from resuming a thread
type ThreadResumeResponse struct {
	ApprovalPolicy  AskForApprovalWrapper `json:"approvalPolicy"`
	Cwd             string                `json:"cwd"`
	Model           string                `json:"model"`
	ModelProvider   string                `json:"modelProvider"`
	ReasoningEffort *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox         SandboxPolicyWrapper  `json:"sandbox"`
	Thread          Thread                `json:"thread"`
}

// Resume resumes an existing thread
func (s *ThreadService) Resume(ctx context.Context, params ThreadResumeParams) (ThreadResumeResponse, error) {
	var response ThreadResumeResponse
	if err := s.client.sendRequest(ctx, "thread/resume", params, &response); err != nil {
		return ThreadResumeResponse{}, err
	}
	return response, nil
}

// ThreadForkParams are parameters for forking a thread
type ThreadForkParams struct {
	ThreadID              string          `json:"threadId"`
	ApprovalPolicy        *AskForApproval `json:"approvalPolicy,omitempty"`
	BaseInstructions      *string         `json:"baseInstructions,omitempty"`
	Config                json.RawMessage `json:"config,omitempty"`
	Cwd                   *string         `json:"cwd,omitempty"`
	DeveloperInstructions *string         `json:"developerInstructions,omitempty"`
	Model                 *string         `json:"model,omitempty"`
	ModelProvider         *string         `json:"modelProvider,omitempty"`
	Sandbox               *SandboxMode    `json:"sandbox,omitempty"`
}

// ThreadForkResponse is the response from forking a thread
type ThreadForkResponse struct {
	ApprovalPolicy  AskForApprovalWrapper `json:"approvalPolicy"`
	Cwd             string                `json:"cwd"`
	Model           string                `json:"model"`
	ModelProvider   string                `json:"modelProvider"`
	ReasoningEffort *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox         SandboxPolicyWrapper  `json:"sandbox"`
	Thread          Thread                `json:"thread"`
}

// Fork creates a fork of a thread
func (s *ThreadService) Fork(ctx context.Context, params ThreadForkParams) (ThreadForkResponse, error) {
	var response ThreadForkResponse
	if err := s.client.sendRequest(ctx, "thread/fork", params, &response); err != nil {
		return ThreadForkResponse{}, err
	}
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

// Rollback rolls back a thread by N turns
func (s *ThreadService) Rollback(ctx context.Context, params ThreadRollbackParams) (ThreadRollbackResponse, error) {
	var response ThreadRollbackResponse
	if err := s.client.sendRequest(ctx, "thread/rollback", params, &response); err != nil {
		return ThreadRollbackResponse{}, err
	}
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
	if err := s.client.sendRequest(ctx, "thread/name/set", params, nil); err != nil {
		return ThreadSetNameResponse{}, err
	}
	return ThreadSetNameResponse{}, nil
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
	if err := s.client.sendRequest(ctx, "thread/archive", params, nil); err != nil {
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

// Unarchive unarchives a thread
func (s *ThreadService) Unarchive(ctx context.Context, params ThreadUnarchiveParams) (ThreadUnarchiveResponse, error) {
	var response ThreadUnarchiveResponse
	if err := s.client.sendRequest(ctx, "thread/unarchive", params, &response); err != nil {
		return ThreadUnarchiveResponse{}, err
	}
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

// Unsubscribe unsubscribes from a thread
func (s *ThreadService) Unsubscribe(ctx context.Context, params ThreadUnsubscribeParams) (ThreadUnsubscribeResponse, error) {
	var response ThreadUnsubscribeResponse
	if err := s.client.sendRequest(ctx, "thread/unsubscribe", params, &response); err != nil {
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
	if err := s.client.sendRequest(ctx, "thread/compact/start", params, nil); err != nil {
		return ThreadCompactStartResponse{}, err
	}
	return ThreadCompactStartResponse{}, nil
}
