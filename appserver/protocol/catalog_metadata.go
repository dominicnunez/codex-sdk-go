package protocol

// AutoCompactTokenLimitScope selects the context charged against the compact limit.
type AutoCompactTokenLimitScope string

const (
	AutoCompactTokenLimitScopeTotal           AutoCompactTokenLimitScope = "total"
	AutoCompactTokenLimitScopeBodyAfterPrefix AutoCompactTokenLimitScope = "body_after_prefix"
)

var validAutoCompactScopes = map[AutoCompactTokenLimitScope]struct{}{
	AutoCompactTokenLimitScopeTotal: {}, AutoCompactTokenLimitScopeBodyAfterPrefix: {},
}

func (s AutoCompactTokenLimitScope) MarshalJSON() ([]byte, error) {
	return marshalEnumString("AutoCompactTokenLimitScope", s, validAutoCompactScopes)
}

func (s *AutoCompactTokenLimitScope) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "AutoCompactTokenLimitScope", validAutoCompactScopes, s)
}

// MultiAgentVersion is the multi-agent runtime supported by a model.
type MultiAgentVersion string

const (
	MultiAgentVersionDisabled MultiAgentVersion = "disabled"
	MultiAgentVersionV1       MultiAgentVersion = "v1"
	MultiAgentVersionV2       MultiAgentVersion = "v2"
)

var validMultiAgentVersions = map[MultiAgentVersion]struct{}{
	MultiAgentVersionDisabled: {}, MultiAgentVersionV1: {}, MultiAgentVersionV2: {},
}

func (v MultiAgentVersion) MarshalJSON() ([]byte, error) {
	return marshalEnumString("MultiAgentVersion", v, validMultiAgentVersions)
}

func (v *MultiAgentVersion) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "MultiAgentVersion", validMultiAgentVersions, v)
}

// ModelServiceTier describes a service tier advertised by the model catalog.
type ModelServiceTier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *ModelServiceTier) UnmarshalJSON(data []byte) error {
	type wire ModelServiceTier
	var decoded wire
	required := []string{"id", "name", "description"}
	if err := unmarshalResponseObject(data, &decoded, required, required); err != nil {
		return err
	}
	*s = ModelServiceTier(decoded)
	return nil
}
