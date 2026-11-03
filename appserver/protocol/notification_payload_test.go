package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRawNotificationPayloadOwnership(t *testing.T) {
	for _, registration := range []string{"replacement", "additive"} {
		for _, payload := range []struct {
			name string
			raw  json.RawMessage
		}{
			{"nil", nil},
			{"empty", json.RawMessage{}},
			{"populated", json.RawMessage(`{"value":"original"}`)},
		} {
			t.Run(registration+"/"+payload.name, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				defer client.Close()
				original := bytes.Clone(payload.raw)
				var retained []json.RawMessage
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				handler := func(gotContext context.Context, n p.Notification) {
					if gotContext != ctx || n.Method != "extension/changed" || n.JSONRPC != "2.0" {
						t.Error("callback context or envelope changed")
					}
					if (n.Params == nil) != (payload.raw == nil) || !bytes.Equal(n.Params, original) {
						t.Error("callback payload or presence changed")
					}
					retained = append(retained, n.Params)
				}
				var remove func()
				if registration == "replacement" {
					client.OnNotification("extension/changed", handler)
					remove = func() { client.OnNotification("extension/changed", nil) }
				} else {
					remove = client.AddNotificationListener("extension/changed", handler)
				}
				defer remove()
				n := p.Notification{JSONRPC: "2.0", Method: "extension/changed", Params: payload.raw}
				mock.InjectServerNotification(ctx, n)
				mock.InjectServerNotification(ctx, n)
				if len(retained) != 2 {
					t.Fatal("callback not delivered twice")
				}
				if len(original) != 0 {
					retained[0][0] = '!'
					if !bytes.Equal(retained[1], original) || !bytes.Equal(n.Params, original) {
						t.Fatal("callback retains source or another delivery's buffer")
					}
					n.Params[0] = '?'
					if !bytes.Equal(retained[1], original) {
						t.Fatal("retained callback payload borrows transport input")
					}
				}
				remove()
				mock.InjectServerNotification(ctx, n)
				if len(retained) != 2 {
					t.Fatal("removed callback still admitted")
				}
			})
		}
	}
}

func TestNotificationPayloadFanoutOwnership(t *testing.T) {
	for _, mode := range []string{"baseline", "panic-only", "mutate-only", "mutate-panic", "copy-mutate-panic"} {
		t.Run(mode, func(t *testing.T) {
			mock := NewMockTransport()
			failure := errors.New("synthetic callback panic")
			reports := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
				reports++
				if method != "thread/name/updated" || !errors.Is(err, failure) {
					t.Error("diagnostic ownership changed")
				}
				panic("synthetic reporter panic")
			}))
			t.Cleanup(func() { _ = client.Close() })
			client.CacheThreadState(p.Thread{ID: "t", Name: p.Ptr("old")})
			stateName := ""
			removeState := client.AddThreadStateListener("t", func(thread p.Thread) {
				if thread.Name != nil {
					stateName = *thread.Name
				}
			}, nil)
			defer removeState()
			removeMutator := client.AddNotificationListener("thread/name/updated", func(_ context.Context, n p.Notification) {
				if mode == "mutate-only" || mode == "mutate-panic" || mode == "copy-mutate-panic" {
					raw := n.Params
					if mode == "copy-mutate-panic" {
						raw = append(json.RawMessage(nil), raw...)
					}
					offset := bytes.Index(raw, []byte(`"new"`))
					if offset < 0 {
						t.Fatal("mutation fixture missing")
					}
					copy(raw[offset+1:offset+4], []byte("bad"))
				}
				if mode == "panic-only" || mode == "mutate-panic" || mode == "copy-mutate-panic" {
					panic(failure)
				}
			})
			defer removeMutator()
			rawName := ""
			removeRaw := client.AddNotificationListener("thread/name/updated", func(_ context.Context, n p.Notification) {
				var fields map[string]string
				if err := json.Unmarshal(n.Params, &fields); err != nil {
					t.Fatal(err)
				}
				rawName = fields["threadName"]
			})
			defer removeRaw()
			typedName := ""
			removeTyped := client.AddThreadNameUpdatedListener(func(n p.ThreadNameUpdatedNotification) {
				if n.ThreadName != nil {
					typedName = *n.ThreadName
				}
			})
			defer removeTyped()
			primaryName := ""
			client.OnThreadNameUpdated(func(n p.ThreadNameUpdatedNotification) {
				if n.ThreadName != nil {
					primaryName = *n.ThreadName
				}
			})
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/name/updated", Params: json.RawMessage(`{"threadId":"t","threadName":"new"}`)})
			cached, ok := client.ThreadStateSnapshot("t")
			if !ok || cached.Name == nil || *cached.Name != "new" || stateName != "new" {
				t.Fatal("internal committed state not preserved")
			}
			if rawName != "new" || typedName != "new" || primaryName != "new" {
				t.Errorf("raw payload mutation reached later recipients: raw=%q typed=%q primary=%q", rawName, typedName, primaryName)
			}
			expectedReports := 0
			if mode == "panic-only" || mode == "mutate-panic" || mode == "copy-mutate-panic" {
				expectedReports = 1
			}
			if reports != expectedReports {
				t.Errorf("panic reports=%d want=%d", reports, expectedReports)
			}
		})
	}
}
