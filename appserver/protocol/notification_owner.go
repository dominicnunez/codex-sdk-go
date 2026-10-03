package protocol

import (
	"bytes"
	"encoding/json"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

// These synced notifications use exact schema properties, matching transport
// ownership and the other typed notification decoders.
func decodeOwnedNotification[T any](data []byte, required []string) (T, error) {
	var decoded T
	err := unmarshalInboundObject(data, &decoded, required, required)
	return decoded, err
}

func (n *ExternalAgentConfigImportProgressNotification) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportProgressNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"importId", "itemTypeResults"})
	if err != nil {
		return err
	}
	*n = ExternalAgentConfigImportProgressNotification(decoded)
	return nil
}

func (n *ExternalAgentConfigImportCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire ExternalAgentConfigImportCompletedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"importId", "itemTypeResults"})
	if err != nil {
		return err
	}
	*n = ExternalAgentConfigImportCompletedNotification(decoded)
	return nil
}

func (n *ProjectChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ProjectChangedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"changeType", "projectId"})
	if err != nil {
		return err
	}
	*n = ProjectChangedNotification(decoded)
	return nil
}

func (n *ThreadDeletedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadDeletedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadDeletedNotification(decoded)
	return nil
}

func (n *ThreadRevertedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRevertedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadRevertedNotification(decoded)
	return nil
}

func (n *ThreadQueueChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadQueueChangedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId"})
	if err != nil {
		return err
	}
	*n = ThreadQueueChangedNotification(decoded)
	return nil
}

func (n *EnvironmentConnectionNotification) UnmarshalJSON(data []byte) error {
	type wire EnvironmentConnectionNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"environmentId", "threadId"})
	if err != nil {
		return err
	}
	*n = EnvironmentConnectionNotification(decoded)
	return nil
}

func (n *StrictReviewRequiredNotification) UnmarshalJSON(data []byte) error {
	type wire StrictReviewRequiredNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"startedAtMs", "threadId", "turnId"})
	if err != nil {
		return err
	}
	*n = StrictReviewRequiredNotification(decoded)
	return nil
}

func (n *TurnModerationMetadataNotification) UnmarshalJSON(data []byte) error {
	type wire TurnModerationMetadataNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"metadata", "threadId", "turnId"}, []string{"threadId", "turnId"}); err != nil {
		return err
	}
	*n = TurnModerationMetadataNotification(decoded)
	return nil
}

func decodeModerationMetadataJSON(data []byte) (TurnModerationMetadataJSONNotification, error) {
	type wire TurnModerationMetadataJSONNotification
	var decoded struct {
		wire
		Metadata moderationMetadataValue `json:"metadata"`
	}
	// metadata is open JSON and may be null; only its presence is required.
	if err := unmarshalInboundObject(data, &decoded, []string{"metadata", "threadId", "turnId"}, []string{"threadId", "turnId"}); err != nil {
		return TurnModerationMetadataJSONNotification{}, err
	}
	metadata, err := decoded.Metadata.value()
	if err != nil {
		return TurnModerationMetadataJSONNotification{}, err
	}
	decoded.wire.Metadata = metadata
	return TurnModerationMetadataJSONNotification(decoded.wire), nil
}

func (n *TurnModerationMetadataJSONNotification) UnmarshalJSON(data []byte) error {
	decoded, err := decodeModerationMetadataJSON(data)
	if err != nil {
		return err
	}
	*n = decoded
	return nil
}

// Consecutive object occurrences retain the legacy map merge. A different JSON
// kind, including null, replaces the value and resets that accumulation.
// Raw members retain their numeric tokens rather than converting to float64.
type moderationMetadataValue struct {
	raw    json.RawMessage
	object map[string]json.RawMessage
}

func (m *moderationMetadataValue) UnmarshalJSON(data []byte) error {
	incoming := bytes.TrimSpace(data)
	if len(incoming) > 0 && incoming[0] == '{' && (m.object != nil || len(m.raw) > 0 && m.raw[0] == '{') {
		if m.object == nil {
			if err := jsondecode.Unmarshal(m.raw, &m.object); err != nil {
				return err
			}
			m.raw = nil
		}
		// Accumulate once and encode once after envelope admission, rather than
		// repeatedly decoding and encoding the growing object for each duplicate.
		return jsondecode.Unmarshal(incoming, &m.object)
	}
	m.object = nil
	m.raw = append(json.RawMessage(nil), incoming...)
	return nil
}

func (m *moderationMetadataValue) value() (json.RawMessage, error) {
	if m.object != nil {
		return jsonencode.Marshal(m.object)
	}
	return m.raw, nil
}

func (n *ModelSafetyBufferingUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ModelSafetyBufferingUpdatedNotification
	var decoded struct {
		wire
		Reasons  nonNullStringList `json:"reasons"`
		UseCases nonNullStringList `json:"useCases"`
	}
	required := []string{"model", "reasons", "showBufferingUi", "threadId", "turnId", "useCases"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.wire.Reasons = []string(decoded.Reasons)
	decoded.wire.UseCases = []string(decoded.UseCases)
	*n = ModelSafetyBufferingUpdatedNotification(decoded.wire)
	return nil
}
