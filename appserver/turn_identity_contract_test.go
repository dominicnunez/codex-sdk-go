package appserver

import (
	"encoding/json"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestNestedTurnIdentityMatchesTypedAndFallback(t *testing.T) {
	for _, fields := range []string{
		`"id":"real","ID":"foreign"`,
		`"ID":"foreign","id":"real"`,
		`"id":"real","ID":null`,
		`"id":"old","\u0069d":"real"`,
	} {
		var start protocol.TurnStartResponse
		if err := json.Unmarshal([]byte(`{"turn":{`+fields+`,"status":"inProgress","items":[]}}`), &start); err != nil {
			t.Fatal(err)
		}
		for _, items := range []string{"[]", "null"} {
			params := json.RawMessage(`{"threadId":"thread","turn":{` + fields + `,"status":"completed","items":` + items + `}}`)
			got, ok, err := parseTurnCompletedForThread(params, "thread", false)
			if !ok || got.turnID != start.Turn.ID || !matchesActiveTurn(start.Turn.ID, got) || (err != nil) != (items == "null") {
				t.Fatalf("fields=%s items=%s start=%q completion=%+v ok=%v err=%v", fields, items, start.Turn.ID, got, ok, err)
			}
		}
	}
}

func TestMalformedTurnIdentityRecoveryRetainsPriorPolicy(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{`{"ID":"active","items":null}`, "active"},
		{`{"id":"active","id":null,"items":null}`, "active"},
		{`{"id":null,"ID":"active","items":null}`, "active"},
		{`{"id":"active","ID":99,"items":null}`, ""},
		{`{"id":99,"id":"active","items":null}`, ""},
		{`{"id":null,"items":null}`, ""},
	} {
		if got := extractRawTurnCompletedID(json.RawMessage(tc.data)); got != tc.want {
			t.Fatalf("data=%s ID=%q want=%q", tc.data, got, tc.want)
		}
	}
}
