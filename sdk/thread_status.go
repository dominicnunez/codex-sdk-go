package codex

import (
	"encoding/json"
	"fmt"
)

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
