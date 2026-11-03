package protocol

import "fmt"

type resetCreditType string
type resetCreditStatus string

var validResetCreditTypes = map[resetCreditType]struct{}{
	"codexRateLimits": {}, "unknown": {},
}

var validResetCreditStatuses = map[resetCreditStatus]struct{}{
	"available": {}, "redeeming": {}, "redeemed": {}, "unknown": {},
}

func (v *resetCreditType) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "rateLimitResetCredits.resetType", validResetCreditTypes, v)
}

func (v *resetCreditStatus) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "rateLimitResetCredits.status", validResetCreditStatuses, v)
}

// Validate each recognized raw value without replacing the public RawMessage.
// Inner names follow the schema exactly; differently cased properties are open
// extras, while the enclosing response keeps its existing native aliases.
func validateResetCreditMetadata(data []byte) error {
	return validateOptionalFields(data, func(_ string, summary []byte) error {
		if isNullJSONValue(summary) {
			return nil
		}
		var count struct {
			AvailableCount int64 `json:"availableCount"`
		}
		if err := unmarshalResponseObject(summary, &count, []string{"availableCount"}, []string{"availableCount"}); err != nil {
			return err
		}
		return validateFields(summary, func(_ string, credits []byte) error {
			if isNullJSONValue(credits) {
				return nil
			}
			if len(credits) == 0 || credits[0] != '[' {
				return fmt.Errorf("must be an array or null")
			}
			return validateArrayElements(credits, func(index int, credit []byte) error {
				if err := validateResetCreditRecord(credit); err != nil {
					return fmt.Errorf("credit %d: %w", index, err)
				}
				return nil
			})
		}, false, "credits")
	}, "rateLimitResetCredits")
}

func validateResetCreditRecord(data []byte) error {
	var wire struct {
		GrantedAt   int64             `json:"grantedAt"`
		ID          string            `json:"id"`
		ResetType   resetCreditType   `json:"resetType"`
		Status      resetCreditStatus `json:"status"`
		ExpiresAt   *int64            `json:"expiresAt"`
		Title       *string           `json:"title"`
		Description *string           `json:"description"`
	}
	required := []string{"grantedAt", "id", "resetType", "status"}
	return unmarshalResponseObject(data, &wire, required, required)
}
