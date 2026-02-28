package codex

// CollaborationModeSettings configures a single collaboration mode.
type CollaborationModeSettings struct {
	Model                 string           `json:"model"`
	DeveloperInstructions *string          `json:"developer_instructions,omitempty"`
	ReasoningEffort       *ReasoningEffort `json:"reasoning_effort,omitempty"`
}

// CollaborationMode pairs a mode kind with its settings.
type CollaborationMode struct {
	Mode     ModeKind                  `json:"mode"`
	Settings CollaborationModeSettings `json:"settings"`
}
