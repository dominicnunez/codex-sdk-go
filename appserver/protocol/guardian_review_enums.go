package protocol

// Private wire values validate every occurrence without adding JSON methods to
// the public scalar types or changing anonymous application envelopes.
type guardianUserAuthValue GuardianUserAuthorization

func (v *guardianUserAuthValue) UnmarshalJSON(data []byte) error {
	var value GuardianUserAuthorization
	if err := unmarshalEnumString(data, "guardian.review.userAuthorization", map[GuardianUserAuthorization]struct{}{
		GuardianUserAuthorizationUnknown: {},
		GuardianUserAuthorizationLow:     {},
		GuardianUserAuthorizationMedium:  {},
		GuardianUserAuthorizationHigh:    {},
	}, &value); err != nil {
		return err
	}
	*v = guardianUserAuthValue(value)
	return nil
}

type autoReviewSourceValue AutoReviewDecisionSource

func (v *autoReviewSourceValue) UnmarshalJSON(data []byte) error {
	var value AutoReviewDecisionSource
	if err := unmarshalEnumString(data, "guardian.review.decisionSource", map[AutoReviewDecisionSource]struct{}{
		AutoReviewDecisionSourceAgent: {},
	}, &value); err != nil {
		return err
	}
	*v = autoReviewSourceValue(value)
	return nil
}
