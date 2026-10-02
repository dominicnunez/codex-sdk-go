package transport

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

func TestStdioAllKnownNotificationsSurviveBestEffortPressure(t *testing.T) {
	data, err := os.ReadFile("../protocol/schema/json/ServerNotification.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Events []struct {
			Properties struct {
				Method struct {
					Values []string `json:"enum"`
				} `json:"method"`
			} `json:"properties"`
		} `json:"oneOf"`
	}
	if err := json.Unmarshal(data, &schema); err != nil || len(schema.Events) == 0 {
		t.Fatalf("notification inventory err=%v count=%d", err, len(schema.Events))
	}
	r, w := io.Pipe()
	tr := NewStdioTransport(r, io.Discard)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
	entered, release := make(chan struct{}, 8), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	seen := make(chan string, len(schema.Events))
	barrier := make(chan struct{})
	tr.OnNotify(func(_ context.Context, n Notification) {
		if n.Method == "audit/unknown" {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			return
		}
		if string(n.Params) == `{"barrier":true}` {
			close(barrier)
			return
		}
		seen <- n.Method
	})
	for range 8 {
		writeAuditNotification(t, w, Notification{Method: "audit/unknown"})
		waitAuditSignal(t, entered)
	}
	for range 128 {
		writeAuditNotification(t, w, Notification{Method: "audit/unknown"})
	}
	missing := map[string]bool{}
	for _, event := range schema.Events {
		if len(event.Properties.Method.Values) != 1 {
			t.Fatal("unsupported method schema shape")
		}
		method := event.Properties.Method.Values[0]
		missing[method] = true
		writeAuditNotification(t, w, Notification{Method: method, Params: json.RawMessage(`{"threadId":"a","thread":{"id":"a"},"processHandle":"a","processId":"a","sessionId":"a","importId":"a","projectId":"a","watchId":"a","subscriptionId":"a"}`)})
	}
	writeAuditNotification(t, w, Notification{Method: "configWarning", Params: json.RawMessage(`{"barrier":true}`)})
	waitAuditSignal(t, barrier)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for len(missing) > 0 {
		select {
		case method := <-seen:
			delete(missing, method)
		case <-deadline.C:
			t.Fatalf("known notifications lost under best-effort pressure: %v", missing)
		}
	}
	if err := tr.ScanErr(); err != nil {
		t.Fatal(err)
	}
	unblock()
}
