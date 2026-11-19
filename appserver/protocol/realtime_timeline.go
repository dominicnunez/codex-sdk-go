package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

type ThreadRealtimeVoiceTranscript struct {
	Role ThreadRealtimeTranscriptRole `json:"role"`
	Text string                       `json:"text"`
}
type ThreadRealtimeFrameChannels struct {
	VoiceTranscript ThreadRealtimeVoiceTranscript `json:"voice_transcript"`
}
type ThreadRealtimeHandoffMode string

const (
	ThreadRealtimeHandoffModeStart ThreadRealtimeHandoffMode = "start"
	ThreadRealtimeHandoffModeSteer ThreadRealtimeHandoffMode = "steer"
)

var validThreadRealtimeHandoffMode = map[ThreadRealtimeHandoffMode]struct{}{ThreadRealtimeHandoffModeStart: {}, ThreadRealtimeHandoffModeSteer: {}}

func (v ThreadRealtimeHandoffMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadRealtimeHandoffMode", v, validThreadRealtimeHandoffMode)
}
func (v *ThreadRealtimeHandoffMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadRealtimeHandoffMode", validThreadRealtimeHandoffMode, v)
}

type ThreadRealtimeSessionInputReason string

const (
	ThreadRealtimeSessionInputReasonChunk      ThreadRealtimeSessionInputReason = "chunk"
	ThreadRealtimeSessionInputReasonSessionEnd ThreadRealtimeSessionInputReason = "sessionEnd"
)

var validThreadRealtimeSessionInputReason = map[ThreadRealtimeSessionInputReason]struct{}{ThreadRealtimeSessionInputReasonChunk: {}, ThreadRealtimeSessionInputReasonSessionEnd: {}}

func (v ThreadRealtimeSessionInputReason) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadRealtimeSessionInputReason", v, validThreadRealtimeSessionInputReason)
}
func (v *ThreadRealtimeSessionInputReason) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadRealtimeSessionInputReason", validThreadRealtimeSessionInputReason, v)
}

type FrameThreadRealtimeItem struct {
	ID                string                      `json:"id"`
	RealtimeSessionID string                      `json:"realtimeSessionId"`
	ObservedAtMs      *int64                      `json:"observedAtMs,omitempty"`
	Channels          ThreadRealtimeFrameChannels `json:"channels"`
}

func (*FrameThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *FrameThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire FrameThreadRealtimeItem
	var decoded wire
	required := []string{"id", "realtimeSessionId", "channels"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateRecordField(data, "channels", recordRule{required: []string{"voice_transcript"}, children: map[string]recordRule{"voice_transcript": {required: []string{"role", "text"}}}}, false); err != nil {
		return err
	}
	*v = FrameThreadRealtimeItem(decoded)
	return nil
}
func (v *FrameThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	type wire FrameThreadRealtimeItem
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"frame", (*wire)(v)})
}

type TurnInputAcceptedThreadRealtimeItem struct {
	ID                string                    `json:"id"`
	RealtimeSessionID string                    `json:"realtimeSessionId"`
	ObservedAtMs      *int64                    `json:"observedAtMs,omitempty"`
	HandoffID         string                    `json:"handoff_id"`
	Input             string                    `json:"input"`
	InputItemID       string                    `json:"input_item_id"`
	Mode              ThreadRealtimeHandoffMode `json:"mode"`
	SourceEventIDs    []string                  `json:"source_event_ids"`
	TurnID            string                    `json:"turn_id"`
}

func (*TurnInputAcceptedThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *TurnInputAcceptedThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire TurnInputAcceptedThreadRealtimeItem
	var decoded wire
	required := []string{"id", "realtimeSessionId", "handoff_id", "input", "input_item_id", "mode", "source_event_ids", "turn_id"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateStringArrays(data, false, "source_event_ids"); err != nil {
		return err
	}
	*v = TurnInputAcceptedThreadRealtimeItem(decoded)
	return nil
}
func (v *TurnInputAcceptedThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	if v.SourceEventIDs == nil {
		return nil, fmt.Errorf("source_event_ids must be an array")
	}
	type wire TurnInputAcceptedThreadRealtimeItem
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"turnInputAccepted", (*wire)(v)})
}

type SessionInputSavedThreadRealtimeItem struct {
	ID                string                           `json:"id"`
	RealtimeSessionID string                           `json:"realtimeSessionId"`
	ObservedAtMs      *int64                           `json:"observedAtMs,omitempty"`
	Input             string                           `json:"input"`
	InputItemID       string                           `json:"input_item_id"`
	Reason            ThreadRealtimeSessionInputReason `json:"reason"`
	SourceEventIDs    []string                         `json:"source_event_ids"`
}

func (*SessionInputSavedThreadRealtimeItem) isThreadRealtimeItem() {}
func (v *SessionInputSavedThreadRealtimeItem) UnmarshalJSON(data []byte) error {
	type wire SessionInputSavedThreadRealtimeItem
	var decoded wire
	required := []string{"id", "realtimeSessionId", "input", "input_item_id", "reason", "source_event_ids"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	if err := validateStringArrays(data, false, "source_event_ids"); err != nil {
		return err
	}
	*v = SessionInputSavedThreadRealtimeItem(decoded)
	return nil
}
func (v *SessionInputSavedThreadRealtimeItem) MarshalJSON() ([]byte, error) {
	if v.SourceEventIDs == nil {
		return nil, fmt.Errorf("source_event_ids must be an array")
	}
	type wire SessionInputSavedThreadRealtimeItem
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{"sessionInputSaved", (*wire)(v)})
}
