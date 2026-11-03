package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMarketplaceUpgradeErrorAdmission(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`[]`, true}, {`[{"marketplaceName":"","message":""}]`, true},
		{`[{}]`, false}, {`[null]`, false}, {`[{"message":"x"}]`, false}, {`[{"marketplaceName":"x"}]`, false},
		{`[{"marketplaceName":null,"message":"x"}]`, false}, {`[{"marketplaceName":"x","message":null}]`, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			mock.SetResponse("marketplace/upgrade", p.Response{Result: json.RawMessage(`{"errors":` + tc.raw + `,"selectedMarketplaces":[],"upgradedRoots":[]}`)})
			result, err := c.Marketplace.Upgrade(context.Background(), p.MarketplaceUpgradeParams{})
			if tc.valid {
				if err != nil {
					t.Fatal("valid control:", err)
				}
			} else {
				if err == nil || !reflect.DeepEqual(result, p.MarketplaceUpgradeResponse{}) {
					t.Fatal("malformed error record published")
				}
				mock.SetResponse("marketplace/upgrade", p.Response{Result: json.RawMessage(`{"errors":[{"marketplaceName":"recovered","message":""}],"selectedMarketplaces":[],"upgradedRoots":[]}`)})
				retry, err := c.Marketplace.Upgrade(context.Background(), p.MarketplaceUpgradeParams{})
				if err != nil || len(retry.Errors) != 1 || retry.Errors[0].MarketplaceName != "recovered" || len(mock.SentRequests) != 2 {
					t.Fatal("same-client recovery failed")
				}
			}
		})
	}
}
