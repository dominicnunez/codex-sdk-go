package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestResetCreditMetadataAdmission(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, tc := range []struct {
			raw   string
			valid bool
		}{
			{`null`, true}, {`{"availableCount":0}`, true}, {`{"availableCount":0,"credits":null}`, true}, {`{"availableCount":0,"credits":[]}`, true},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"","resetType":"codexRateLimits","status":"available"}]}`, true},
			{`{"availableCount":-1,"credits":[{"grantedAt":-1,"id":"","resetType":"unknown","status":"unknown","title":"","description":null,"expiresAt":0}]}`, true},
			{`17`, false}, {`{"availableCount":"x"}`, false}, {`{"availableCount":1,"credits":{}}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"bogus","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"codexRateLimits","status":"bogus"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"codexRateLimits","status":"available","expiresAt":"x"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"codexRateLimits","status":"available","title":17}]}`, false},
			{`{}`, false}, {`{"availableCount":null}`, false}, {`{"availableCount":1,"credits":[null]}`, false},
			{`{"availableCount":1,"credits":[{}]}`, false},
			{`{"availableCount":1,"credits":[{"id":"id","resetType":"codexRateLimits","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":null,"id":"id","resetType":"codexRateLimits","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"resetType":"codexRateLimits","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":null,"resetType":"codexRateLimits","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":null,"status":"available"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"codexRateLimits"}]}`, false},
			{`{"availableCount":1,"credits":[{"grantedAt":0,"id":"id","resetType":"codexRateLimits","status":null}]}`, false},
		} {
			t.Run(fmt.Sprint(explicit)+"/"+tc.raw, func(t *testing.T) {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				mock.SetResponse("account/rateLimits/read", p.Response{Result: json.RawMessage(`{"rateLimits":{},"rateLimitResetCredits":` + tc.raw + `}`)})
				reader := func() (p.GetAccountRateLimitsResponse, error) {
					if explicit {
						return c.Account.GetRateLimitsWithParams(context.Background(), &p.GetAccountRateLimitsParams{ExcludeResetCreditDetails: p.Ptr(false)})
					}
					return c.Account.GetRateLimits(context.Background())
				}
				result, err := reader()
				if tc.valid {
					if err != nil || !bytes.Equal(result.RateLimitResetCredits, []byte(tc.raw)) {
						t.Fatalf("valid metadata rejected or changed: %v %s", err, result.RateLimitResetCredits)
					}
				} else {
					if err == nil || !reflect.DeepEqual(result, p.GetAccountRateLimitsResponse{}) {
						t.Fatal("malformed reset credit published")
					}
					recovered := `{"availableCount":0,"credits":[],"future":{"n":9007199254740993}}`
					mock.SetResponse("account/rateLimits/read", p.Response{Result: json.RawMessage(`{"rateLimits":{},"rateLimitResetCredits":` + recovered + `}`)})
					retry, err := reader()
					if err != nil || string(retry.RateLimitResetCredits) != recovered || len(mock.SentRequests) != 2 {
						t.Fatal("same-client recovery or complete metadata preservation failed")
					}
				}
			})
		}
	}
}
