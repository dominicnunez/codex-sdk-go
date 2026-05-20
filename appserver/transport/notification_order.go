package transport

import "encoding/json"

type threadIDCarrier struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type rawTurnCompletedCarrier struct {
	ThreadID string          `json:"threadId"`
	Turn     json.RawMessage `json:"turn"`
}

func unmarshalThreadIDCarrier(params json.RawMessage) (threadIDCarrier, bool) {
	var carrier threadIDCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return threadIDCarrier{}, false
	}
	return carrier, true
}

func unmarshalTurnCompletedCarrier(params json.RawMessage) (rawTurnCompletedCarrier, bool) {
	var carrier rawTurnCompletedCarrier
	if err := json.Unmarshal(params, &carrier); err != nil {
		return rawTurnCompletedCarrier{}, false
	}
	return carrier, true
}

func itemCompletedThreadKey(params json.RawMessage) string {
	carrier, ok := unmarshalThreadIDCarrier(params)
	if !ok || carrier.ThreadID == "" {
		return ""
	}
	return carrier.ThreadID
}

func turnCompletedThreadKey(params json.RawMessage) string {
	carrier, ok := unmarshalTurnCompletedCarrier(params)
	if !ok || carrier.ThreadID == "" {
		return ""
	}
	return carrier.ThreadID
}
