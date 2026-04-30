package codex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// DynamicToolCallParams represents parameters for a dynamic tool call.
type DynamicToolCallParams struct {
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments"` // any JSON structure
	CallID    string      `json:"callId"`
	Namespace *string     `json:"namespace,omitempty"`
	ThreadID  string      `json:"threadId"`
	TurnID    string      `json:"turnId"`
}

func (p *DynamicToolCallParams) UnmarshalJSON(data []byte) error {
	type wire DynamicToolCallParams
	var decoded wire
	required := []string{"arguments", "callId", "threadId", "tool", "turnId"}
	nonNull := []string{"callId", "threadId", "tool", "turnId"}
	if err := unmarshalInboundObject(data, &decoded, required, nonNull); err != nil {
		return err
	}
	if err := validateNonEmptyStringFields(map[string]string{
		"callId":   decoded.CallID,
		"threadId": decoded.ThreadID,
		"tool":     decoded.Tool,
		"turnId":   decoded.TurnID,
	}); err != nil {
		return err
	}
	*p = DynamicToolCallParams(decoded)
	return nil
}

// DynamicToolCallResponse represents the response to a dynamic tool call.
type DynamicToolCallResponse struct {
	Success      bool                                      `json:"success"`
	ContentItems []DynamicToolCallOutputContentItemWrapper `json:"contentItems"`
}

func (r DynamicToolCallResponse) validate() error {
	if r.ContentItems == nil {
		return errors.New("missing contentItems")
	}
	for i, item := range r.ContentItems {
		if err := item.validateForResponse(); err != nil {
			return fmt.Errorf("contentItems[%d]: %w", i, err)
		}
	}
	return nil
}

// DynamicToolCallOutputContentItem is a discriminated union for tool output content.
type DynamicToolCallOutputContentItem interface {
	dynamicToolCallOutputContentItem()
}

// UnknownDynamicToolCallOutputContentItem represents an unrecognized tool output content type from a newer protocol version.
type UnknownDynamicToolCallOutputContentItem struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (u *UnknownDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (u *UnknownDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
}

// DynamicToolCallOutputContentItemWrapper wraps output content items for JSON marshaling.
type DynamicToolCallOutputContentItemWrapper struct {
	Value DynamicToolCallOutputContentItem
}

// InputTextDynamicToolCallOutputContentItem represents text output.
type InputTextDynamicToolCallOutputContentItem struct {
	Text string `json:"text"`
}

func (i *InputTextDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (i *InputTextDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{
		Type: "inputText",
		Text: i.Text,
	})
}

func (i *InputTextDynamicToolCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"type", "text"}, []string{"type", "text"}); err != nil {
		return err
	}
	if decoded.Type != "inputText" {
		return fmt.Errorf("invalid dynamic tool output content item type %q", decoded.Type)
	}
	i.Text = decoded.Text
	return nil
}

// InputImageDynamicToolCallOutputContentItem represents image output.
type InputImageDynamicToolCallOutputContentItem struct {
	ImageURL string `json:"imageUrl"`
}

func (i *InputImageDynamicToolCallOutputContentItem) dynamicToolCallOutputContentItem() {}

func (i *InputImageDynamicToolCallOutputContentItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string `json:"type"`
		ImageURL string `json:"imageUrl"`
	}{
		Type:     "inputImage",
		ImageURL: i.ImageURL,
	})
}

func (i *InputImageDynamicToolCallOutputContentItem) UnmarshalJSON(data []byte) error {
	type wire struct {
		Type     string `json:"type"`
		ImageURL string `json:"imageUrl"`
	}
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"imageUrl", "type"}, []string{"imageUrl", "type"}); err != nil {
		return err
	}
	if decoded.Type != "inputImage" {
		return fmt.Errorf("invalid dynamic tool output content item type %q", decoded.Type)
	}
	i.ImageURL = decoded.ImageURL
	return nil
}

// UnmarshalJSON implements custom unmarshaling for DynamicToolCallOutputContentItemWrapper.
func (w *DynamicToolCallOutputContentItemWrapper) UnmarshalJSON(data []byte) error {
	itemType, err := decodeRequiredObjectTypeField(data, "dynamic tool output content item")
	if err != nil {
		return err
	}

	switch itemType {
	case "inputText":
		var text InputTextDynamicToolCallOutputContentItem
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		w.Value = &text
	case "inputImage":
		var image InputImageDynamicToolCallOutputContentItem
		if err := json.Unmarshal(data, &image); err != nil {
			return err
		}
		w.Value = &image
	default:
		w.Value = &UnknownDynamicToolCallOutputContentItem{Type: itemType, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

// MarshalJSON implements custom marshaling for DynamicToolCallOutputContentItemWrapper.
func (w DynamicToolCallOutputContentItemWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

func (w DynamicToolCallOutputContentItemWrapper) validateForResponse() error {
	switch value := w.Value.(type) {
	case nil:
		return errors.New("missing content item")
	case *InputTextDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return nil
	case *InputImageDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return nil
	case *UnknownDynamicToolCallOutputContentItem:
		if value == nil {
			return errors.New("missing content item")
		}
		return fmt.Errorf("unsupported content item type %q", value.Type)
	default:
		return fmt.Errorf("unsupported content item type %T", w.Value)
	}
}
