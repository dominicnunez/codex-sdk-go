package appserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func denseLifecycleObject(prefix, suffix string, count int) json.RawMessage {
	var data strings.Builder
	data.WriteString(prefix)
	for i := range count {
		fmt.Fprintf(&data, `,"metadata%d":0`, i)
	}
	data.WriteString(suffix)
	return json.RawMessage(data.String())
}

func TestLifecycleIdentitySelectionAllocations(t *testing.T) {
	if lifecycleRaceEnabled {
		t.Skip("measure scanner-pool allocations without race instrumentation")
	}
	malformedItem := `"item":{"type":"agentMessage","id":99,"text":"bad"}`
	item := denseLifecycleObject(`{"threadId":"thread","turnId":"turn",`+malformedItem, `}`, 50000)
	lateItem := denseLifecycleObject(`{"threadId":"thread","turnId":"turn"`, `,`+malformedItem+`}`, 50000)
	turn := denseLifecycleObject(`{"id":"turn","status":99`, `,"items":[]}`, 50000)
	completion := append(json.RawMessage(`{"threadId":"thread","turn":`), turn...)
	completion = append(completion, '}')
	thread := denseLifecycleObject(`{"threadId":"thread","turnId":"turn"`, `}`, 50000)
	outerTurn := denseLifecycleObject(`{"threadId":"thread","turn":{"id":"turn"}`, `}`, 50000)
	for name, check := range map[string]func(){
		"thread carrier": func() {
			got, ok := unmarshalThreadIDCarrier(thread)
			if !ok || got.ThreadID != "thread" || got.TurnID != "turn" {
				t.Fatal(got, ok)
			}
		},
		"item carrier": func() {
			got, ok := unmarshalItemCompletedCarrier(item)
			if !ok || got.ThreadID != "thread" || got.TurnID != "turn" {
				t.Fatal(got, ok)
			}
		},
		"turn carrier": func() {
			got, ok := unmarshalTurnCompletedCarrier(outerTurn)
			if !ok || got.ThreadID != "thread" {
				t.Fatal(got, ok)
			}
		},
		"nested turn identity": func() {
			if got := extractRawTurnCompletedID(turn); got != "turn" {
				t.Fatal(got)
			}
		},
		"item lifecycle parse early failure": func() {
			got, ok, err := parseItemCompletedForThread(item, "thread")
			if !ok || err == nil || got.ThreadID != "thread" || got.TurnID != "turn" {
				t.Fatal(got, ok, err)
			}
		},
		"item lifecycle parse late failure": func() {
			got, ok, err := parseItemCompletedForThread(lateItem, "thread")
			if !ok || err == nil || got.ThreadID != "thread" || got.TurnID != "turn" {
				t.Fatal(got, ok, err)
			}
		},
		"turn lifecycle parse": func() {
			got, ok, err := parseTurnCompletedForThread(completion, "thread", false)
			if !ok || err == nil || got.turnID != "turn" {
				t.Fatal(got, ok, err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(3, check)
			if allocations > 100 {
				t.Fatalf("identity selection allocated %.0f times for ignored metadata; want <=100", allocations)
			}
		})
	}
}

func FuzzLifecycleExactCarrier(f *testing.F) {
	for _, data := range []string{
		`{"threadId":"a","ThreadID":"b","turnId":"t","item":{"id":"i"}}`,
		`{"threadId":99,"threadId":"a","turnId":null,"item":null}`,
		`{"threadId":"a","threadId":null,"turnId":"t","item":[],"item":{}}`,
		`{"\u0074hreadId":"a","turnId":"t","item":{}}`,
		`null`, `[]`, `{`,
	} {
		f.Add([]byte(data))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		var fields map[string]json.RawMessage
		wantOK := json.Unmarshal(data, &fields) == nil && fields != nil
		var want rawItemCompletedCarrier
		for name, dest := range map[string]any{"threadId": &want.ThreadID, "turnId": &want.TurnID, "item": &want.Item} {
			if raw, exists := fields[name]; exists && json.Unmarshal(raw, dest) != nil {
				wantOK = false
			}
		}
		got, ok := unmarshalItemCompletedCarrier(data)
		if ok != wantOK || (ok && (got.ThreadID != want.ThreadID || got.TurnID != want.TurnID || string(got.Item) != string(want.Item))) {
			t.Fatalf("carrier = %#v,%v; independent map reference = %#v,%v", got, ok, want, wantOK)
		}
	})
}
