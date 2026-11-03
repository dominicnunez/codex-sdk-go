package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// HookPromptFragment is a prompt contributed by a hook run.
type HookPromptFragment struct {
	HookRunID string `json:"hookRunId"`
	Text      string `json:"text"`
}

// HookPromptThreadItem records hook-generated prompt fragments in thread history.
type HookPromptThreadItem struct {
	StartedAtMs   *int64               `json:"startedAtMs,omitempty"`
	CompletedAtMs *int64               `json:"completedAtMs,omitempty"`
	ID            string               `json:"id"`
	Fragments     []HookPromptFragment `json:"fragments"`
}

func (HookPromptThreadItem) threadItem() {}

func (h *HookPromptThreadItem) MarshalJSON() ([]byte, error) {
	type wire HookPromptThreadItem
	value := wire(*h)
	if value.Fragments == nil {
		value.Fragments = []HookPromptFragment{}
	}
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{Type: "hookPrompt", wire: &value})
}

func decodeHookPromptThreadItem(data []byte) (ThreadItem, error) {
	item, err := decodeThreadItemInto(data, &HookPromptThreadItem{}, "id", "fragments")
	if err != nil {
		return nil, err
	}
	state := hookPromptPresence{}
	if err := validateOptionalArrays(data, state.observeArray, "fragments"); err != nil {
		return nil, err
	}
	for index := range state.visible {
		for bit, name := range []string{"hookRunId", "text"} {
			if state.fields[index]&(1<<bit) == 0 {
				return nil, fmt.Errorf("hookPrompt.fragments[%d].%s: %w", index, name, ErrMissingResultField)
			}
		}
	}
	return item, nil
}

// Native slice decoding reuses records at the same index across duplicate
// arrays. Track their canonical required-field presence across that same
// history, while keeping forbidden nulls visible even if later values replace them.
type hookPromptPresence struct {
	fields  []uint8
	visible int
}

func (state *hookPromptPresence) observeArray(raw []byte) error {
	count := 0
	err := validateArrayElements(raw, func(index int, item []byte) error {
		count = index + 1
		if isNullJSONValue(item) {
			return fmt.Errorf("hookPrompt.fragments[%d]: %w", index, ErrNullResultField)
		}
		if len(state.fields) <= index {
			state.fields = append(state.fields, make([]uint8, index+1-len(state.fields))...)
		}
		return state.observeRecord(index, item)
	})
	if count == 0 {
		state.fields = nil
	}
	state.visible = count
	return err
}

func (state *hookPromptPresence) observeRecord(index int, item []byte) error {
	var err error
	jsonobject.WalkFields(item, true, func(key, value []byte) {
		if err != nil {
			return
		}
		for bit, name := range []string{"hookRunId", "text"} {
			if !jsonobject.FieldMatches(key, name) {
				continue
			}
			if isNullJSONValue(value) {
				err = fmt.Errorf("hookPrompt.fragments[%d].%s: %w", index, name, ErrNullResultField)
				return
			}
			state.fields[index] |= 1 << bit
			return
		}
	})
	return err
}
