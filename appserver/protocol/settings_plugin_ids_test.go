package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func disabledPluginSettingsPayload(t *testing.T, members string) json.RawMessage {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(collabSettingsBody(`{"mode":"default","settings":{"model":"model"}}`), &envelope); err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(envelope["threadSettings"], &settings); err != nil {
		t.Fatal(err)
	}
	delete(settings, "disabledPluginIds")
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if members != "" {
		raw = append(raw[:len(raw)-1], []byte(","+members+"}")...)
	}
	return json.RawMessage(`{"threadId":"collab-thread","threadSettings":` + string(raw) + `}`)
}

func TestSettingsPluginIDsRejectNulls(t *testing.T) {
	for _, members := range []string{
		`"disabledPluginIds":null`,
		`"disabledPluginIds":null,"disabledPluginIds":[]`,
		`"disabledPluginIds":null,"disabledPluginIds":["new"]`,
		`"disabledPluginIds":["new"],"disabledPluginIds":null`,
		`"disabledPluginIds":[null]`,
		`"disabledPluginIds":["new",null]`,
		`"disabledPluginIds":[null,"new"]`,
		`"disabledPluginIds":[null],"disabledPluginIds":[]`,
	} {
		t.Run(members, func(t *testing.T) {
			mock := NewMockTransport()
			on, add, reports := 0, 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
				if method != "thread/settings/updated" {
					t.Errorf("diagnostic origin=%s", method)
				}
				reports++
			}))
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "collab-thread", DisabledPluginIDs: []string{"keep"}})
			client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { on++ })
			remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { add++ })
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: disabledPluginSettingsPayload(t, members)})
			snapshot, ok := client.ThreadStateSnapshot("collab-thread")
			if on != 0 || add != 0 || reports == 0 || !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"keep"}) {
				t.Errorf("invalid plugin array published: on=%d add=%d reports=%d ids=%v", on, add, reports, snapshot.DisabledPluginIDs)
			}
			before := reports
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: disabledPluginSettingsPayload(t, `"disabledPluginIds":["recovered"]`)})
			snapshot, _ = client.ThreadStateSnapshot("collab-thread")
			if on != 1 || add != 1 || reports != before || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"recovered"}) {
				t.Error("same-client valid recovery failed")
			}
		})
	}
}

func TestSettingsPluginIDsValidControls(t *testing.T) {
	for _, tc := range []struct {
		members string
		want    []string
	}{
		{"", nil}, {`"disabledPluginIds":[]`, []string{}},
		{`"disabledPluginIds":["plugin-a","plugin-b"]`, []string{"plugin-a", "plugin-b"}},
		{`"disabledPluginIds":[""]`, []string{""}},
	} {
		t.Run(tc.members, func(t *testing.T) {
			mock := NewMockTransport()
			calls, reports := 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { reports++ }))
			defer client.Close()
			client.CacheThreadState(p.Thread{ID: "collab-thread", DisabledPluginIDs: []string{"keep"}})
			check := func(n p.ThreadSettingsUpdatedNotification) {
				calls++
				if !reflect.DeepEqual(n.ThreadSettings.DisabledPluginIDs, tc.want) {
					t.Errorf("callback IDs=%v want%v", n.ThreadSettings.DisabledPluginIDs, tc.want)
				}
			}
			client.OnThreadSettingsUpdated(check)
			remove := client.AddThreadSettingsUpdatedListener(check)
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: disabledPluginSettingsPayload(t, tc.members)})
			snapshot, ok := client.ThreadStateSnapshot("collab-thread")
			wantCache := tc.want
			if wantCache == nil {
				// Settings cache admission represents the authoritative default as
				// an explicit empty list, while callback decoding preserves omission.
				wantCache = []string{}
			}
			if calls != 2 || reports != 0 || !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, wantCache) {
				t.Fatalf("valid IDs rejected/changed: calls=%d reports=%d ids=%v", calls, reports, snapshot.DisabledPluginIDs)
			}
		})
	}
}
