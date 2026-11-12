package protocol

type AgentMessageModeration struct {
	Moderated bool                 `json:"moderated"`
	Display   *AgentMessageDisplay `json:"display,omitempty"`
}

// AgentMessageDisplay retains presentation instructions, including unknown actions.
type AgentMessageDisplay struct {
	Action               string  `json:"action"`
	BlockType            *string `json:"blockType,omitempty"`
	CheckTrustedContacts *bool   `json:"checkTrustedContacts,omitempty"`
	CyberTrustedAccess   *bool   `json:"cyberTrustedAccess,omitempty"`
}
