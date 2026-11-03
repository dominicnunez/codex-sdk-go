package protocol

// WebSearchContextSize selects the web-search context size.
type WebSearchContextSize string

const (
	WebSearchContextSizeLow    WebSearchContextSize = "low"
	WebSearchContextSizeMedium WebSearchContextSize = "medium"
	WebSearchContextSizeHigh   WebSearchContextSize = "high"
)

var validWebSearchContextSizes = map[WebSearchContextSize]struct{}{
	WebSearchContextSizeLow: {}, WebSearchContextSizeMedium: {}, WebSearchContextSizeHigh: {},
}

func (s WebSearchContextSize) MarshalJSON() ([]byte, error) {
	return marshalEnumString("web_search.context_size", s, validWebSearchContextSizes)
}

func (s *WebSearchContextSize) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "web_search.context_size", validWebSearchContextSizes, s)
}

// WebSearchLocation contains optional location metadata for web search.
type WebSearchLocation struct {
	City     *string `json:"city,omitempty"`
	Country  *string `json:"country,omitempty"`
	Region   *string `json:"region,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
}

// WebSearchToolConfig configures the structured web-search tool.
type WebSearchToolConfig struct {
	AllowedDomains *[]string             `json:"allowed_domains,omitempty"`
	ContextSize    *WebSearchContextSize `json:"context_size,omitempty"`
	Location       *WebSearchLocation    `json:"location,omitempty"`
}

// Config owns validation so the public records keep ordinary JSON composition.
func validateWebSearchConfig(data []byte) error {
	return validateOptionalFields(data, func(_ string, tools []byte) error {
		return validateOptionalFields(tools, func(_ string, search []byte) error {
			if isNullJSONValue(search) {
				return nil
			}
			if err := validateClosedObjectFields(search, "allowed_domains", "context_size", "location"); err != nil {
				return err
			}
			if err := validateNullableStringArrays(search, false, "allowed_domains"); err != nil {
				return err
			}
			return validateFields(search, func(_ string, location []byte) error {
				if isNullJSONValue(location) {
					return nil
				}
				return validateClosedObjectFields(location, "city", "country", "region", "timezone")
			}, false, "location")
		}, "web_search")
	}, "tools")
}
