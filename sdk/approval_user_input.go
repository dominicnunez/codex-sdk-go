package codex

import (
	"errors"
	"fmt"
)

// ToolRequestUserInputParams represents parameters for requesting user input for a tool.
type ToolRequestUserInputParams struct {
	ItemID    string                         `json:"itemId"`
	ThreadID  string                         `json:"threadId"`
	TurnID    string                         `json:"turnId"`
	Questions []ToolRequestUserInputQuestion `json:"questions"`
}

func (p *ToolRequestUserInputParams) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputParams
	var decoded wire
	required := []string{"itemId", "questions", "threadId", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateNonEmptyStringFields(map[string]string{
		"itemId":   decoded.ItemID,
		"threadId": decoded.ThreadID,
		"turnId":   decoded.TurnID,
	}); err != nil {
		return err
	}
	*p = ToolRequestUserInputParams(decoded)
	return nil
}

// ToolRequestUserInputQuestion represents a question to ask the user.
type ToolRequestUserInputQuestion struct {
	ID       string                        `json:"id"`
	Header   string                        `json:"header"`
	Question string                        `json:"question"`
	IsSecret bool                          `json:"isSecret"`
	IsOther  bool                          `json:"isOther"`
	Options  *[]ToolRequestUserInputOption `json:"options,omitempty"`
}

func (q *ToolRequestUserInputQuestion) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputQuestion
	var decoded wire
	required := []string{"header", "id", "question"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateNonEmptyStringField("id", decoded.ID); err != nil {
		return err
	}
	*q = ToolRequestUserInputQuestion(decoded)
	return nil
}

// ToolRequestUserInputOption represents a selectable option for a question.
type ToolRequestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func (o *ToolRequestUserInputOption) UnmarshalJSON(data []byte) error {
	type wire ToolRequestUserInputOption
	var decoded wire
	required := []string{"description", "label"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*o = ToolRequestUserInputOption(decoded)
	return nil
}

// ToolRequestUserInputResponse represents the response containing user's answers.
type ToolRequestUserInputResponse struct {
	Answers map[string]ToolRequestUserInputAnswer `json:"answers"` // question ID → answer
}

func (r ToolRequestUserInputResponse) validate() error {
	if r.Answers == nil {
		return errors.New("missing answers")
	}
	for questionID, answer := range r.Answers {
		if answer.Answers == nil {
			return fmt.Errorf("answers[%q].answers: missing answers", questionID)
		}
	}
	return nil
}

// ToolRequestUserInputAnswer represents an answer to a question.
type ToolRequestUserInputAnswer struct {
	Answers []string `json:"answers"`
}
