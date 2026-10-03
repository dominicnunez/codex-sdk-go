package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestTokenCacheWriteMetadata(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	defer client.Close()
	var got *codex.ThreadTokenUsageUpdatedNotification
	client.OnThreadTokenUsageUpdated(func(n codex.ThreadTokenUsageUpdatedNotification) { got = &n })
	const usage = `{"cachedInputTokens":1,"inputTokens":2,"outputTokens":3,"reasoningOutputTokens":4,"totalTokens":5,"cacheWriteInputTokens":6}`
	mock.InjectServerNotification(context.Background(), codex.Notification{
		Method: "thread/tokenUsage/updated",
		Params: json.RawMessage(`{"threadId":"thread-a","turnId":"turn-a","tokenUsage":{"last":` + usage + `,"total":` + usage + `}}`),
	})
	if got == nil || got.TokenUsage.Last.CacheWriteInputTokens != 6 || got.TokenUsage.Total.CacheWriteInputTokens != 6 {
		t.Fatalf("usage = %+v", got)
	}
	requireWireFields(t, got.TokenUsage.Last, usage)
	for _, extra := range []string{``, `,"cacheWriteInputTokens":0`, `,"cacheWriteInputTokens":-1`, `,"cacheWriteInputTokens":null`} {
		var value codex.TokenUsageBreakdown
		err := json.Unmarshal([]byte(`{"cachedInputTokens":1,"inputTokens":2,"outputTokens":3,"reasoningOutputTokens":4,"totalTokens":5`+extra+`}`), &value)
		if (err != nil) != (extra == `,"cacheWriteInputTokens":null`) {
			t.Fatalf("extra=%s, err=%v", extra, err)
		}
	}
}
