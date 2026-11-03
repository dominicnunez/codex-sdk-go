package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func collabResumeResponse(t *testing.T, mock *MockTransport, raw string) {
	t.Helper()
	thread := validProcessThreadPayload("collab-thread")
	thread["name"] = "recovered"
	body := validThreadLifecycleResponse(thread)
	body["collaborationMode"] = json.RawMessage(raw)
	if err := mock.SetResponseData("thread/resume", body); err != nil {
		t.Fatal(err)
	}
}

func collabSettingsBody(raw string) json.RawMessage {
	return json.RawMessage(`{"threadId":"collab-thread","threadSettings":{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"provider","sandboxPolicy":{"type":"readOnly"},"collaborationMode":` + raw + `,"disabledPluginIds":["updated"]}}`)
}

func TestCollabAdmissionRejectsIncomplete(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"settings":{"model":"model"}}`, `{"mode":"default"}`,
		`{"mode":"default","settings":null}`, `{"mode":"default","settings":{}}`,
		`{"mode":"default","settings":{"model":null}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "collab-thread", Name: p.Ptr("prior")})
			updates := 0
			remove := client.AddThreadStateListener("collab-thread", func(p.Thread) { updates++ }, nil)
			defer remove()
			updates = 0
			collabResumeResponse(t, mock, raw)
			result, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
			if err == nil || !reflect.ValueOf(result).IsZero() || updates != 0 {
				t.Errorf("incomplete collaboration published byresume: updates=%d err=%v", updates, err)
			}
			snapshot, ok := client.ThreadStateSnapshot("collab-thread")
			if !ok || snapshot.Name == nil || *snapshot.Name != "prior" {
				t.Error("incomplete response changed cache")
			}
			collabResumeResponse(t, mock, `{"mode":"default","settings":{"model":""}}`)
			if _, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"}); err != nil || len(mock.SentRequests) != 2 {
				t.Fatalf("valid resume recovery: %v", err)
			}
			collabIncompleteNotification(t, raw)
		})
	}
}

func collabIncompleteNotification(t *testing.T, raw string) {
	t.Helper()
	mock := NewMockTransport()
	on, add, reports := 0, 0, 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
		if method != "thread/settings/updated" {
			t.Errorf("diagnostic origin=%s", method)
		}
		reports++
	}))
	defer client.Close()
	client.CacheThreadState(p.Thread{ID: "collab-thread", DisabledPluginIDs: []string{"prior"}})
	client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { on++ })
	remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { add++ })
	defer remove()
	mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: collabSettingsBody(raw)})
	snapshot, ok := client.ThreadStateSnapshot("collab-thread")
	if on != 0 || add != 0 || reports == 0 || !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"prior"}) {
		t.Errorf("incomplete collaboration published callbacks/cache: on=%d add=%d reports=%d", on, add, reports)
	}
	before := reports
	mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: collabSettingsBody(`{"mode":"plan","settings":{"model":""}}`)})
	if on != 1 || add != 1 || reports != before {
		t.Errorf("notification recovery failed: on=%d add=%d reports=%d", on, add, reports)
	}
}

func TestCollabAdmissionValidControls(t *testing.T) {
	for _, raw := range []string{
		`{"mode":"default","settings":{"model":"model"}}`,
		`{"mode":"plan","settings":{"model":""}}`,
		`{"mode":"default","settings":{"model":"model","developer_instructions":null,"reasoning_effort":null}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			mock := NewMockTransport()
			calls, reports := 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { reports++ }))
			defer client.Close()
			collabResumeResponse(t, mock, raw)
			if _, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"}); err != nil {
				t.Fatal(err)
			}
			client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { calls++ })
			remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { calls++ })
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: collabSettingsBody(raw)})
			if calls != 2 || reports != 0 {
				t.Fatalf("valid notification rejected: calls=%d reports=%d", calls, reports)
			}
		})
	}
}
