package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// FunctionCallOutputBody is either plain text or an array of output content items.
// Set exactly one field. A non-nil empty Content slice encodes an empty array.
type FunctionCallOutputBody struct {
	Text    *string
	Content []FunctionCallOutputContentItemWrapper
}

func (b FunctionCallOutputBody) MarshalJSON() ([]byte, error) {
	if (b.Text != nil) == (b.Content != nil) {
		return nil, fmt.Errorf("tool output requires exactly one of Text or Content")
	}
	if b.Text != nil {
		return json.Marshal(*b.Text)
	}
	return json.Marshal(b.Content)
}

func (b *FunctionCallOutputBody) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("missing tool output")
	}
	var decoded FunctionCallOutputBody
	switch data[0] {
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		decoded.Text = &text
	case '[':
		if err := json.Unmarshal(data, &decoded.Content); err != nil {
			return err
		}
	default:
		return fmt.Errorf("tool output must be text or a content array")
	}
	*b = decoded
	return nil
}

// FunctionCallOutputThreadItem is an externally supplied tool result in history.
type FunctionCallOutputThreadItem struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Namespace *string                `json:"namespace,omitempty"`
	Output    FunctionCallOutputBody `json:"output"`
}

func (FunctionCallOutputThreadItem) threadItem() {}

func (v *FunctionCallOutputThreadItem) MarshalJSON() ([]byte, error) {
	type wire FunctionCallOutputThreadItem
	return json.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"functionCallOutput", (*wire)(v)})
}

func decodeFunctionCallOutputThreadItem(data []byte) (ThreadItem, error) {
	return decodeThreadItemInto(data, &FunctionCallOutputThreadItem{}, "id", "name", "output")
}
