package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// Preserve the public response decoder and lifecycle-validation ordering. The
// private service carrier borrows raw input only until response admission ends.
type threadResumeResult struct {
	value ThreadResumeResponse
	raw   []byte
}

func (r *threadResumeResult) UnmarshalJSON(data []byte) error {
	if err := r.value.UnmarshalJSON(data); err != nil {
		return err
	}
	r.raw = data
	return nil
}

func (r *threadResumeResult) validate() error {
	defer func() { r.raw = nil }()
	if err := r.value.validate(); err != nil {
		return err
	}
	return validateCollaborationModeField(r.raw, true, true)
}

const (
	collabModePresent uint8 = 1 << iota
	collabSettingsPresent
	collabModelPresent
)

type collabFieldPresence struct {
	present bool
	fields  uint8
}

// Outer matching and nullability belong to the carrier. Inner records keep
// native folded matching and merge across duplicate objects; a nullable root
// reset starts fresh presence, while forbidden nested nulls stay invalid.
func validateCollaborationModeField(data []byte, aliases, nullable bool) error {
	state := collabFieldPresence{}
	var err error
	// Both owners already validated the complete response/settings object.
	// Walk borrowed fields directly instead of validating the payload again.
	jsonobject.WalkFields(data, true, func(key, raw []byte) {
		if err != nil {
			return
		}
		matched := jsonobject.FieldMatches(key, "collaborationMode")
		if aliases {
			matched = jsonobject.FieldMatchesFolded(key, "collaborationmode")
		}
		if !matched {
			return
		}
		if isNullJSONValue(raw) {
			if !nullable {
				err = fmt.Errorf("collaborationMode: %w", ErrNullResultField)
				return
			}
			state = collabFieldPresence{}
			return
		}
		state.present = true
		err = state.observeMode(raw)
	})
	if err != nil {
		return err
	}
	if !state.present {
		return nil
	}
	for bit, field := range []string{"mode", "settings", "settings.model"} {
		if state.fields&(1<<bit) == 0 {
			return fmt.Errorf("collaborationMode.%s: %w", field, ErrMissingResultField)
		}
	}
	return nil
}

func (state *collabFieldPresence) observeMode(raw []byte) error {
	var err error
	jsonobject.WalkFields(raw, true, func(key, value []byte) {
		if err != nil {
			return
		}
		switch {
		case jsonobject.FieldMatchesFolded(key, "mode"):
			if isNullJSONValue(value) {
				err = fmt.Errorf("collaborationMode.mode: %w", ErrNullResultField)
				return
			}
			state.fields |= collabModePresent
		case jsonobject.FieldMatchesFolded(key, "settings"):
			if isNullJSONValue(value) {
				err = fmt.Errorf("collaborationMode.settings: %w", ErrNullResultField)
				return
			}
			state.fields |= collabSettingsPresent
			err = state.observeSettings(value)
		}
	})
	return err
}

func (state *collabFieldPresence) observeSettings(raw []byte) error {
	var err error
	jsonobject.WalkFields(raw, true, func(key, value []byte) {
		if err != nil || !jsonobject.FieldMatchesFolded(key, "model") {
			return
		}
		if isNullJSONValue(value) {
			err = fmt.Errorf("collaborationMode.settings.model: %w", ErrNullResultField)
			return
		}
		state.fields |= collabModelPresent
	})
	return err
}
