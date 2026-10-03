package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Measure complete public service calls, including nested records and the
// response envelope, rather than the validation helper alone.
func BenchmarkSyncedAdmissionServices(b *testing.B) {
	for _, count := range []int{1, 128} {
		app := `{"id":"app","name":"App","pluginDisplayNames":["Plugin"],"toolSummaries":[{"name":"tool","description":"Tool","isEnabled":true,"isReadOnly":false}]}`
		apps := "[" + strings.TrimSuffix(strings.Repeat(app+",", count), ",") + "]"
		bucket := `{"startDate":"2026-10-03","tokens":1}`
		group := `{"estimatedUsageCreditsMicros":1,"inputTokens":1,"outputTokens":1,"model":"model"}`
		buckets := "[" + strings.TrimSuffix(strings.Repeat(bucket+",", count), ",") + "]"
		groups := "[" + strings.TrimSuffix(strings.Repeat(group+",", count), ",") + "]"
		for _, tc := range []struct {
			name, method, payload string
			call                  func(*codex.Client) error
		}{
			{"apps", "app/read", `{"apps":` + apps + `,"missingAppIds":[]}`, func(c *codex.Client) error {
				_, err := c.Apps.Read(context.Background(), codex.AppsReadParams{})
				return err
			}},
			{"usage", "account/usage/read", `{"summary":{},"dailyUsageBuckets":` + buckets + `,"threadUsage":{"threadId":"thread","estimatedUsageCreditsMicros":1,"groups":` + groups + `}}`, func(c *codex.Client) error { _, err := c.Account.GetTokenUsage(context.Background(), nil); return err }},
		} {
			b.Run(fmt.Sprintf("%s/%d", tc.name, count), func(b *testing.B) {
				mock := NewMockTransport()
				mock.SetResponse(tc.method, codex.Response{Result: json.RawMessage(tc.payload)})
				client := codex.NewClient(mock)
				defer client.Close()
				if err := tc.call(client); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.payload)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// The mock's request log is instrumentation, not retained SDK state.
					mock.SentRequests = mock.SentRequests[:0]
					if err := tc.call(client); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
