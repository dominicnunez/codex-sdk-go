package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"reflect"
	"strings"
	"testing"
)

func TestAgentDisplaySchemaBranches(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"mode":"show","version":1,"trustedContacts":"check"}`, true},
		{`{"mode":"show","version":1,"trustedContacts":"skip","annotation":{"kind":"wellbeingHelpline","subject":"answer"}}`, true},
		{`{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"}}`, true},
		{`{"mode":"block","version":1,"reason":{"kind":"protection","protectionType":""}}`, true},
		{`{"mode":"block","version":1,"reason":{"kind":"cyber","trustedAccess":false}}`, true},
		{`{"mode":"show","version":1,"trustedContacts":"check","reason":false,"replacement":[]}`, true},
		{`{"mode":"block","version":1,"reason":{"kind":"cyber","protectionType":false}}`, true},
		{`{"mode":"show","version":1}`, false},
		{`{"mode":"show","version":2,"trustedContacts":"check"}`, false},
		{`{"mode":"show","version":1,"trustedContacts":"future"}`, false},
		{`{"mode":"show","version":1,"trustedContacts":"check","annotation":{"kind":"wellbeingHelpline"}}`, false},
		{`{"mode":"hide","version":1,"replacement":{}}`, false},
		{`{"mode":"block","version":1,"reason":{"kind":"protection"}}`, false},
		{`{"mode":"block","version":1,"reason":{"kind":"future"}}`, false},
		{`{"action":"future"}`, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var item p.ThreadItemWrapper
			err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":true,"display":`+tc.body+`}}`), &item)
			if (err == nil) != tc.valid {
				t.Fatalf("admission=%v", err)
			}
			if !tc.valid {
				return
			}
			data, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			var again p.ThreadItemWrapper
			if err := json.Unmarshal(data, &again); err != nil {
				t.Fatalf("roundtrip %s: %v", data, err)
			}
		})
	}
}

func TestAgentDisplayMergesAndResets(t *testing.T) {
	for _, tc := range []struct {
		members string
		valid   bool
	}{
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1}},"MODERATION":{"DISPLAY":{"trustedContacts":"skip"}}`, true},
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check","annotation":{"kind":"wellbeingHelpline"}}},"moderation":{"display":{"annotation":{"subject":"prompt"}}}`, true},
		{`"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":{"kind":"protection"}}},"moderation":{"display":{"reason":{"protectionType":""}}}`, true},
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check"}},"moderation":{"display":null}`, true},
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check"}},"moderation":{"display":null},"moderation":{"display":{"mode":"show"}}`, false},
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":0,"trustedContacts":"check"}},"moderation":{"display":{"version":1}}`, false},
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"future"}},"moderation":{"display":{"trustedContacts":"check"}}`, false},
		{`"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":{"kind":"cyber","trustedAccess":null}}}`, true},
	} {
		var item p.ThreadItemWrapper
		err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &item)
		if (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.members, err)
		}
	}
}
func TestAgentDisplayPlainComposition(t *testing.T) {
	var envelope struct {
		p.AgentMessageDisplay
		Extra string `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"mode":"show","version":1,"trustedContacts":"check","extra":"kept"}`), &envelope); err != nil || envelope.Extra != "kept" || envelope.Mode != p.AgentMessageDisplayModeShow {
		t.Fatalf("envelope=%+v err=%v", envelope, err)
	}
}

func TestAgentDisplayLegacyErrorParity(t *testing.T) {
	for _, member := range []string{`"id":false`, `"moderation":false`, `"moderation":{"moderated":false,"display":false}`, `"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":false}}`, `"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":[]}}`, `"moderation":{"moderated":3}`, `"moderation":{"moderated":false,"display":{"action":3}}`, `"moderation":{"moderated":false,"display":{"checkTrustedContacts":"bad"}}`} {
		body := []byte(`{"type":"agentMessage","id":"i","text":"t",` + member + `}`)
		var direct p.AgentMessageThreadItem
		wantErr := json.Unmarshal(body, &direct)
		var item p.ThreadItemWrapper
		gotErr := json.Unmarshal(body, &item)
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) {
			t.Fatalf("%s native=%v owner=%v", body, wantErr, gotErr)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s native=%+v owner=%+v", body, *want, *got)
		}
	}
}
func TestAgentDisplayInactiveLegacyExtras(t *testing.T) {
	for _, member := range []string{`"action":false`, `"blockType":[]`, `"checkTrustedContacts":{}`, `"cyberTrustedAccess":"x"`} {
		body := []byte(`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check",` + member + `}}}`)
		var item p.ThreadItemWrapper
		if err := json.Unmarshal(body, &item); err != nil {
			t.Fatalf("inactive %s: %v", member, err)
		}
	}
}

func TestAgentDisplayPublicRoutes(t *testing.T) {
	for _, display := range []string{`{"mode":"show","version":1,"trustedContacts":"skip","annotation":{"kind":"wellbeingHelpline","subject":"prompt"}}`, `{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"}}`, `{"mode":"block","version":1,"reason":{"kind":"cyber","trustedAccess":false}}`, `{"mode":"show","version":1}`} {
		var check p.ThreadItemWrapper
		body := `{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":false,"display":` + display + `}}`
		valid := display != `{"mode":"show","version":1}`
		tr := NewMockTransport()
		calls, failures := 0, 0
		c := p.NewClient(tr, p.WithHandlerErrorCallback(func(_ string, _ error) { failures++ }))
		t.Cleanup(func() { _ = c.Close() })
		thread := validProcessThreadPayload("t")
		thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []json.RawMessage{json.RawMessage(body)}}}
		if err := tr.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
			t.Fatal(err)
		}
		read, err := c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
		if (err == nil) != valid {
			t.Fatalf("display=%s read=%v", display, err)
		}
		if valid {
			d := read.Thread.Turns[0].Items[0].Value.(*p.AgentMessageThreadItem).Moderation.Display
			if d.Mode == "" || d.Version == nil || *d.Version != 1 {
				t.Fatalf("lost display=%+v", d)
			}
			snapshot, ok := c.ThreadStateSnapshot("t")
			if !ok {
				t.Fatal("missing cache")
			}
			d.Mode = "changed"
			original := snapshot.Turns[0].Items[0].Value.(*p.AgentMessageThreadItem).Moderation.Display
			if original.Mode == "changed" {
				t.Fatal("response mutation leaked")
			}
		}
		observe := func(item p.ThreadItemWrapper) {
			calls++
			check = item
			d := item.Value.(*p.AgentMessageThreadItem).Moderation.Display
			if d.Mode == "" || d.Version == nil || *d.Version != 1 {
				t.Errorf("callback display=%+v", d)
			}
		}
		c.OnItemStarted(func(n p.ItemStartedNotification) { observe(n.Item) })
		c.AddItemStartedListener(func(n p.ItemStartedNotification) { observe(n.Item) })
		c.OnItemCompleted(func(n p.ItemCompletedNotification) { observe(n.Item) })
		c.AddItemCompletedListener(func(n p.ItemCompletedNotification) { observe(n.Item) })
		for _, method := range []string{p.NotifyItemStarted, p.NotifyItemCompleted} {
			tr.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"completedAtMs":0,"item":` + body + `}`)})
		}
		if valid && (calls != 4 || check.Value == nil) || !valid && (calls != 0 || failures != 4) {
			t.Fatalf("valid=%v calls=%d failures=%d", valid, calls, failures)
		}
	}
}

func FuzzAgentDisplayBranches(f *testing.F) {
	for a := range 4 {
		for b := range 7 {
			f.Add(uint8(a), uint8(b), uint8(0), "opaque")
		}
	}
	f.Fuzz(func(t *testing.T, a, b, c uint8, text string) {
		modes := []string{"show", "hide", "block", "future"}
		versions := []any{nil, 0, 1, 2, 256, false, "1"}
		contacts := []any{"check", "skip", nil, true, "future"}
		annotations := []any{nil, map[string]any{"kind": "wellbeingHelpline", "subject": "answer"}, map[string]any{"kind": "wellbeingHelpline", "subject": "prompt"}, map[string]any{"kind": "wellbeingHelpline"}, false}
		replacements := []any{map[string]any{"kind": "wellbeingPromptHelpline"}, nil, map[string]any{"kind": "future"}, true}
		reasons := []any{map[string]any{"kind": "protection", "protectionType": text}, map[string]any{"kind": "cyber", "trustedAccess": false, "protectionType": false}, map[string]any{"kind": "protection"}, nil, map[string]any{"kind": "future"}}
		mode := modes[int(a)%len(modes)]
		version := versions[int(b)%len(versions)]
		contact := contacts[int(c)%len(contacts)]
		annotation := annotations[int(b)%len(annotations)]
		replacement := replacements[int(c)%len(replacements)]
		reason := reasons[int(c)%len(reasons)]
		display := map[string]any{"mode": mode, "version": version, "trustedContacts": contact, "annotation": annotation, "replacement": replacement, "reason": reason}
		body, err := json.Marshal(map[string]any{"type": "agentMessage", "id": "i", "text": "t", "moderation": map[string]any{"moderated": false, "display": display}})
		if err != nil {
			t.Fatal(err)
		}
		valid := version == 1
		switch mode {
		case "show":
			valid = valid && (contact == "check" || contact == "skip") && (int(b)%len(annotations) < 3)
		case "hide":
			valid = valid && int(c)%len(replacements) == 0
		case "block":
			valid = valid && int(c)%len(reasons) < 2
		default:
			valid = false
		}
		var item p.ThreadItemWrapper
		err = json.Unmarshal(body, &item)
		if (err == nil) != valid {
			t.Fatalf("valid=%v err=%v body=%s", valid, err, body)
		}
		if !valid {
			return
		}
		d := item.Value.(*p.AgentMessageThreadItem).Moderation.Display
		if string(d.Mode) != mode || d.Version == nil || *d.Version != 1 {
			t.Fatalf("changed display=%+v", d)
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var round p.ThreadItemWrapper
		if err := json.Unmarshal(encoded, &round); err != nil {
			t.Fatalf("roundtrip %s: %v", encoded, err)
		}
	})
}

func TestAgentDisplayLegacyNewExtrasAndFinalReset(t *testing.T) {
	for _, members := range []string{
		`"moderation":{"moderated":false,"display":{"action":"future","version":"opaque"}}`,
		`"moderation":{"moderated":false,"display":{"action":"future","version":999}}`,
		`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"skip","action":null}},"moderation":{"display":null}`,
		`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"skip"}},"moderation":{"display":null},"moderation":{"display":{"action":"future","version":"opaque"}}`,
	} {
		var item p.ThreadItemWrapper
		if err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+members+`}`), &item); err != nil {
			t.Fatalf("%s: %v", members, err)
		}
	}
}

func TestAgentDisplayActiveFailuresSurviveReset(t *testing.T) {
	for _, display := range []string{
		`{"mode":"show","version":1,"trustedContacts":"future"}`,
		`{"mode":"show","version":1,"trustedContacts":"check","annotation":{"kind":"future","subject":"answer"}}`,
		`{"mode":"show","version":1,"trustedContacts":"check","annotation":{"kind":"wellbeingHelpline","subject":null}}`,
		`{"mode":"hide","version":1,"replacement":null}`,
		`{"mode":"hide","version":1,"replacement":{"kind":"future"}}`,
		`{"mode":"block","version":1,"reason":{"kind":"protection","protectionType":null}}`,
		`{"mode":"block","version":1,"reason":{"kind":"cyber","trustedAccess":"bad"}}`,
		`{"mode":"block","version":1,"reason":{"kind":"future"}}`,
	} {
		var item p.ThreadItemWrapper
		body := []byte(`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":false,"display":` + display + `},"moderation":{"display":null}}`)
		if err := json.Unmarshal(body, &item); err == nil {
			t.Fatalf("reset erased active failure %s", display)
		}
	}
	for _, display := range []string{
		`{"mode":"show","version":1,"trustedContacts":"check","reason":false,"action":null}`,
		`{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"annotation":false}`,
		`{"mode":"block","version":1,"reason":{"kind":"cyber","protectionType":false}}`,
	} {
		var item p.ThreadItemWrapper
		body := []byte(`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":false,"display":` + display + `},"moderation":{"display":null}}`)
		if err := json.Unmarshal(body, &item); err != nil {
			t.Fatalf("inactive extra rejected %s: %v", display, err)
		}
	}
}

func TestAgentDisplayInactiveExtrasAcrossResets(t *testing.T) {
	for _, members := range []string{
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"annotation":false}},"moderation":{"display":null},"moderation":{"display":{"mode":"show","version":1,"trustedContacts":"check"}}`,
		`"moderation":{"moderated":false,"display":{"action":"future","version":"opaque","annotation":false}},"moderation":{"display":null},"moderation":{"display":{"mode":"show","version":1,"trustedContacts":"check"}}`,
		`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check","action":false}},"moderation":{"display":null},"moderation":{"display":{"action":"future"}}`,
	} {
		var item p.ThreadItemWrapper
		body := []byte(`{"type":"agentMessage","id":"i","text":"t",` + members + `}`)
		if err := json.Unmarshal(body, &item); err != nil {
			t.Fatalf("inactive prior extra affected new segment %s: %v", members, err)
		}
	}
}

func TestAgentDisplayLateSelectorsAndModeChanges(t *testing.T) {
	for _, members := range []string{
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"trustedContacts":"future"}},"moderation":{"display":{"mode":"show","trustedContacts":"check"}}`,
		`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check","annotation":{"kind":"wellbeingHelpline","subject":"answer"}}},"moderation":{"display":{"mode":"hide","replacement":{"kind":"wellbeingPromptHelpline"},"annotation":{"kind":"future"}}},"moderation":{"display":{"mode":"show","annotation":{"kind":"wellbeingHelpline"}}}`,
		`"moderation":{"moderated":false,"display":{"trustedContacts":"check","version":1}},"moderation":{"display":{"mode":"show"}}`,
		`"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":{"protectionType":""}}},"moderation":{"display":{"reason":{"kind":"protection"}}}`,
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"reason":{"kind":"cyber","trustedAccess":false}}},"moderation":{"display":{"mode":"block"}}`,
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"trustedContacts":"check"}},"moderation":{"display":{"mode":"show"}}`,
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"annotation":{"kind":"wellbeingHelpline","subject":false}}},"moderation":{"display":{"mode":"show","trustedContacts":"check","annotation":{"subject":"answer"}}}`,
		`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"reason":{"kind":false,"trustedAccess":false}}},"moderation":{"display":{"mode":"block","reason":{"kind":"cyber"}}}`,
		`"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":{"kind":"cyber","trustedAccess":false,"protectionType":""}}},"moderation":{"display":{"reason":{"kind":"protection"}}}`,
	} {
		var item p.ThreadItemWrapper
		body := []byte(`{"type":"agentMessage","id":"i","text":"t",` + members + `}`)
		if err := json.Unmarshal(body, &item); err != nil {
			t.Errorf("native selected merge rejected %s: %v", members, err)
		}
	}
}

func TestAgentDisplayInactivePointerResets(t *testing.T) {
	for _, tc := range []struct {
		members string
		valid   bool
	}{
		{`"moderation":{"moderated":false,"display":{"mode":"show","version":1,"trustedContacts":"check"}},"moderation":{"display":{"mode":"hide","trustedContacts":null,"replacement":{"kind":"wellbeingPromptHelpline"}}},"moderation":{"display":{"mode":"show"}}`, false},
		{`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"trustedContacts":null,"replacement":{"kind":"wellbeingPromptHelpline"}}},"moderation":{"display":{"mode":"show","trustedContacts":"check"}}`, true},
		{`"moderation":{"moderated":false,"display":{"mode":"block","version":1,"reason":{"kind":"cyber"}}},"moderation":{"display":{"mode":"hide","reason":null,"replacement":{"kind":"wellbeingPromptHelpline"}}},"moderation":{"display":{"mode":"block"}}`, false},
		{`"moderation":{"moderated":false,"display":{"mode":"hide","version":1,"reason":null,"replacement":{"kind":"wellbeingPromptHelpline"}}},"moderation":{"display":{"mode":"block","reason":{"kind":"cyber"}}}`, true},
	} {
		var item p.ThreadItemWrapper
		err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &item)
		if (err == nil) != tc.valid {
			t.Errorf("valid=%v err=%v body=%s", tc.valid, err, tc.members)
		}
	}
}

func TestAgentDisplayInactiveSiblingsRemainOwnedValues(t *testing.T) {
	body := []byte(`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":false,"display":{"mode":"hide","version":1,"replacement":{"kind":"wellbeingPromptHelpline"},"reason":{"kind":false,"trustedAccess":false}}},"moderation":{"display":{"mode":"block","reason":{"kind":"cyber"}}}}`)
	var item p.ThreadItemWrapper
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	d := item.Value.(*p.AgentMessageThreadItem).Moderation.Display
	if d.Reason == nil || d.Reason.TrustedAccess == nil || *d.Reason.TrustedAccess {
		t.Fatalf("valid sibling false was lost: %+v", d.Reason)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var again p.ThreadItemWrapper
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatal(err)
	}
	r := again.Value.(*p.AgentMessageThreadItem).Moderation.Display.Reason
	if r.TrustedAccess == nil || *r.TrustedAccess {
		t.Fatalf("false sibling changed on roundtrip: %s", encoded)
	}
}

func TestAgentDisplayPreservesModerationPrecedence(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{"type":"agentMessage","id":"i","text":"t","moderation":{"display":{"mode":"future","version":1,"trustedContacts":"check"}}}`, "moderation.moderated is required"},
		{`{"type":"agentMessage","id":"i","text":"t","moderation":{"moderated":null,"display":{"mode":"show","version":0,"trustedContacts":"check"}}}`, "moderation.moderated must not be null"},
	} {
		var v p.ThreadItemWrapper
		err := json.Unmarshal([]byte(tc.body), &v)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("got %v, want %s", err, tc.want)
		}
	}
}

func TestAgentDisplayLegacyNullOrderAndWrappers(t *testing.T) {
	for _, extra := range []string{"", `,"version":"opaque"`} {
		for _, tc := range []struct{ members, want string }{
			{`"moderation":{"moderated":null,"display":{"action":null` + extra + `}}`, "moderation: moderation.moderated must not be null"},
			{`"moderation":{"display":{"action":null` + extra + `},"moderated":null}`, "moderation: moderation.display.action must not be null"},
			{`"moderation":{"display":{"action":null` + extra + `}}`, "moderation: moderation.display.action must not be null"},
		} {
			var v p.ThreadItemWrapper
			err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &v)
			if err == nil || err.Error() != tc.want {
				t.Errorf("members=%s got %v, want %s", tc.members, err, tc.want)
			}
		}
	}
	for _, tc := range []struct{ members, want string }{
		{`"moderation":{"display":{"action":null}},"moderation":{"display":null},"moderation":{"display":{"mode":"future"}}`, "moderation: moderation.display.action must not be null"},
		{`"moderation":{"moderated":null,"display":{"action":null}},"moderation":{"display":null},"moderation":{"display":{"mode":"future"}}`, "moderation: moderation.moderated must not be null"},
		{`"moderation":{"display":{"mode":"show","version":1,"trustedContacts":"check","action":null},"moderated":null}`, "moderation: moderation.moderated must not be null"},
	} {
		var v p.ThreadItemWrapper
		err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &v)
		if err == nil || err.Error() != tc.want {
			t.Errorf("members=%s got %v, want %s", tc.members, err, tc.want)
		}
	}
}

func TestAgentDisplayCurrentDetectionUsesNativeNamesAndHistory(t *testing.T) {
	for _, tc := range []struct {
		members string
		valid   bool
	}{
		{`"MODERATION":{"moderated":false,"DISPLAY":{"MODE":"show","VERSION":1,"TRUSTEDCONTACTS":"check"}}`, true},
		{`"moderation":{"moderated":false,"display":{"mo\u0064e":"show","ver\u0073ion":1,"trustedContacts":"check"}}`, true},
		{`"MODERATION":{"moderated":false,"DISPLAY":{"MODE":"future"}},"moderation":{"display":null}`, false},
		{`"moderation":{"moderated":false,"display":{"mo\u0064e":"show","version":1,"trustedContacts":"future"}},"moderation":null`, false},
		{`"moderation":{"moderated":false,"display":{"action":"future","version":"opaque","reason":false,"annotation":false}}`, true},
		{`"moderation":{"moderated":false,"display":{"action":"future","unused":{"mode":"future"}}}`, true},
	} {
		var v p.ThreadItemWrapper
		err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &v)
		if (err == nil) != tc.valid {
			t.Errorf("valid=%v err=%v members=%s", tc.valid, err, tc.members)
		}
	}
}
