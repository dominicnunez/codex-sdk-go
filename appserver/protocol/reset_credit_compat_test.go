package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

const validResetCredit = `{"grantedAt":0,"id":"","resetType":"unknown","status":"unknown"}`

func resetCreditRead(t *testing.T, body string) (p.GetAccountRateLimitsResponse, error) {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("account/rateLimits/read", p.Response{Result: json.RawMessage(body)})
	return client.Account.GetRateLimits(context.Background())
}

func resetCreditBody(summary string) string {
	return `{"rateLimits":{},"rateLimitResetCredits":` + summary + `}`
}

func TestResetCreditOccurrences(t *testing.T) {
	for _, summary := range []string{
		`{"availableCount":null,"availableCount":0}`,
		`{"availableCount":"x","availableCount":0}`,
		`{"availableCount":0.5}`,
		`{"availableCount":9223372036854775808}`,
		`{"availableCount":-9223372036854775809}`,
		`{"AVAILABLECOUNT":0}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"grantedAt":0`, `"grantedAt":"wrong"`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"id":""`, `"id":42`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"resetType":"unknown"`, `"resetType":42`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"status":"unknown"`, `"status":42`, 1) + `]}`,
		`{"availableCount":1,"credits":[{}],"credits":[` + validResetCredit + `]}`,
		`{"availableCount":1,"credits":[null],"credits":null}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"id":""`, `"ID":""`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"resetType":"unknown"`, `"resetType":"bogus","resetType":"unknown"`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"status":"unknown"`, `"status":null,"status":"unknown"`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"grantedAt":0`, `"grantedAt":null,"grantedAt":0`, 1) + `]}`,
		`{"availableCount":1,"credits":[` + strings.Replace(validResetCredit, `"id":""`, `"id":null,"id":""`, 1) + `]}`,
		`{"availableCount":1,"credits":[{"grantedAt":0,"id":"","resetType":"unknown","status":"unknown","description":42}]}`,
		`{"availableCount":1,"credits":[{"grantedAt":0,"id":"","resetType":"unknown","status":"unknown","title":[],"title":""}]}`,
		`{"availableCount":1,"credits":[{"grantedAt":0,"id":"","resetType":"unknown","status":"unknown","expiresAt":9223372036854775808}]}`,
	} {
		t.Run(summary, func(t *testing.T) {
			response, err := resetCreditRead(t, resetCreditBody(summary))
			if err == nil || !reflect.DeepEqual(response, p.GetAccountRateLimitsResponse{}) {
				t.Fatalf("invalid metadata occurrence published: %s", summary)
			}
		})
	}
	for _, field := range []string{`"rateLimitResetCredits"`, `"RATELIMITRESETCREDITS"`, `"rateLimitResetCre\u0064its"`} {
		t.Run(field, func(t *testing.T) {
			body := `{"rateLimits":{},` + field + `:{},"rateLimitResetCredits":{"availableCount":0}}`
			response, err := resetCreditRead(t, body)
			if err == nil || !reflect.DeepEqual(response, p.GetAccountRateLimitsResponse{}) {
				t.Fatal("earlier malformed summary hidden")
			}
		})
	}
}

func TestResetCreditRawAndExtras(t *testing.T) {
	for _, summary := range []string{
		`null`,
		` { "availableCount" : -9223372036854775808, "credits":null, "future":{"large":18446744073709551615} } `,
		`{"availableCount":9223372036854775807,"credits":[]}`,
		`{"availableCount":99,"credits":[{"grantedAt":-9223372036854775808,"id":"","resetType":"codexRateLimits","status":"available","title":"","description":null,"expiresAt":9223372036854775807,"future":{"precise":9007199254740993}}]}`,
		`{"AVAILABLECOUNT":"open","availableCount":0,"CREDITS":42,"credits":[{"ID":{},"id":"","GRANTEDAT":null,"grantedAt":-1,"RESETTYPE":"extra","resetType":"unknown","STATUS":42,"status":"unknown","TITLE":42,"title":"","EXPIRESAT":"extra","expiresAt":null}]}`,
		`{"availableCount":-1,"AVAILABLECOUNT":{},"credits":[{"grantedAt":-1,"GRANTEDAT":"extra","id":"","ID":42,"resetType":"unknown","RESETTYPE":42,"status":"unknown","STATUS":{},"title":null,"TITLE":{},"description":"","DESCRIPTION":42}]}`,
		`{"availableCo\u0075nt":1,"credits":[{"grantedAt":0,"id":"","resetT\u0079pe":"unknown","status":"redeemed"}]}`,
	} {
		t.Run(summary, func(t *testing.T) {
			body := resetCreditBody(summary)
			var expected struct {
				Raw json.RawMessage `json:"rateLimitResetCredits"`
			}
			if err := json.Unmarshal([]byte(body), &expected); err != nil {
				t.Fatal(err)
			}
			result, err := resetCreditRead(t, body)
			if err != nil || !bytes.Equal(result.RateLimitResetCredits, expected.Raw) {
				t.Fatalf("valid complete raw metadata changed: %v got=%s want=%s", err, result.RateLimitResetCredits, expected.Raw)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var again p.GetAccountRateLimitsResponse
			if err := json.Unmarshal(encoded, &again); err != nil {
				t.Fatal(err)
			}
			var expectedCompact, actualCompact bytes.Buffer
			if err := json.Compact(&expectedCompact, expected.Raw); err != nil {
				t.Fatal(err)
			}
			if err := json.Compact(&actualCompact, again.RateLimitResetCredits); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(expectedCompact.Bytes(), actualCompact.Bytes()) {
				t.Fatal("serialization lost complete metadata or numeric precision")
			}
		})
	}
	for _, body := range []string{`{"rateLimits":{}}`, `{"rateLimits":{},"rateLimitResetCredits":{"availableCount":-1},"RATELIMITRESETCREDITS":null}`} {
		result, err := resetCreditRead(t, body)
		if err != nil {
			t.Fatal(err)
		}
		var native struct {
			Raw json.RawMessage `json:"rateLimitResetCredits"`
		}
		if err := json.Unmarshal([]byte(body), &native); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(result.RateLimitResetCredits, native.Raw) {
			t.Fatal("outer native selected raw value changed")
		}
	}
}

func TestResetCreditReceiverAndPriorErrors(t *testing.T) {
	priorRaw := json.RawMessage(`{"availableCount":17}`)
	priorAccount := p.Ptr("prior")
	bucket := &p.RateLimitSnapshot{}
	before := p.GetAccountRateLimitsResponse{AccountID: priorAccount, RateLimitResetCredits: priorRaw, RateLimitsByLimitId: map[string]*p.RateLimitSnapshot{"prior": bucket}}
	actual := before
	if err := actual.UnmarshalJSON([]byte(resetCreditBody(`{}`))); err == nil || !reflect.DeepEqual(actual, before) || actual.AccountID != priorAccount || &actual.RateLimitResetCredits[0] != &priorRaw[0] || actual.RateLimitsByLimitId["prior"] != bucket {
		t.Fatal("failed metadata admission changed receiver references")
	}
	marker := &p.RateLimitSnapshot{}
	actual.RateLimitsByLimitId["identity"] = marker
	if before.RateLimitsByLimitId["identity"] != marker {
		t.Fatal("failed admission replaced map")
	}
	valid := `{"availableCount":0}`
	bad := `{` + strings.Repeat(" ", len(valid)-2) + `}`
	for _, template := range []string{
		`{"rateLimitResetCredits":@VALUE@}`,
		`{"rateLimits":null,"rateLimitResetCredits":@VALUE@}`,
		`{"rateLimits":{},"rateLimitResetCredits":@VALUE@,"ordinaryUsageAllowed":"wrong"}`,
		`{"rateLimits":{},"rateLimitResetCredits":@VALUE@,"rateLimitsByLimitId":{"bad":null}}`,
		`{"rateLimits":{},"rateLimitResetCredits":@VALUE@,"rateLimitsByLimitId":42}`,
	} {
		t.Run(template, func(t *testing.T) {
			decode := func(value string) error {
				var r p.GetAccountRateLimitsResponse
				return r.UnmarshalJSON([]byte(strings.Replace(template, "@VALUE@", value, 1)))
			}
			expected, actual := decode(valid), decode(bad)
			if expected == nil || actual == nil || !reflect.DeepEqual(expected, actual) {
				t.Fatalf("old error priority changed: %v vs %v", expected, actual)
			}
		})
	}
}

func TestResetCreditEnumsAndDiagnostics(t *testing.T) {
	for _, path := range []string{"schema/json/v2/GetAccountRateLimitsResponse.json", "schema/json/codex_app_server_protocol.v2.schemas.json"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Definitions map[string]struct {
				Enum []string `json:"enum"`
			}
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		types, statuses := schema.Definitions["RateLimitResetType"].Enum, schema.Definitions["RateLimitResetCreditStatus"].Enum
		if len(types) != 2 || len(statuses) != 4 {
			t.Fatal("unexpected current reset-credit enums")
		}
		for _, kind := range types {
			for _, status := range statuses {
				credit := strings.Replace(strings.Replace(validResetCredit, `"resetType":"unknown"`, `"resetType":"`+kind+`"`, 1), `"status":"unknown"`, `"status":"`+status+`"`, 1)
				if _, err := resetCreditRead(t, resetCreditBody(`{"availableCount":-1,"credits":[`+credit+`]}`)); err != nil {
					t.Fatal("current schema enum rejected", kind, status, err)
				}
			}
		}
	}
	for _, summary := range []string{`{"availableCount":` + strings.Repeat("9", 100000) + `}`, `{"availableCount":0,"credits":[{"grantedAt":0,"id":"","resetType":"unknown","status":` + quoteResetText(t, strings.Repeat("\x00", 100000)) + `}]}`} {
		_, err := resetCreditRead(t, resetCreditBody(summary))
		if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "rateLimitResetCredits") {
			t.Fatal("unbounded or missing metadata diagnostic")
		}
	}
	_, err := resetCreditRead(t, resetCreditBody(`{"availableCount":0.5}`))
	var native *json.UnmarshalTypeError
	if !errors.As(err, &native) || native.Type.Kind() != reflect.Int64 {
		t.Fatalf("native integer error cause lost: %v", err)
	}
}

func quoteResetText(t *testing.T, text string) string {
	t.Helper()
	raw, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func FuzzResetCreditMetadata(f *testing.F) {
	for _, seed := range []struct {
		count int64
		text  string
		mode  uint8
	}{{0, "", 0}, {-1, "id", 1}, {9223372036854775807, "", 2}, {-9223372036854775808, "é\x00", 3}, {0, "", 4}, {1, "bad", 5}, {2, string([]byte{0xff}), 9}} {
		f.Add(seed.count, seed.text, seed.mode)
	}
	f.Fuzz(func(t *testing.T, count int64, text string, mode uint8) {
		if len(text) > 1024 {
			text = text[:1024]
		}
		credit := map[string]any{"grantedAt": count, "id": text, "resetType": []string{"codexRateLimits", "unknown"}[int(mode)%2], "status": []string{"available", "redeeming", "redeemed", "unknown"}[int(mode)%4], "expiresAt": count, "title": text, "description": nil, "TITLE": 42, "future": map[string]any{"precise": json.Number("18446744073709551615")}}
		summary := any(map[string]any{"availableCount": count})
		switch mode % 6 {
		case 0:
			summary = nil
		case 1:
			summary = map[string]any{"availableCount": count, "credits": nil}
		case 2:
			summary = map[string]any{"availableCount": count, "credits": []any{}}
		case 3:
			summary = map[string]any{"availableCount": count, "credits": []any{credit}, "AVAILABLECOUNT": "open extra"}
		case 4:
			required := []string{"grantedAt", "id", "resetType", "status"}[int(mode>>3)%4]
			credit[required] = nil
			summary = map[string]any{"availableCount": count, "credits": []any{credit}}
		case 5:
			credit["status"] = "future/" + text
			summary = map[string]any{"availableCount": count, "credits": []any{credit}}
		}
		raw, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		result, err := resetCreditRead(t, resetCreditBody(string(raw)))
		if mode%6 >= 4 {
			if err == nil || !reflect.DeepEqual(result, p.GetAccountRateLimitsResponse{}) {
				t.Fatal("invalid generated metadata published")
			}
			return
		}
		if err != nil || !bytes.Equal(result.RateLimitResetCredits, raw) {
			t.Fatalf("valid generated complete metadata changed: %v got=%s want=%s", err, result.RateLimitResetCredits, raw)
		}
	})
}
