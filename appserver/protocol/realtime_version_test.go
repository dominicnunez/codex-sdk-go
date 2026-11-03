package protocol_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRealtimeVersionSchemaDelivery(t *testing.T) {
	for _, path := range []string{"schema/json/ClientRequest.json", "schema/json/ServerNotification.json", "schema/json/codex_app_server_protocol.v2.schemas.json", "schema/json/v2/ThreadRealtimeStartedNotification.json"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Definitions map[string]struct {
				Enum []string `json:"enum"`
			}
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		values := schema.Definitions["RealtimeConversationVersion"].Enum
		if len(values) != 3 {
			t.Fatal("unexpected version schema", values)
		}
		for _, version := range values {
			t.Run(path+"/"+version, func(t *testing.T) {
				encoded, _ := json.Marshal(version)
				var direct p.RealtimeConversationVersion
				if err := json.Unmarshal(encoded, &direct); err != nil || string(direct) != version {
					t.Error("current enum version rejected:", err)
				}
				mock := NewMockTransport()
				diagnostics := 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
					if method != "thread/realtime/started" || err == nil {
						t.Error("diagnostic origin changed")
					}
					diagnostics++
				}))
				t.Cleanup(func() { _ = client.Close() })
				onCalls, addCalls := 0, 0
				check := func(n p.ThreadRealtimeStartedNotification) {
					if n.ThreadID != "thread" || n.RealtimeSessionID == nil || *n.RealtimeSessionID != "session" || string(n.Version) != version {
						t.Error("notification metadata changed")
					}
				}
				client.OnThreadRealtimeStarted(func(n p.ThreadRealtimeStartedNotification) { onCalls++; check(n) })
				remove := client.AddThreadRealtimeStartedListener(func(n p.ThreadRealtimeStartedNotification) { addCalls++; check(n) })
				defer remove()
				body, _ := json.Marshal(map[string]any{"threadId": "thread", "realtimeSessionId": "session", "version": version})
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/realtime/started", Params: body})
				if onCalls != 1 || addCalls != 1 || diagnostics != 0 {
					t.Fatalf("current version dropped: on=%d add=%d diagnostics=%d", onCalls, addCalls, diagnostics)
				}
			})
		}
	}
}

func TestRealtimeV3RejectAndRecover(t *testing.T) {
	if p.RealtimeConversationVersionV3 != "v3" {
		t.Fatal("v3 constant changed")
	}
	for _, raw := range []string{
		`{"threadId":"thread","version":"bogus"}`,
		`{"threadId":"thread"}`,
		`{"threadId":"thread","version":null}`,
		`{"version":"v3"}`,
		`{"threadId":null,"version":"v3"}`,
		`{"threadId":"thread","version":null,"version":"v3"}`,
		`{"threadId":"thread","version":"bogus","version":"v3"}`,
		`{"threadId":"thread","version":42,"version":"v3"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			before := p.ThreadRealtimeStartedNotification{ThreadID: "before", RealtimeSessionID: p.Ptr("old"), Version: p.RealtimeConversationVersionV2}
			decoded := before
			if err := decoded.UnmarshalJSON([]byte(raw)); err == nil || !reflect.DeepEqual(decoded, before) || decoded.RealtimeSessionID != before.RealtimeSessionID {
				t.Fatal("invalid notification changed receiver")
			}
			mock := NewMockTransport()
			diagnostics := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
				if method != "thread/realtime/started" || err == nil {
					t.Error("diagnostic origin lost")
				}
				diagnostics++
			}))
			t.Cleanup(func() { _ = client.Close() })
			onCalls, addCalls := 0, 0
			check := func(n p.ThreadRealtimeStartedNotification) {
				if n.ThreadID != "thread" || n.Version != p.RealtimeConversationVersionV3 || n.RealtimeSessionID == nil || *n.RealtimeSessionID != "recovered" {
					t.Error("recovered notification changed")
				}
			}
			client.OnThreadRealtimeStarted(func(n p.ThreadRealtimeStartedNotification) { onCalls++; check(n) })
			remove := client.AddThreadRealtimeStartedListener(func(n p.ThreadRealtimeStartedNotification) { addCalls++; check(n) })
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/realtime/started", Params: json.RawMessage(raw)})
			if onCalls != 0 || addCalls != 0 || diagnostics != 2 {
				t.Fatalf("invalid notification delivered or diagnostics changed: %d %d %d", onCalls, addCalls, diagnostics)
			}
			valid := p.Notification{Method: "thread/realtime/started", Params: json.RawMessage(`{"threadId":"thread","version":"v3","realtimeSessionId":"recovered"}`)}
			mock.InjectServerNotification(context.Background(), valid)
			if onCalls != 1 || addCalls != 1 || diagnostics != 2 {
				t.Fatal("same-client valid recovery failed")
			}
			remove()
			mock.InjectServerNotification(context.Background(), valid)
			if onCalls != 2 || addCalls != 1 || diagnostics != 2 {
				t.Fatal("listener removal changed other registration")
			}
		})
	}
	version := p.RealtimeConversationVersionV2
	if err := json.Unmarshal([]byte(`"bogus"`), &version); err == nil || version != p.RealtimeConversationVersionV2 {
		t.Fatal("invalid enum changed receiver")
	}
}

func TestRealtimeV3OptionalSession(t *testing.T) {
	for _, tc := range []struct {
		field string
		want  *string
	}{{"", nil}, {`,"realtimeSessionId":null`, nil}, {`,"realtimeSessionId":""`, p.Ptr("")}} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		calls := 0
		check := func(n p.ThreadRealtimeStartedNotification) {
			calls++
			if n.Version != p.RealtimeConversationVersionV3 || !reflect.DeepEqual(n.RealtimeSessionID, tc.want) {
				t.Error("optional session metadata changed")
			}
		}
		client.OnThreadRealtimeStarted(check)
		remove := client.AddThreadRealtimeStartedListener(check)
		defer remove()
		mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/realtime/started", Params: json.RawMessage(`{"threadId":"thread","ver\u0073ion":"v3"` + tc.field + `}`)})
		if calls != 2 {
			t.Fatal("valid optional session rejected")
		}
	}
}
