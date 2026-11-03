package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestCollabAdmissionNativeHistories(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"mode":"default","settings":{"model":"m"},"settings":{}}`, true},
		{`{"mode":"default","SETTINGS":{"MODEL":""}}`, true},
		{`{"MODE":"plan","\u017fettings":{"model":"m"}}`, true},
		{`{"mode":"default","settings":{"model":"m"},"settings":{"model":null}}`, false},
		{`{"mode":"default","settings":{"mo\u0064el":null,"model":"m"}}`, false},
		{`{"mode":"default","settings":null,"settings":{"model":"m"}}`, false},
		{`{"mode":"default","settings":{"model":"m"},"SETTINGS":null}`, false},
		{`{"mode":"default","settings":{"model":"m","MODEL":null}}`, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			mock := NewMockTransport()
			calls, reports := 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { reports++ }))
			defer client.Close()
			collabResumeResponse(t, mock, tc.raw)
			result, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
			if (err == nil) != tc.valid {
				t.Fatalf("resume history valid=%t err=%v", tc.valid, err)
			}
			if !tc.valid {
				collabIncompleteNotification(t, tc.raw)
				return
			}
			var reference p.CollaborationMode
			if err := json.Unmarshal([]byte(tc.raw), &reference); err != nil {
				t.Fatal(err)
			}
			if result.CollaborationMode == nil || !reflect.DeepEqual(*result.CollaborationMode, reference) {
				t.Fatal("resume values disagree with ordinary native record")
			}
			check := func(n p.ThreadSettingsUpdatedNotification) {
				calls++
				if !reflect.DeepEqual(n.ThreadSettings.CollaborationMode, reference) {
					t.Error("notification values disagree with ordinary native record")
				}
			}
			client.OnThreadSettingsUpdated(check)
			remove := client.AddThreadSettingsUpdatedListener(check)
			defer remove()

			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: collabSettingsBody(tc.raw)})
			if calls != 2 || reports != 0 {
				t.Fatalf("valid notification history calls=%d reports=%d", calls, reports)
			}
		})
	}
}

func resumeWithRawCollabMembers(t *testing.T, members string) error {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	body := validThreadLifecycleResponse(validProcessThreadPayload("collab-thread"))
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	mock.SetResponse("thread/resume", p.Response{Result: json.RawMessage(string(raw[:len(raw)-1]) + "," + members + "}")})
	_, err = client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
	return err
}

func TestCollabAdmissionOuterMatchingAndReset(t *testing.T) {
	for _, tc := range []struct {
		members string
		valid   bool
	}{
		{`"collaborationMode":null`, true},
		{`"COLLABORATIONMODE":{"mode":"default","settings":{"model":"m"}}`, true},
		{`"collaborationMode":{"mode":"default"},"collaborationMode":{"settings":{"model":"m"}}`, true},
		{`"collaborationMode":{"mode":"default"},"COLLABORATIONMODE":null`, true},
		{`"collaborationMode":{"mode":"default","settings":{"model":"m"}},"COLLABORATIONMODE":null,"collaborationMode":{"settings":{"model":"m"}}`, false},
		{`"collaborationMode":{"settings":null},"collaborationMode":null`, false},
		{`"COLLABORATIONMODE":{"mode":"default","settings":{}}`, false},
	} {
		if err := resumeWithRawCollabMembers(t, tc.members); (err == nil) != tc.valid {
			t.Errorf("outer/reset valid=%t err=%v input=%s", tc.valid, err, tc.members)
		}
	}
	// ThreadSettings' owner ignores outer differently cased extras. Its inner
	// plain records still recognize native aliases, as exercised above.
	body := collabSettingsBody(`{"mode":"default","settings":{"model":"m"}}`)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	settings := envelope["threadSettings"]
	settings = append(settings[:len(settings)-1], []byte(`,"COLLABORATIONMODE":{"settings":null}}`)...)
	var value p.ThreadSettings
	if err := json.Unmarshal(settings, &value); err != nil || value.CollaborationMode.Settings.Model != "m" {
		t.Fatalf("ignored settings outer extra affected admission: %v", err)
	}
}

func TestCollabAdmissionRequiredOuterPolicies(t *testing.T) {
	collabIncompleteNotification(t, "null")
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(collabSettingsBody(`{"mode":"default","settings":{"model":"m"}}`), &envelope); err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(envelope["threadSettings"], &settings); err != nil {
		t.Fatal(err)
	}
	delete(settings, "collaborationMode")
	settings["COLLABORATIONMODE"] = json.RawMessage(`{"mode":"default","settings":{"model":"m"}}`)
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var target p.ThreadSettings
	if err := json.Unmarshal(raw, &target); err == nil {
		t.Fatal("ignored outer alias supplied required canonical collaborationMode")
	}
}
