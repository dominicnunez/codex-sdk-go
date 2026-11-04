package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadSectionNotificationLifecycle(t *testing.T) {
	mock := NewMockTransport()
	failures := 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, _ error) {
		if method != p.NotifyThreadSectionChanged {
			t.Errorf("error origin = %q", method)
		}
		failures++
	}))
	t.Cleanup(func() { _ = client.Close() })
	var primary, listeners []p.ThreadSectionChangedNotification
	client.OnThreadSectionChanged(func(n p.ThreadSectionChangedNotification) { primary = append(primary, n) })
	remove := client.AddThreadSectionChangedListener(func(n p.ThreadSectionChangedNotification) { listeners = append(listeners, n) })
	inject := func(body string) {
		mock.InjectServerNotification(context.Background(), p.Notification{Method: p.NotifyThreadSectionChanged, Params: json.RawMessage(body)})
	}
	for _, section := range []string{`null`, `""`, `"section"`} {
		inject(`{"threadId":"t","ThreadID":99,"sectionId":` + section + `,"SectionID":99}`)
	}
	if len(primary) != 3 || !reflect.DeepEqual(primary, listeners) {
		t.Fatalf("delivery primary=%+v listeners=%+v", primary, listeners)
	}
	if primary[0].ThreadID != "t" || primary[0].SectionID != nil || primary[1].SectionID == nil || *primary[1].SectionID != "" || primary[2].SectionID == nil || *primary[2].SectionID != "section" {
		t.Fatalf("nullable destination = %+v", primary)
	}
	for _, body := range []string{`{}`, `null`, `[]`, `{"threadId":"t"}`, `{"sectionId":null}`, `{"threadId":null,"sectionId":null}`, `{"threadId":99,"sectionId":null}`, `{"threadId":"t","sectionId":99}`, `{"threadId":99,"threadId":"t","sectionId":null}`, `{"threadId":"t","sectionId":99,"sectionId":null}`, `{"ThreadID":"t","sectionId":null}`, `{"threadId":"t","SectionID":null}`} {
		before := failures
		inject(body)
		if len(primary) != 3 || len(listeners) != 3 || failures != before+2 {
			t.Fatalf("invalid publication for %s: primary=%d listeners=%d failures=%d", body, len(primary), len(listeners), failures)
		}
	}
	remove()
	inject(`{"threadId":"","sectionId":null}`)
	client.OnThreadSectionChanged(nil)
	inject(`{"threadId":"t","sectionId":null}`)
	if len(primary) != 4 || primary[3].ThreadID != "" || len(listeners) != 3 {
		t.Fatalf("unsubscription primary=%+v listeners=%+v", primary, listeners)
	}
}

func TestThreadSectionNotificationRoundTrip(t *testing.T) {
	for _, body := range []string{`{"threadId":"","sectionId":null}`, `{"threadId":"t","sectionId":""}`, `{"threadId":"t","sectionId":"s"}`} {
		var n p.ThreadSectionChangedNotification
		if err := json.Unmarshal([]byte(body), &n); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(n)
		if err != nil || string(encoded) != body {
			t.Fatalf("roundtrip=%s err=%v want=%s", encoded, err, body)
		}
		before := n
		if err := json.Unmarshal([]byte(`{"threadId":"new"}`), &n); err == nil || !reflect.DeepEqual(n, before) {
			t.Fatalf("failed admission mutated receiver: %+v err=%v", n, err)
		}
	}
}
