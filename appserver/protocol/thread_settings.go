package protocol

// ActivePermissionProfile identifies the active permission profile for a thread.
type ActivePermissionProfile struct {
	Extends *string `json:"extends,omitempty"`
	ID      string  `json:"id"`
}

func (p *ActivePermissionProfile) UnmarshalJSON(data []byte) error {
	type wire ActivePermissionProfile
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"id"}, []string{"id"}); err != nil {
		return err
	}
	*p = ActivePermissionProfile(decoded)
	return nil
}

// ThreadSettings contains runtime settings for a thread.
type ThreadSettings struct {
	ActivePermissionProfile *ActivePermissionProfile `json:"activePermissionProfile,omitempty"`
	ApprovalPolicy          AskForApprovalWrapper    `json:"approvalPolicy"`
	ApprovalsReviewer       ApprovalsReviewer        `json:"approvalsReviewer"`
	CollaborationMode       CollaborationMode        `json:"collaborationMode"`
	Cwd                     string                   `json:"cwd"`
	Effort                  *ReasoningEffort         `json:"effort,omitempty"`
	Model                   string                   `json:"model"`
	ModelProvider           string                   `json:"modelProvider"`
	Personality             *Personality             `json:"personality,omitempty"`
	SandboxPolicy           SandboxPolicyWrapper     `json:"sandboxPolicy"`
	ServiceTier             *string                  `json:"serviceTier,omitempty"`
	Summary                 *ReasoningSummaryWrapper `json:"summary,omitempty"`
}

func (s *ThreadSettings) UnmarshalJSON(data []byte) error {
	type wire ThreadSettings
	var decoded wire
	required := []string{
		"approvalPolicy",
		"approvalsReviewer",
		"collaborationMode",
		"cwd",
		"model",
		"modelProvider",
		"sandboxPolicy",
	}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	validatedCwd, err := validateInboundAbsolutePathField("threadSettings.cwd", decoded.Cwd)
	if err != nil {
		return err
	}
	decoded.Cwd = validatedCwd
	*s = ThreadSettings(decoded)
	return nil
}

// ThreadSettingsUpdatedNotification is sent when a thread's settings change.
type ThreadSettingsUpdatedNotification struct {
	ThreadID       string         `json:"threadId"`
	ThreadSettings ThreadSettings `json:"threadSettings"`
}

func (n *ThreadSettingsUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadSettingsUpdatedNotification
	var decoded wire
	required := []string{"threadId", "threadSettings"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadSettingsUpdatedNotification(decoded)
	return nil
}
