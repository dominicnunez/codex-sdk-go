package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestStringArrayNullableSiblings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		newValue func() any
	}{
		{"plugin default prompt", `{"capabilities":[],"screenshotUrls":[],"screenshots":[],"defaultPrompt":[null]}`, func() any { return new(protocol.PluginInterface) }},
		{"question options", `{"title":"q","options":[null]}`, func() any { return new(protocol.AsyncUserInputQuestion) }},
		{"app categories", `{"id":"a","name":"A","appMetadata":{"categories":[null]}}`, func() any { return new(protocol.AppInfo) }},
		{"app subcategories", `{"id":"a","name":"A","appMetadata":{"subCategories":[null]}}`, func() any { return new(protocol.AppInfo) }},
		{"web queries", `{"type":"search","queries":[null]}`, func() any { return new(protocol.WebSearchActionWrapper) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, value := range []string{`null`, `[]`, `[""]`, `["valid"]`} {
				if err := json.Unmarshal([]byte(strings.ReplaceAll(tc.body, "[null]", value)), tc.newValue()); err != nil {
					t.Fatalf("valid nullable array %s: %v", value, err)
				}
			}
			if err := json.Unmarshal([]byte(tc.body), tc.newValue()); err == nil {
				t.Fatal("null item admitted in nullable string array")
			}
		})
	}
}

func TestStringArrayOverwrittenPathNulls(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		newValue func() any
	}{
		{"filesystem paths", `{"watchId":"w","changedPaths":[null],"changedPaths":["/tmp"]}`, func() any { return new(protocol.FsChangedNotification) }},
		{"plugin screenshots", `{"capabilities":[],"screenshotUrls":[],"screenshots":[null],"screenshots":[]}`, func() any { return new(protocol.PluginInterface) }},
		{"marketplace roots", `{"errors":[],"selectedMarketplaces":[],"upgradedRoots":[null],"upgradedRoots":[]}`, func() any { return new(protocol.MarketplaceUpgradeResponse) }},
		{"plugin whole array null", `{"capabilities":null,"capabilities":[],"screenshotUrls":[],"screenshots":[]}`, func() any { return new(protocol.PluginInterface) }},
		{"loaded folded null", `{"data":[],"DATA":null}`, func() any { return new(protocol.ThreadLoadedListResponse) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tc.body), tc.newValue()); err == nil {
				t.Fatal("invalid overwritten array occurrence admitted")
			}
		})
	}
}

func TestStringArrayVerificationCallback(t *testing.T) {
	for _, values := range []string{`[null]`, `["not-a-verification"]`, `[null],"verifications":["trustedAccessForCyber"]`, `["not-a-verification"],"verifications":[]`} {
		t.Run(values, func(t *testing.T) {
			mock := NewMockTransport()
			called, reported := 0, 0
			client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(string, error) { reported++ }))
			defer client.Close()
			client.OnModelVerification(func(protocol.ModelVerificationNotification) { called++ })
			mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "model/verification", Params: json.RawMessage(`{"threadId":"t","turnId":"u","verifications":` + values + `}`)})
			if called != 0 || reported != 1 {
				t.Fatalf("invalid verification published: calls=%d reports=%d", called, reported)
			}
			mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "model/verification", Params: json.RawMessage(`{"threadId":"t","turnId":"u","verifications":["trustedAccessForCyber"]}`)})
			if called != 1 || reported != 1 {
				t.Fatal("valid verification did not recover")
			}
		})
	}
}

func TestStringArrayVerificationSpellings(t *testing.T) {
	for _, value := range []string{`"trustedAccessForCyber"`, `"trusted\u0041ccessForCyber"`, `"\u0074rustedAccessForCyber"`} {
		var notification protocol.ModelVerificationNotification
		if err := json.Unmarshal([]byte(`{"threadId":"t","turnId":"u","verifications":[`+value+`]}`), &notification); err != nil {
			t.Fatalf("valid enum spelling %s rejected: %v", value, err)
		}
		if len(notification.Verifications) != 1 || notification.Verifications[0] != protocol.ModelVerificationTrustedAccessForCyber {
			t.Fatal("valid enum value changed")
		}
	}
}
