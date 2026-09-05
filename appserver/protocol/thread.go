package protocol

import (
	"encoding/json"
	"errors"
)

// Thread represents a conversation thread with all its metadata
type Thread struct {
	ForkedFromID     *string              `json:"forkedFromId,omitempty"`
	ParentThreadID   *string              `json:"parentThreadId,omitempty"`
	ProjectID        *string              `json:"projectId"`
	RecencyAt        *int64               `json:"recencyAt,omitempty"`
	Section          *ThreadSection       `json:"section,omitempty"`
	SectionEnteredAt *int64               `json:"sectionEnteredAt,omitempty"`
	SessionID        string               `json:"sessionId"`
	HistoryMode      ThreadHistoryMode    `json:"historyMode,omitempty"`
	Model            *string              `json:"model,omitempty"`
	ReasoningEffort  *ReasoningEffort     `json:"reasoningEffort,omitempty"`
	ID               string               `json:"id"`
	CLIVersion       string               `json:"cliVersion"`
	CreatedAt        int64                `json:"createdAt"`
	Cwd              string               `json:"cwd"`
	ModelProvider    string               `json:"modelProvider"`
	Preview          string               `json:"preview"`
	Source           SessionSourceWrapper `json:"source"`
	Status           ThreadStatusWrapper  `json:"status"`
	ThreadSource     *ThreadSource        `json:"threadSource,omitempty"`
	Turns            []Turn               `json:"turns"`
	UpdatedAt        int64                `json:"updatedAt"`
	Ephemeral        bool                 `json:"ephemeral"`
	AgentNickname    *string              `json:"agentNickname,omitempty"`
	AgentRole        *string              `json:"agentRole,omitempty"`
	GitInfo          *GitInfo             `json:"gitInfo,omitempty"`
	Name             *string              `json:"name,omitempty"`
	Path             *string              `json:"path,omitempty"`
}

func (t *Thread) UnmarshalJSON(data []byte) error {
	type threadWire struct {
		ForkedFromID     *string               `json:"forkedFromId,omitempty"`
		ParentThreadID   *string               `json:"parentThreadId,omitempty"`
		ProjectID        *string               `json:"projectId,omitempty"`
		RecencyAt        *int64                `json:"recencyAt,omitempty"`
		Section          *ThreadSection        `json:"section,omitempty"`
		SectionEnteredAt *int64                `json:"sectionEnteredAt,omitempty"`
		SessionID        string                `json:"sessionId"`
		HistoryMode      ThreadHistoryMode     `json:"historyMode"`
		Model            *string               `json:"model"`
		ReasoningEffort  *ReasoningEffort      `json:"reasoningEffort"`
		ID               *string               `json:"id"`
		CLIVersion       *string               `json:"cliVersion"`
		CreatedAt        *int64                `json:"createdAt"`
		Cwd              *string               `json:"cwd"`
		ModelProvider    *string               `json:"modelProvider"`
		Preview          *string               `json:"preview"`
		Source           *SessionSourceWrapper `json:"source"`
		Status           *ThreadStatusWrapper  `json:"status"`
		ThreadSource     *ThreadSource         `json:"threadSource"`
		Turns            *[]Turn               `json:"turns"`
		UpdatedAt        *int64                `json:"updatedAt"`
		Ephemeral        *bool                 `json:"ephemeral"`
		AgentNickname    *string               `json:"agentNickname"`
		AgentRole        *string               `json:"agentRole"`
		GitInfo          *GitInfo              `json:"gitInfo"`
		Name             *string               `json:"name"`
		Path             *string               `json:"path"`
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
	if err := validateInboundObjectFields(data, []string{"projectId", "sessionId"}, []string{"sessionId"}); err != nil {
		return err
	}

	t.ID = *wire.ID
	t.ForkedFromID = wire.ForkedFromID
	t.ParentThreadID = wire.ParentThreadID
	t.ProjectID = wire.ProjectID
	t.RecencyAt = wire.RecencyAt
	t.Section = wire.Section
	t.SectionEnteredAt = wire.SectionEnteredAt
	t.SessionID = wire.SessionID
	t.HistoryMode = wire.HistoryMode
	if t.HistoryMode == "" {
		t.HistoryMode = ThreadHistoryModeLegacy
	}
	t.Model = wire.Model
	t.ReasoningEffort = wire.ReasoningEffort
	t.CLIVersion = *wire.CLIVersion
	t.CreatedAt = *wire.CreatedAt
	validatedCwd, err := validateInboundAbsolutePathField("thread.cwd", *wire.Cwd)
	if err != nil {
		return err
	}
	t.Cwd = validatedCwd
	t.ModelProvider = *wire.ModelProvider
	t.Preview = *wire.Preview
	t.Source = *wire.Source
	t.Status = *wire.Status
	t.ThreadSource = wire.ThreadSource
	t.Turns = *wire.Turns
	t.UpdatedAt = *wire.UpdatedAt
	t.Ephemeral = *wire.Ephemeral
	t.AgentNickname = wire.AgentNickname
	t.AgentRole = wire.AgentRole
	t.GitInfo = wire.GitInfo
	t.Name = wire.Name
	t.Path, err = validateInboundAbsolutePathPointerField("thread.path", wire.Path)
	if err != nil {
		return err
	}

	return nil
}

// GitInfo contains git repository information
type GitInfo struct {
	Branch    *string `json:"branch,omitempty"`
	OriginURL *string `json:"originUrl,omitempty"`
	SHA       *string `json:"sha,omitempty"`
}

// Turn represents a single turn in a conversation
type Turn struct {
	ItemsView   TurnItemsView       `json:"itemsView,omitempty"`
	StartedAt   *int64              `json:"startedAt,omitempty"`
	CompletedAt *int64              `json:"completedAt,omitempty"`
	DurationMs  *int64              `json:"durationMs,omitempty"`
	ID          string              `json:"id"`
	Status      TurnStatus          `json:"status"`
	Items       []ThreadItemWrapper `json:"items"`
	Error       *TurnError          `json:"error,omitempty"`
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "id", "status", "items"); err != nil {
		return err
	}
	type wire Turn
	var decoded wire
	decoded.ItemsView = TurnItemsViewFull
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
	Misalignment      *MisalignmentErrorDetails `json:"misalignment,omitempty"`
	Message           string                    `json:"message"`
	CodexErrorInfo    json.RawMessage           `json:"codexErrorInfo,omitempty"`
	AdditionalDetails *string                   `json:"additionalDetails,omitempty"`
	Raw               json.RawMessage           `json:"-"`
}

func (e *TurnError) UnmarshalJSON(data []byte) error {
	type wire TurnError
	var decoded wire
	required := []string{"message"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.Raw = append(json.RawMessage(nil), data...)
	*e = TurnError(decoded)
	return nil
}

// Error implements the error interface.
func (e *TurnError) Error() string { return e.Message }
