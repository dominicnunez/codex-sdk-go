package protocol

import "encoding/json"

// These variants previously used encoding/json directly. Preserve omission,
// null-object and receiver merge behavior for valid fields, while rejecting
// forbidden null arrays and string elements before changing the receiver.
type reasoningItemWire ReasoningThreadItem

func (r *ReasoningThreadItem) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "content", "summary"); err != nil {
		return err
	}
	type ReasoningThreadItem reasoningItemWire
	return json.Unmarshal(data, (*ReasoningThreadItem)(r))
}

type textInputWire TextUserInput

func (t *TextUserInput) UnmarshalJSON(data []byte) error {
	if err := validateOptionalArrays(data, nil, "text_elements"); err != nil {
		return err
	}
	type TextUserInput textInputWire
	return json.Unmarshal(data, (*TextUserInput)(t))
}

type sandboxWorkspaceWire SandboxWorkspaceWrite

func (s *SandboxWorkspaceWrite) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "writable_roots"); err != nil {
		return err
	}
	type SandboxWorkspaceWrite sandboxWorkspaceWire
	return json.Unmarshal(data, (*SandboxWorkspaceWrite)(s))
}

type workspacePolicyWire SandboxPolicyWorkspaceWrite

func (s *SandboxPolicyWorkspaceWrite) UnmarshalJSON(data []byte) error {
	if err := validateOptionalStringArrays(data, "writableRoots"); err != nil {
		return err
	}
	type SandboxPolicyWorkspaceWrite workspacePolicyWire
	return json.Unmarshal(data, (*SandboxPolicyWorkspaceWrite)(s))
}
