package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func TestStdioSectionNotificationRetention(t *testing.T) {
	for _, owner := range []string{`"t"`, `99`} {
		for _, closeBeforeDrain := range []bool{false, true} {
			t.Run(owner+" close="+fmt.Sprint(closeBeforeDrain), func(t *testing.T) {
				r, w := io.Pipe()
				tr := NewStdioTransport(r, io.Discard)
				t.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
				entered, release := make(chan struct{}, 2), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				t.Cleanup(unblock)
				// Admission precedes handler installation: valid owners wait in the
				// ordered queue; malformed owners cross the pending replay buffer.
				params := json.RawMessage(`{"threadId":` + owner + `,"sectionId":null}`)
				writeAuditNotification(t, w, Notification{Method: "thread/section/changed", Params: params})
				deadline := time.Now().Add(2 * time.Second)
				for {
					ready := retainedNotificationBytes(tr) > 0
					if owner == `99` {
						tr.mu.Lock()
						ready = len(tr.pendingNotifHandle) == 1
						tr.mu.Unlock()
					}
					if ready {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("notification did not reach pre-handler retention")
					}
					time.Sleep(time.Millisecond)
				}
				tr.OnNotify(func(context.Context, Notification) { entered <- struct{}{}; <-release })
				waitAuditSignal(t, entered)
				writeAuditNotification(t, w, Notification{Method: "thread/section/changed", Params: params})
				_ = w.Close()
				waitAuditSignal(t, tr.ReaderStopped())
				if got := retainedNotificationBytes(tr); got <= 0 {
					t.Fatal("in-flight notification lost reservation")
				}
				if closeBeforeDrain {
					_ = tr.Close()
				}
				unblock()
				waitNotificationBytes(t, tr, 0)
			})
		}
	}
}
