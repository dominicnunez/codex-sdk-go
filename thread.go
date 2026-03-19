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

// Thread represents a conversation thread with all its metadata
type Thread struct {
	ID            string               `json:"id"`
	CLIVersion    string               `json:"cliVersion"`
	CreatedAt     int64                `json:"createdAt"`
	Cwd           string               `json:"cwd"`
	ModelProvider string               `json:"modelProvider"`
	Preview       string               `json:"preview"`
	Source        SessionSourceWrapper `json:"source"`
	Status        ThreadStatusWrapper  `json:"status"`
	Turns         []Turn               `json:"turns"`
	UpdatedAt     int64                `json:"updatedAt"`
	Ephemeral     bool                 `json:"ephemeral"`
	AgentNickname *string              `json:"agentNickname,omitempty"`
	AgentRole     *string              `json:"agentRole,omitempty"`
	GitInfo       *GitInfo             `json:"gitInfo,omitempty"`
	Name          *string              `json:"name,omitempty"`
	Path          *string              `json:"path,omitempty"`
}

func (t *Thread) UnmarshalJSON(data []byte) error {
	type threadWire struct {
		ID            *string               `json:"id"`
		CLIVersion    *string               `json:"cliVersion"`
		CreatedAt     *int64                `json:"createdAt"`
		Cwd           *string               `json:"cwd"`
		ModelProvider *string               `json:"modelProvider"`
		Preview       *string               `json:"preview"`
		Source        *SessionSourceWrapper `json:"source"`
		Status        *ThreadStatusWrapper  `json:"status"`
		Turns         *[]Turn               `json:"turns"`
		UpdatedAt     *int64                `json:"updatedAt"`
		Ephemeral     *bool                 `json:"ephemeral"`
		AgentNickname *string               `json:"agentNickname"`
		AgentRole     *string               `json:"agentRole"`
		GitInfo       *GitInfo              `json:"gitInfo"`
		Name          *string               `json:"name"`
		Path          *string               `json:"path"`
	}

	var wire threadWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	switch {
	case wire.ID == nil:
		return errors.New("missing thread.id")
	case wire.CLIVersion == nil:
		return errors.New("missing thread.cliVersion")
	case wire.CreatedAt == nil:
		return errors.New("missing thread.createdAt")
	case wire.Cwd == nil:
		return errors.New("missing thread.cwd")
	case wire.ModelProvider == nil:
		return errors.New("missing thread.modelProvider")
	case wire.Preview == nil:
		return errors.New("missing thread.preview")
	case wire.Source == nil:
		return errors.New("missing thread.source")
	case wire.Status == nil:
		return errors.New("missing thread.status")
	case wire.Turns == nil:
		return errors.New("missing thread.turns")
	case wire.UpdatedAt == nil:
		return errors.New("missing thread.updatedAt")
	case wire.Ephemeral == nil:
		return errors.New("missing thread.ephemeral")
	}

	t.ID = *wire.ID
	t.CLIVersion = *wire.CLIVersion
	t.CreatedAt = *wire.CreatedAt
	t.Cwd = *wire.Cwd
	t.ModelProvider = *wire.ModelProvider
	t.Preview = *wire.Preview
	t.Source = *wire.Source
	t.Status = *wire.Status
	t.Turns = *wire.Turns
	t.UpdatedAt = *wire.UpdatedAt
	t.Ephemeral = *wire.Ephemeral
	t.AgentNickname = wire.AgentNickname
	t.AgentRole = wire.AgentRole
	t.GitInfo = wire.GitInfo
	t.Name = wire.Name
	t.Path = wire.Path

	return nil
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

func (s *SubAgentSourceThreadSpawn) UnmarshalJSON(data []byte) error {
	var raw struct {
		ThreadSpawn json.RawMessage `json:"thread_spawn"`
	}
	if err := unmarshalInboundObject(data, &raw, []string{"thread_spawn"}, []string{"thread_spawn"}); err != nil {
		return err
	}
	if err := validateRequiredObjectFields(raw.ThreadSpawn, "depth", "parent_thread_id"); err != nil {
		return err
	}
	type wire SubAgentSourceThreadSpawn
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = SubAgentSourceThreadSpawn(decoded)
	return nil
}

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
type ThreadStatusNotLoaded struct{}

func (ThreadStatusNotLoaded) isThreadStatus() {}

// ThreadStatusIdle represents an idle thread
type ThreadStatusIdle struct{}

func (ThreadStatusIdle) isThreadStatus() {}

// ThreadStatusSystemError represents a thread with a system error
type ThreadStatusSystemError struct{}

func (ThreadStatusSystemError) isThreadStatus() {}

// ThreadStatusActive represents an active thread
type ThreadStatusActive struct {
	ActiveFlags []ThreadActiveFlag `json:"activeFlags"`
}

func (ThreadStatusActive) isThreadStatus() {}

func (t *ThreadStatusActive) UnmarshalJSON(data []byte) error {
	type wire ThreadStatusActive
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"activeFlags"}, []string{"activeFlags"}); err != nil {
		return err
	}
	*t = ThreadStatusActive(decoded)
	return nil
}

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
	ID     string              `json:"id"`
	Status TurnStatus          `json:"status"`
	Items  []ThreadItemWrapper `json:"items"`
	Error  *TurnError          `json:"error,omitempty"`
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "id", "status", "items"); err != nil {
		return err
	}
	type wire Turn
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Turn(decoded)
	return nil
}

// TurnError represents an error in a turn.
// It implements the error interface so callers can use errors.As to inspect
// structured fields (CodexErrorInfo, AdditionalDetails).
type TurnError struct {
	Message           string          `json:"message"`
	CodexErrorInfo    json.RawMessage `json:"codexErrorInfo,omitempty"`
	AdditionalDetails *string         `json:"additionalDetails,omitempty"`
}

// Error implements the error interface.
func (e *TurnError) Error() string { return e.Message }

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

	return nil, errors.New("sub-agent source: missing discriminator")
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
	typeField, err := decodeRequiredObjectTypeField(data, "thread status")
	if err != nil {
		return err
	}

	switch typeField {
	case "notLoaded":
		t.Value = ThreadStatusNotLoaded{}
	case "idle":
		t.Value = ThreadStatusIdle{}
	case "systemError":
		t.Value = ThreadStatusSystemError{}
	case "active":
		var status ThreadStatusActive
		if err := validateRequiredTaggedObjectFields(data, "activeFlags"); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		t.Value = status
	default:
		t.Value = UnknownThreadStatus{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
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
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "notLoaded"})
	case ThreadStatusIdle:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "idle"})
	case ThreadStatusSystemError:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "systemError"})
	case ThreadStatusActive:
		return json.Marshal(struct {
			Type string `json:"type"`
			ThreadStatusActive
		}{Type: "active", ThreadStatusActive: v})
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

// ApprovalPolicyGranular represents granular approval policy overrides.
type ApprovalPolicyGranular struct {
	Granular struct {
		MCPElicitations    bool  `json:"mcp_elicitations"`
		RequestPermissions *bool `json:"request_permissions,omitempty"`
		Rules              bool  `json:"rules"`
		SandboxApproval    bool  `json:"sandbox_approval"`
		SkillApproval      *bool `json:"skill_approval,omitempty"`
	} `json:"granular"`
}

func (ApprovalPolicyGranular) isAskForApproval() {}

func (a *ApprovalPolicyGranular) UnmarshalJSON(data []byte) error {
	var raw struct {
		Granular json.RawMessage `json:"granular"`
	}
	if err := unmarshalInboundObject(data, &raw, []string{"granular"}, []string{"granular"}); err != nil {
		return err
	}
	if err := validateRequiredObjectFields(raw.Granular, "mcp_elicitations", "rules", "sandbox_approval"); err != nil {
		return err
	}
	type wire ApprovalPolicyGranular
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = ApprovalPolicyGranular(decoded)
	return nil
}

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

	// Try granular object — validate that the discriminating "granular" key
	// is present, otherwise any JSON object would silently match
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawObj); err == nil {
		if _, hasKey := rawObj["granular"]; hasKey {
			var granular ApprovalPolicyGranular
			if err := json.Unmarshal(data, &granular); err != nil {
				return fmt.Errorf("unmarshal approval policy granular: %w", err)
			}
			a.Value = granular
			return nil
		}
		return errors.New("approval policy: missing discriminator")
	}

	return fmt.Errorf("unable to unmarshal approval policy from: %.200s", data)
}

// MarshalJSON for AskForApprovalWrapper
func (a AskForApprovalWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeAskForApproval(a.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case approvalPolicyLiteral:
		return json.Marshal(string(v))
	case ApprovalPolicyGranular:
		return json.Marshal(v)
	case UnknownAskForApproval:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown AskForApproval type: %T", v)
	}
}

func normalizeAskForApproval(value AskForApproval) AskForApproval {
	switch v := value.(type) {
	case nil:
		return nil
	case *approvalPolicyLiteral:
		if v == nil {
			return nil
		}
		return *v
	case *ApprovalPolicyGranular:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownAskForApproval:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
}

// SandboxPolicy represents sandbox access policy
type SandboxPolicy interface {
	isSandboxPolicy()
}

// SandboxPolicyDangerFullAccess allows full system access
type SandboxPolicyDangerFullAccess struct{}

func (SandboxPolicyDangerFullAccess) isSandboxPolicy() {}

// SandboxPolicyReadOnly allows read-only access
type SandboxPolicyReadOnly struct {
	Access *ReadOnlyAccessWrapper `json:"access,omitempty"`
}

func (SandboxPolicyReadOnly) isSandboxPolicy() {}

// SandboxPolicyExternalSandbox uses external sandbox
type SandboxPolicyExternalSandbox struct {
	NetworkAccess *NetworkAccess `json:"networkAccess,omitempty"` // "restricted" or "enabled"
}

func (SandboxPolicyExternalSandbox) isSandboxPolicy() {}

// SandboxPolicyWorkspaceWrite allows workspace writes
type SandboxPolicyWorkspaceWrite struct {
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
	IncludePlatformDefaults *bool    `json:"includePlatformDefaults,omitempty"`
	ReadableRoots           []string `json:"readableRoots,omitempty"`
}

func (ReadOnlyAccessRestricted) isReadOnlyAccess() {}

// ReadOnlyAccessFullAccess allows full read access
type ReadOnlyAccessFullAccess struct{}

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
	typeField, err := decodeRequiredObjectTypeField(data, "read only access")
	if err != nil {
		return err
	}

	switch typeField {
	case "restricted":
		var v ReadOnlyAccessRestricted
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = v
	case "fullAccess":
		w.Value = ReadOnlyAccessFullAccess{}
	default:
		w.Value = UnknownReadOnlyAccess{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}

// MarshalJSON for ReadOnlyAccessWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (w ReadOnlyAccessWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeReadOnlyAccess(w.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case ReadOnlyAccessRestricted:
		return json.Marshal(struct {
			Type string `json:"type"`
			ReadOnlyAccessRestricted
		}{Type: "restricted", ReadOnlyAccessRestricted: v})
	case ReadOnlyAccessFullAccess:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "fullAccess"})
	case UnknownReadOnlyAccess:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown ReadOnlyAccess type: %T", v)
	}
}

func normalizeReadOnlyAccess(value ReadOnlyAccess) ReadOnlyAccess {
	switch v := value.(type) {
	case nil:
		return nil
	case *ReadOnlyAccessRestricted:
		if v == nil {
			return nil
		}
		return *v
	case *ReadOnlyAccessFullAccess:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownReadOnlyAccess:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
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
	typeField, err := decodeRequiredObjectTypeField(data, "sandbox policy")
	if err != nil {
		return err
	}

	switch typeField {
	case "dangerFullAccess":
		s.Value = SandboxPolicyDangerFullAccess{}
	case "readOnly":
		var policy SandboxPolicyReadOnly
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "externalSandbox":
		var policy SandboxPolicyExternalSandbox
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	case "workspaceWrite":
		var policy SandboxPolicyWorkspaceWrite
		if err := validateRequiredTaggedObjectFields(data); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}
		s.Value = policy
	default:
		s.Value = UnknownSandboxPolicy{Type: typeField, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON for SandboxPolicyWrapper injects the correct type discriminator
// so that client-constructed values marshal correctly without requiring callers
// to manually set the Type field.
func (s SandboxPolicyWrapper) MarshalJSON() ([]byte, error) {
	value := normalizeSandboxPolicy(s.Value)
	if value == nil {
		return []byte("null"), nil
	}
	switch v := value.(type) {
	case SandboxPolicyDangerFullAccess:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{Type: "dangerFullAccess"})
	case SandboxPolicyReadOnly:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyReadOnly
		}{Type: "readOnly", SandboxPolicyReadOnly: v})
	case SandboxPolicyExternalSandbox:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyExternalSandbox
		}{Type: "externalSandbox", SandboxPolicyExternalSandbox: v})
	case SandboxPolicyWorkspaceWrite:
		return json.Marshal(struct {
			Type string `json:"type"`
			SandboxPolicyWorkspaceWrite
		}{Type: "workspaceWrite", SandboxPolicyWorkspaceWrite: v})
	case UnknownSandboxPolicy:
		return v.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown SandboxPolicy type: %T", v)
	}
}

func normalizeSandboxPolicy(value SandboxPolicy) SandboxPolicy {
	switch v := value.(type) {
	case nil:
		return nil
	case *SandboxPolicyDangerFullAccess:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyReadOnly:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyExternalSandbox:
		if v == nil {
			return nil
		}
		return *v
	case *SandboxPolicyWorkspaceWrite:
		if v == nil {
			return nil
		}
		return *v
	case *UnknownSandboxPolicy:
		if v == nil {
			return nil
		}
		return *v
	default:
		return value
	}
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
}

// ThreadStartResponse is the response from starting a thread
type ThreadStartResponse struct {
	ApprovalPolicy    AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd               string                `json:"cwd"`
	Model             string                `json:"model"`
	ModelProvider     string                `json:"modelProvider"`
	ReasoningEffort   *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox           SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier       *ServiceTier          `json:"serviceTier,omitempty"`
	Thread            Thread                `json:"thread"`
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
	return validateApprovalsReviewer(approvalsReviewer)
}

func validateApprovalsReviewer(reviewer ApprovalsReviewer) error {
	switch reviewer {
	case "":
		return errors.New("missing approvalsReviewer")
	case ApprovalsReviewerUser, ApprovalsReviewerGuardianSubagent:
		return nil
	default:
		return fmt.Errorf("invalid approvalsReviewer %q", reviewer)
	}
}

func (r ThreadStartResponse) validate() error {
	return validateThreadLifecycleResponseFields(
		r.ApprovalPolicy,
		r.ApprovalsReviewer,
		r.Cwd,
		r.Model,
		r.ModelProvider,
		r.Sandbox,
		r.Thread,
	)
}

// Start initiates a new thread
func (s *ThreadService) Start(ctx context.Context, params ThreadStartParams) (ThreadStartResponse, error) {
	var response ThreadStartResponse
	if err := s.client.sendRequest(ctx, methodThreadStart, params, &response); err != nil {
		return ThreadStartResponse{}, err
	}
	if err := response.validate(); err != nil {
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
	if err := response.validate(); err != nil {
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
	SortKey        *ThreadSortKey     `json:"sortKey,omitempty"`
	SourceKinds    []ThreadSourceKind `json:"sourceKinds,omitempty"`
}

// ThreadListResponse is the response from listing threads
type ThreadListResponse struct {
	Data       []Thread `json:"data"`
	NextCursor *string  `json:"nextCursor,omitempty"`
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

// ThreadResumeParams are parameters for resuming a thread
type ThreadResumeParams struct {
	ThreadID              string             `json:"threadId"`
	ApprovalPolicy        *AskForApproval    `json:"approvalPolicy,omitempty"`
	ApprovalsReviewer     *ApprovalsReviewer `json:"approvalsReviewer,omitempty"`
	BaseInstructions      *string            `json:"baseInstructions,omitempty"`
	Config                json.RawMessage    `json:"config,omitempty"`
	Cwd                   *string            `json:"cwd,omitempty"`
	DeveloperInstructions *string            `json:"developerInstructions,omitempty"`
	Model                 *string            `json:"model,omitempty"`
	ModelProvider         *string            `json:"modelProvider,omitempty"`
	Personality           *Personality       `json:"personality,omitempty"`
	Sandbox               *SandboxMode       `json:"sandbox,omitempty"`
	ServiceTier           *ServiceTier       `json:"serviceTier,omitempty"`
}

// ThreadResumeResponse is the response from resuming a thread
type ThreadResumeResponse struct {
	ApprovalPolicy    AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd               string                `json:"cwd"`
	Model             string                `json:"model"`
	ModelProvider     string                `json:"modelProvider"`
	ReasoningEffort   *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox           SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier       *ServiceTier          `json:"serviceTier,omitempty"`
	Thread            Thread                `json:"thread"`
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
		r.Thread,
	)
}

// Resume resumes an existing thread
func (s *ThreadService) Resume(ctx context.Context, params ThreadResumeParams) (ThreadResumeResponse, error) {
	var response ThreadResumeResponse
	if err := s.client.sendRequest(ctx, methodThreadResume, params, &response); err != nil {
		return ThreadResumeResponse{}, err
	}
	if err := response.validate(); err != nil {
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
	Model                 *string            `json:"model,omitempty"`
	ModelProvider         *string            `json:"modelProvider,omitempty"`
	Sandbox               *SandboxMode       `json:"sandbox,omitempty"`
	ServiceTier           *ServiceTier       `json:"serviceTier,omitempty"`
}

// ThreadForkResponse is the response from forking a thread
type ThreadForkResponse struct {
	ApprovalPolicy    AskForApprovalWrapper `json:"approvalPolicy"`
	ApprovalsReviewer ApprovalsReviewer     `json:"approvalsReviewer"`
	Cwd               string                `json:"cwd"`
	Model             string                `json:"model"`
	ModelProvider     string                `json:"modelProvider"`
	ReasoningEffort   *ReasoningEffort      `json:"reasoningEffort,omitempty"`
	Sandbox           SandboxPolicyWrapper  `json:"sandbox"`
	ServiceTier       *ServiceTier          `json:"serviceTier,omitempty"`
	Thread            Thread                `json:"thread"`
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
		r.Thread,
	)
}

// Fork creates a fork of a thread
func (s *ThreadService) Fork(ctx context.Context, params ThreadForkParams) (ThreadForkResponse, error) {
	var response ThreadForkResponse
	if err := s.client.sendRequest(ctx, methodThreadFork, params, &response); err != nil {
		return ThreadForkResponse{}, err
	}
	if err := response.validate(); err != nil {
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
	if err := response.validate(); err != nil {
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
	if err := response.validate(); err != nil {
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
	if err := response.validate(); err != nil {
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

func (r *ThreadUnsubscribeResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "status"); err != nil {
		return err
	}
	type wire ThreadUnsubscribeResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
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
