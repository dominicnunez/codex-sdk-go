package protocol

type AgentMessageModeration struct {
	Moderated bool                 `json:"moderated"`
	Display   *AgentMessageDisplay `json:"display,omitempty"`
}

// AgentMessageDisplay retains the current display union and optional legacy hints.
// Schema validation belongs to the private agent-message admission boundary.
type AgentMessageDisplay struct {
	Mode                 AgentMessageDisplayMode `json:"mode,omitempty"`
	Version              *uint8                  `json:"version,omitempty"`
	TrustedContacts      *TrustedContacts        `json:"trustedContacts,omitempty"`
	Annotation           *AnswerAnnotation       `json:"annotation,omitempty"`
	Replacement          *AnswerReplacement      `json:"replacement,omitempty"`
	Reason               *BlockReason            `json:"reason,omitempty"`
	Action               string                  `json:"action"`
	BlockType            *string                 `json:"blockType,omitempty"`
	CheckTrustedContacts *bool                   `json:"checkTrustedContacts,omitempty"`
	CyberTrustedAccess   *bool                   `json:"cyberTrustedAccess,omitempty"`
}

type AgentMessageDisplayMode string

const (
	AgentMessageDisplayModeShow  AgentMessageDisplayMode = "show"
	AgentMessageDisplayModeHide  AgentMessageDisplayMode = "hide"
	AgentMessageDisplayModeBlock AgentMessageDisplayMode = "block"
)

type TrustedContacts string

const (
	TrustedContactsCheck TrustedContacts = "check"
	TrustedContactsSkip  TrustedContacts = "skip"
)

type HelplineSubject string

const (
	HelplineSubjectAnswer HelplineSubject = "answer"
	HelplineSubjectPrompt HelplineSubject = "prompt"
)

type AnswerAnnotation struct {
	Kind    string          `json:"kind"`
	Subject HelplineSubject `json:"subject"`
}
type AnswerReplacement struct {
	Kind string `json:"kind"`
}
type BlockReason struct {
	Kind           string `json:"kind"`
	ProtectionType string `json:"protectionType"`
	TrustedAccess  *bool  `json:"trustedAccess,omitempty"`
}
