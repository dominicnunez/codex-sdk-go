package protocol

import (
	"encoding/json"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

func (p McpServerElicitationRequestParams) MarshalJSON() ([]byte, error) {
	type wire McpServerElicitationRequestParams
	if p.Mode == McpServerElicitationModeOpenAIForm || p.Mode == McpServerElicitationModeOpenAIFormLegacy {
		return jsonencode.Marshal(struct {
			wire
			RequestedSchema json.RawMessage `json:"requestedSchema"`
		}{wire(p), p.OpenAIRequestedSchema})
	}
	return jsonencode.Marshal(wire(p))
}
