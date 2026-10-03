package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
)

// These variants previously used encoding/json directly. Preserve omission,
// null-object and receiver merge behavior for valid fields, while rejecting
// forbidden null arrays and string elements before changing the receiver.
type reasoningItemWire ReasoningThreadItem

func (r *ReasoningThreadItem) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "content", "summary"); err != nil {
		return err
	}
	type ReasoningThreadItem reasoningItemWire
	return unmarshalArrayVariant(data, (*ReasoningThreadItem)(r), r)
}

type textInputWire TextUserInput

func (t *TextUserInput) UnmarshalJSON(data []byte) error {
	if err := validateOptionalArrays(data, nil, "text_elements"); err != nil {
		return err
	}
	type TextUserInput textInputWire
	return unmarshalArrayVariant(data, (*TextUserInput)(t), t)
}

type sandboxWorkspaceWire SandboxWorkspaceWrite

func (s *SandboxWorkspaceWrite) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "writable_roots"); err != nil {
		return err
	}
	type SandboxWorkspaceWrite sandboxWorkspaceWire
	return unmarshalArrayVariant(data, (*SandboxWorkspaceWrite)(s), s)
}

type workspacePolicyWire SandboxPolicyWorkspaceWrite

func (s *SandboxPolicyWorkspaceWrite) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "writableRoots"); err != nil {
		return err
	}
	type SandboxPolicyWorkspaceWrite workspacePolicyWire
	return unmarshalArrayVariant(data, (*SandboxPolicyWorkspaceWrite)(s), s)
}

func unmarshalArrayVariant(data []byte, wire, original any) error {
	err := json.Unmarshal(data, wire)
	if err == nil {
		return nil
	}
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) && typeError.Field == "" && typeError.Type == reflect.TypeOf(wire).Elem() {
		// Method-free local types have the public name but a distinct reflect
		// identity. Preserve the public type in direct root type errors too.
		typeError.Type = reflect.TypeOf(original).Elem()
	}
	return err
}
