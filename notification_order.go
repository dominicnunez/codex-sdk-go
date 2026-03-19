package codex

import "encoding/json"

func itemCompletedTurnKey(params json.RawMessage) string {
	carrier, ok := unmarshalThreadIDCarrier(params)
	if !ok || carrier.ThreadID == "" || carrier.TurnID == "" {
		return ""
	}
	return carrier.ThreadID + "\x00" + carrier.TurnID
}

func turnCompletedTurnKey(params json.RawMessage) string {
	carrier, ok := unmarshalTurnCompletedCarrier(params)
	if !ok || carrier.ThreadID == "" {
		return ""
	}
	turnID := extractRawTurnCompletedID(carrier.Turn)
	if turnID == "" {
		return ""
	}
	return carrier.ThreadID + "\x00" + turnID
}
