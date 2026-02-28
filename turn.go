package codex

import (
	"context"
	"encoding/json"
)

// TurnService handles turn-related operations
type TurnService struct {
	client *Client
}

func newTurnService(client *Client) *TurnService {
	return &TurnService{client: client}
}

// ===== Turn Start =====

// TurnStartParams are the parameters for turn/start
type TurnStartParams struct {
	ThreadID       string          `json:"threadId"`
	Input          []UserInput     `json:"input"`
	ApprovalPolicy *AskForApproval `json:"approvalPolicy,omitempty"`
	Cwd            *string         `json:"cwd,omitempty"`
	Effort         *ReasoningEffort         `json:"effort,omitempty"`
	Model          *string                  `json:"model,omitempty"`
	OutputSchema   interface{}              `json:"outputSchema,omitempty"`
	Personality    *Personality             `json:"personality,omitempty"`
	SandboxPolicy  *SandboxPolicy           `json:"sandboxPolicy,omitempty"`
	Summary        *ReasoningSummaryWrapper  `json:"summary,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for TurnStartParams
func (p *TurnStartParams) UnmarshalJSON(data []byte) error {
	type Alias TurnStartParams
	aux := &struct {
		Input []json.RawMessage `json:"input"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		*p = TurnStartParams{}
		return err
	}

	// Unmarshal each input element
	p.Input = make([]UserInput, len(aux.Input))
	for i, rawInput := range aux.Input {
		input, err := UnmarshalUserInput(rawInput)
		if err != nil {
			*p = TurnStartParams{}
			return err
		}
		p.Input[i] = input
	}

	return nil
}

// TurnStartResponse is the response from turn/start
type TurnStartResponse struct {
	Turn Turn `json:"turn"`
}

// Start starts a new turn in a thread
func (s *TurnService) Start(ctx context.Context, params TurnStartParams) (TurnStartResponse, error) {
	var resp TurnStartResponse
	if err := s.client.sendRequest(ctx, "turn/start", params, &resp); err != nil {
		return TurnStartResponse{}, err
	}
	return resp, nil
}

// ===== Turn Interrupt =====

// TurnInterruptParams are the parameters for turn/interrupt
type TurnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

// TurnInterruptResponse is the response from turn/interrupt (empty)
type TurnInterruptResponse struct{}

// Interrupt interrupts an active turn
func (s *TurnService) Interrupt(ctx context.Context, params TurnInterruptParams) (TurnInterruptResponse, error) {
	var resp TurnInterruptResponse
	if err := s.client.sendRequest(ctx, "turn/interrupt", params, &resp); err != nil {
		return TurnInterruptResponse{}, err
	}
	return resp, nil
}

// ===== Turn Steer =====

// TurnSteerParams are the parameters for turn/steer
type TurnSteerParams struct {
	ThreadID       string      `json:"threadId"`
	ExpectedTurnID string      `json:"expectedTurnId"`
	Input          []UserInput `json:"input"`
}

// UnmarshalJSON implements custom unmarshaling for TurnSteerParams
func (p *TurnSteerParams) UnmarshalJSON(data []byte) error {
	type Alias TurnSteerParams
	aux := &struct {
		Input []json.RawMessage `json:"input"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		*p = TurnSteerParams{}
		return err
	}

	// Unmarshal each input element
	p.Input = make([]UserInput, len(aux.Input))
	for i, rawInput := range aux.Input {
		input, err := UnmarshalUserInput(rawInput)
		if err != nil {
			*p = TurnSteerParams{}
			return err
		}
		p.Input[i] = input
	}

	return nil
}

// TurnSteerResponse is the response from turn/steer
type TurnSteerResponse struct {
	TurnID string `json:"turnId"`
}

// Steer steers an active turn with new input
func (s *TurnService) Steer(ctx context.Context, params TurnSteerParams) (TurnSteerResponse, error) {
	var resp TurnSteerResponse
	if err := s.client.sendRequest(ctx, "turn/steer", params, &resp); err != nil {
		return TurnSteerResponse{}, err
	}
	return resp, nil
}

// ===== UserInput Types =====

// UserInput is an interface for different input types
type UserInput interface {
	userInput()
}

// TextUserInput represents text input
type TextUserInput struct {
	Text         string        `json:"text"`
	TextElements []TextElement `json:"text_elements,omitempty"`
}

func (t *TextUserInput) userInput() {}

func (t *TextUserInput) MarshalJSON() ([]byte, error) {
	type Alias TextUserInput
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "text",
		Alias: (*Alias)(t),
	})
}

// ImageUserInput represents image input
type ImageUserInput struct {
	URL string `json:"url"`
}

func (i *ImageUserInput) userInput() {}

func (i *ImageUserInput) MarshalJSON() ([]byte, error) {
	type Alias ImageUserInput
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "image",
		Alias: (*Alias)(i),
	})
}

// LocalImageUserInput represents local image input
type LocalImageUserInput struct {
	Path string `json:"path"`
}

func (l *LocalImageUserInput) userInput() {}

func (l *LocalImageUserInput) MarshalJSON() ([]byte, error) {
	type Alias LocalImageUserInput
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "localImage",
		Alias: (*Alias)(l),
	})
}

// SkillUserInput represents skill input
type SkillUserInput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *SkillUserInput) userInput() {}

func (s *SkillUserInput) MarshalJSON() ([]byte, error) {
	type Alias SkillUserInput
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "skill",
		Alias: (*Alias)(s),
	})
}

// MentionUserInput represents mention input
type MentionUserInput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (m *MentionUserInput) userInput() {}

func (m *MentionUserInput) MarshalJSON() ([]byte, error) {
	type Alias MentionUserInput
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "mention",
		Alias: (*Alias)(m),
	})
}

// UnknownUserInput represents an unrecognized user input type from a newer protocol version.
type UnknownUserInput struct {
	Type string          `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

func (u *UnknownUserInput) userInput() {}

func (u *UnknownUserInput) MarshalJSON() ([]byte, error) {
	return u.Raw, nil
}

// UnmarshalUserInput unmarshals a UserInput from JSON based on the "type" field
func UnmarshalUserInput(data []byte) (UserInput, error) {
	var typeField struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeField); err != nil {
		return nil, err
	}

	switch typeField.Type {
	case "text":
		var input TextUserInput
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, err
		}
		return &input, nil
	case "image":
		var input ImageUserInput
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, err
		}
		return &input, nil
	case "localImage":
		var input LocalImageUserInput
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, err
		}
		return &input, nil
	case "skill":
		var input SkillUserInput
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, err
		}
		return &input, nil
	case "mention":
		var input MentionUserInput
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, err
		}
		return &input, nil
	default:
		return &UnknownUserInput{Type: typeField.Type, Raw: append(json.RawMessage(nil), data...)}, nil
	}
}
