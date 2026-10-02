package protocol_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestTypedNotificationOwnerIgnoresCasingMetadata(t *testing.T) {
	for _, tc := range []struct {
		method   string
		params   string
		register func(*protocol.Client, func(string))
	}{
		{"externalAgentConfig/import/progress", `{"importId":"a","ImportID":"b","itemTypeResults":[]}`, func(c *protocol.Client, h func(string)) {
			c.OnExternalAgentConfigImportProgress(func(n protocol.ExternalAgentConfigImportProgressNotification) { h(n.ImportID) })
		}},
		{"externalAgentConfig/import/completed", `{"importId":"a","ImportID":"b","itemTypeResults":[]}`, func(c *protocol.Client, h func(string)) {
			c.OnExternalAgentConfigImportCompleted(func(n protocol.ExternalAgentConfigImportCompletedNotification) { h(n.ImportID) })
		}},
		{"project/changed", `{"projectId":"a","ProjectID":"b","changeType":"updated"}`, func(c *protocol.Client, h func(string)) {
			c.OnProjectChanged(func(n protocol.ProjectChangedNotification) { h(n.ProjectID) })
		}},
		{"thread/deleted", `{"threadId":"a","ThreadID":"b"}`, func(c *protocol.Client, h func(string)) {
			c.OnThreadDeleted(func(n protocol.ThreadDeletedNotification) { h(n.ThreadID) })
		}},
		{"thread/reverted", `{"threadId":"a","ThreadID":"b"}`, func(c *protocol.Client, h func(string)) {
			c.OnThreadReverted(func(n protocol.ThreadRevertedNotification) { h(n.ThreadID) })
		}},
		{"thread/queue/changed", `{"threadId":"a","ThreadID":"b"}`, func(c *protocol.Client, h func(string)) {
			c.OnThreadQueueChanged(func(n protocol.ThreadQueueChangedNotification) { h(n.ThreadID) })
		}},
		{"thread/environment/connected", `{"threadId":"a","ThreadID":"b","environmentId":"env"}`, func(c *protocol.Client, h func(string)) {
			c.OnEnvironmentConnected(func(n protocol.EnvironmentConnectionNotification) { h(n.ThreadID) })
		}},
		{"thread/environment/disconnected", `{"threadId":"a","ThreadID":99,"environmentId":"env"}`, func(c *protocol.Client, h func(string)) {
			c.OnEnvironmentDisconnected(func(n protocol.EnvironmentConnectionNotification) { h(n.ThreadID) })
		}},
		{"autoApprovalReview/strictReviewRequired", `{"threadId":"a","ThreadID":"b","turnId":"turn","startedAtMs":1}`, func(c *protocol.Client, h func(string)) {
			c.OnStrictReviewRequired(func(n protocol.StrictReviewRequiredNotification) { h(n.ThreadID) })
		}},
		{"turn/moderationMetadata", `{"threadId":"a","ThreadID":"b","turnId":"turn","metadata":{}}`, func(c *protocol.Client, h func(string)) {
			c.OnTurnModerationMetadata(func(n protocol.TurnModerationMetadataNotification) { h(n.ThreadID) })
		}},
		{"turn/moderationMetadata", `{"threadId":"a","ThreadID":"b","turnId":"turn","metadata":null}`, func(c *protocol.Client, h func(string)) {
			c.OnTurnModerationMetadata(func(n protocol.TurnModerationMetadataNotification) { h(n.ThreadID) })
		}},
		{"model/safetyBuffering/updated", `{"threadId":"a","ThreadID":"b","turnId":"turn","model":"m","reasons":[],"useCases":[],"showBufferingUi":false}`, func(c *protocol.Client, h func(string)) {
			c.OnModelSafetyBufferingUpdated(func(n protocol.ModelSafetyBufferingUpdatedNotification) { h(n.ThreadID) })
		}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mock := NewMockTransport()
			client := protocol.NewClient(mock)
			seen := make(chan string, 1)
			tc.register(client, func(id string) { seen <- id })
			mock.InjectServerNotification(context.Background(), protocol.Notification{Method: tc.method, Params: json.RawMessage(tc.params)})
			select {
			case id := <-seen:
				if id != "a" {
					t.Fatalf("typed callback selected %q; want a", id)
				}
			case <-time.After(time.Second):
				t.Fatal("alias metadata prevented valid typed callback")
			}
		})
	}
}
