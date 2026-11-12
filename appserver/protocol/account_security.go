package protocol

import "context"

type GetAccountSecuritySetupResponse struct {
	Notice *AccountSecuritySetupNotice `json:"notice,omitempty"`
}
type AccountSecuritySetupNotice struct {
	AccountID   string                     `json:"accountId"`
	UserID      string                     `json:"userId"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Action      AccountSecuritySetupAction `json:"action"`
}
type AccountSecuritySetupAction struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SecuritySetupRead retrieves a reminder without changing account security.
func (s *AccountService) SecuritySetupRead(ctx context.Context) (GetAccountSecuritySetupResponse, error) {
	var response accountSecurityAdmission
	if err := s.client.sendRequest(ctx, methodAccountSecuritySetupRead, nil, &response); err != nil {
		return GetAccountSecuritySetupResponse{}, err
	}
	return response.GetAccountSecuritySetupResponse, nil
}

type accountSecurityAdmission struct {
	GetAccountSecuritySetupResponse
}

func (r *accountSecurityAdmission) UnmarshalJSON(data []byte) error {
	type wire GetAccountSecuritySetupResponse
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, nil, nil); err != nil {
		return err
	}
	rule := recordRule{required: []string{"accountId", "userId", "title", "description", "action"}, children: map[string]recordRule{"action": {required: []string{"label", "url"}}}}
	if err := validateNestedRecord(data, "notice", rule); err != nil {
		return err
	}
	r.GetAccountSecuritySetupResponse = GetAccountSecuritySetupResponse(decoded)
	return nil
}
