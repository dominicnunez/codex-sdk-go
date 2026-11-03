package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func pluginIDsSettingsRecord(t *testing.T, members string) json.RawMessage {
	t.Helper()
	var envelope struct {
		Settings json.RawMessage `json:"threadSettings"`
	}
	if err := json.Unmarshal(disabledPluginSettingsPayload(t, members), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Settings
}

func TestSettingsPluginIDsExactAndEscapedNames(t *testing.T) {
	for _, tc := range []struct {
		members string
		want    []string
	}{
		{`"disabledPluginIds":["canonical"],"DISABLEDPLUGINIDS":["ignored"]`, []string{"canonical"}},
		{`"disabledPluginIds":["canonical"],"disabledPluginids":null`, []string{"canonical"}},
		{`"DISABLEDPLUGINIDS":[null]`, nil},
		{`"disabledPlugin\u0049ds":[""]`, []string{""}},
		{`"disabledPluginIds":["first"],"disabledPluginIds":["last","null","日本語"]`, []string{"last", "null", "日本語"}},
	} {
		t.Run(tc.members, func(t *testing.T) {
			var direct p.ThreadSettings
			if err := json.Unmarshal(pluginIDsSettingsRecord(t, tc.members), &direct); err != nil || !reflect.DeepEqual(direct.DisabledPluginIDs, tc.want) {
				t.Fatalf("record selection changed: %v ids=%v", err, direct.DisabledPluginIDs)
			}
			mock := NewMockTransport()
			calls, reports := 0, 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { reports++ }))
			defer client.Close()
			check := func(n p.ThreadSettingsUpdatedNotification) {
				calls++
				if !reflect.DeepEqual(n.ThreadSettings.DisabledPluginIDs, tc.want) {
					t.Error("callback field selection changed")
				}
			}
			client.OnThreadSettingsUpdated(check)
			remove := client.AddThreadSettingsUpdatedListener(check)
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: disabledPluginSettingsPayload(t, tc.members)})
			if calls != 2 || reports != 0 {
				t.Fatalf("valid exact-name notification rejected: calls=%d reports=%d", calls, reports)
			}
		})
	}
	for _, members := range []string{`"disabledPlugin\u0049ds":null`, `"disabledPlugin\u0049ds":[null],"disabledPluginIds":["later"]`} {
		var direct p.ThreadSettings
		if err := json.Unmarshal(pluginIDsSettingsRecord(t, members), &direct); err == nil {
			t.Fatal("escaped canonical null bypassed array admission")
		}
	}
}

func TestSettingsPluginIDsReceiverAndPriorErrors(t *testing.T) {
	ids := []string{"prior"}
	value := p.ThreadSettings{Model: "prior", DisabledPluginIDs: ids, CollaborationMode: p.CollaborationMode{Mode: p.ModeKindDefault, Settings: p.CollaborationModeSettings{Model: "prior"}}}
	before := value
	raw := pluginIDsSettingsRecord(t, `"disabledPluginIds":[null]`)
	if err := json.Unmarshal(raw, &value); err == nil || !reflect.DeepEqual(value, before) || &value.DisabledPluginIDs[0] != &ids[0] {
		t.Fatal("new rejection changed prior receiver/reference")
	}
	badPath := strings.Replace(string(raw), `"cwd":"/tmp"`, `"cwd":"relative"`, 1)
	if err := json.Unmarshal([]byte(badPath), &value); err == nil || !strings.Contains(err.Error(), "threadSettings.cwd") {
		t.Fatalf("old path error lost priority: %v", err)
	}
	badCollab := strings.Replace(string(raw), `"model":"model"`, `"model":null`, 1)
	if err := json.Unmarshal([]byte(badCollab), &value); err == nil || !strings.Contains(err.Error(), "collaborationMode.settings.model") {
		t.Fatalf("old collaboration error lost priority: %v", err)
	}
	var outer struct {
		Before   int              `json:"before"`
		Settings p.ThreadSettings `json:"threadSettings"`
		After    string           `json:"after"`
	}
	err := json.Unmarshal([]byte(`{"before":"bad","threadSettings":`+string(raw)+`,"after":"visited"}`), &outer)
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || outer.After != "visited" {
		t.Fatalf("enclosing first error/traversal changed: %v", err)
	}
	mock := NewMockTransport()
	calls, native := 0, 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, err error) {
		var mismatch *json.UnmarshalTypeError
		if errors.As(err, &mismatch) {
			native++
		}
	}))
	defer client.Close()
	client.CacheThreadState(p.Thread{ID: "collab-thread", DisabledPluginIDs: []string{"keep"}})
	client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { calls++ })
	remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { calls++ })
	defer remove()
	mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: disabledPluginSettingsPayload(t, `"disabledPluginIds":[42]`)})
	snapshot, ok := client.ThreadStateSnapshot("collab-thread")
	if calls != 0 || native == 0 || !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"keep"}) {
		t.Fatal("existing numeric-kind rejection or cache preservation changed")
	}
}
