package protocol

import "encoding/json"

func (p McpServerElicitationRequestParams) MarshalJSON() ([]byte, error) {
	type wire McpServerElicitationRequestParams
	if p.Mode == McpServerElicitationModeOpenAIForm || p.Mode == McpServerElicitationModeOpenAIFormLegacy {
		return json.Marshal(struct {
			wire
			RequestedSchema json.RawMessage `json:"requestedSchema"`
		}{wire(p), p.OpenAIRequestedSchema})
	}
	return json.Marshal(wire(p))
}
