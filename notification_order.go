package codex

import "encoding/json"

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
