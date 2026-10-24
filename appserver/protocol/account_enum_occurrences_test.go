package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type accountEnumCarrier struct {
	name, method string
	wrap         func(string) string
	call         func(*p.Client) error
	listen       func(*p.Client, func(), bool)
}

type accountEnumOwner struct {
	name, field, prefix, schema, enum string
	nullable                          bool
	newRecord                         func() any
	carriers                          []accountEnumCarrier
}

func accountEnumOwners() []accountEnumOwner {
	identity := func(raw string) string { return raw }
	readAccount := accountEnumCarrier{"account read", "account/read", func(raw string) string {
		return `{"requiresOpenaiAuth":false,"account":` + raw + `}`
	}, func(c *p.Client) error {
		_, err := c.Account.Get(context.Background(), p.GetAccountParams{})
		return err
	}, nil}
	accountNotification := accountEnumCarrier{"account notification", "account/updated", identity, nil, func(c *p.Client, h func(), add bool) {
		callback := func(p.AccountUpdatedNotification) { h() }
		if add {
			c.AddAccountUpdatedListener(callback)
		} else {
			c.OnAccountUpdated(callback)
		}
	}}
	limits := []accountEnumCarrier{
		{"rate read", "account/rateLimits/read", func(raw string) string { return `{"rateLimits":` + raw + `}` }, func(c *p.Client) error {
			_, err := c.Account.GetRateLimits(context.Background())
			return err
		}, nil},
		{"rate read with params", "account/rateLimits/read", func(raw string) string { return `{"rateLimits":` + raw + `}` }, func(c *p.Client) error {
			_, err := c.Account.GetRateLimitsWithParams(context.Background(), &p.GetAccountRateLimitsParams{})
			return err
		}, nil},
		{"rate bucket", "account/rateLimits/read", func(raw string) string { return `{"rateLimits":{},"rateLimitsByLimitId":{"bucket":` + raw + `}}` }, func(c *p.Client) error {
			_, err := c.Account.GetRateLimits(context.Background())
			return err
		}, nil},
		{"rate bucket reset", "account/rateLimits/read", func(raw string) string {
			return `{"rateLimits":{},"rateLimitsByLimitId":{"bucket":` + raw + `},"rateLimitsByLimitId":null}`
		}, func(c *p.Client) error {
			_, err := c.Account.GetRateLimits(context.Background())
			return err
		}, nil},
		{"rate notification", "account/rateLimits/updated", func(raw string) string { return `{"rateLimits":` + raw + `}` }, nil, func(c *p.Client, h func(), add bool) {
			callback := func(p.AccountRateLimitsUpdatedNotification) { h() }
			if add {
				c.AddAccountRateLimitsUpdatedListener(callback)
			} else {
				c.OnAccountRateLimitsUpdated(callback)
			}
		}},
	}
	return []accountEnumOwner{
		{"account plan", "planType", `"type":"chatgpt","email":"test@example.com",`, "GetAccountResponse", "PlanType", false, func() any { return new(p.ChatgptAccount) }, []accountEnumCarrier{readAccount}},
		{"rate plan", "planType", "", "GetAccountRateLimitsResponse", "PlanType", true, func() any { return new(p.RateLimitSnapshot) }, limits},
		{"rate reason", "rateLimitReachedType", "", "GetAccountRateLimitsResponse", "RateLimitReachedType", true, func() any { return new(p.RateLimitSnapshot) }, limits},
		{"notification plan", "planType", "", "AccountUpdatedNotification", "PlanType", true, func() any { return new(p.AccountUpdatedNotification) }, []accountEnumCarrier{accountNotification}},
		{"notification auth", "authMode", "", "AccountUpdatedNotification", "AuthMode", true, func() any { return new(p.AccountUpdatedNotification) }, []accountEnumCarrier{accountNotification}},
	}
}

func TestAccountEnumRejectsHiddenInvalidOccurrence(t *testing.T) {
	for _, owner := range accountEnumOwners() {
		t.Run(owner.name, func(t *testing.T) {
			valid := "plus"
			switch owner.field {
			case "authMode":
				valid = "chatgpt"
			case "rateLimitReachedType":
				valid = "rate_limit_reached"
			}
			wire := fmt.Sprintf(`{%s%q:"future",%q:%q}`, owner.prefix, owner.field, owner.field, valid)
			if err := json.Unmarshal([]byte(wire), owner.newRecord()); err == nil {
				t.Error("record accepted hidden invalid enum")
			}
			for _, carrier := range owner.carriers {
				if !checkAccountEnumCarrier(t, carrier, wire, "") {
					t.Errorf("%s accepted hidden invalid enum", carrier.name)
				}
			}
		})
	}
}

// Returns true when the public carrier rejects the record. Notification tests
// exercise both registration paths independently and count forbidden delivery.
func checkAccountEnumCarrier(t *testing.T, carrier accountEnumCarrier, record, recovery string) bool {
	t.Helper()
	wire := json.RawMessage(carrier.wrap(record))
	if carrier.call != nil {
		mock := NewMockTransport()
		mock.SetResponse(carrier.method, p.Response{Result: wire})
		client := p.NewClient(mock)
		err := carrier.call(client)
		if err != nil && recovery != "" {
			mock.SetResponse(carrier.method, p.Response{Result: json.RawMessage(carrier.wrap(recovery))})
			if err := carrier.call(client); err != nil {
				t.Fatalf("%s failed recovery: %v", carrier.name, err)
			}
		}
		return err != nil
	}
	rejected := false
	for _, add := range []bool{false, true} {
		mock := NewMockTransport()
		calls, failures := 0, 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, _ error) { failures++ }))
		carrier.listen(client, func() { calls++ }, add)
		mock.InjectServerNotification(context.Background(), p.Notification{JSONRPC: "2.0", Method: carrier.method, Params: wire})
		currentRejected := calls == 0 && failures == 1
		if !add {
			rejected = currentRejected
		} else if rejected != currentRejected {
			t.Errorf("%s: On and Add disagreed on admission", carrier.name)
		}
		if !currentRejected {
			if calls != 1 || failures != 0 {
				t.Fatalf("%s add=%v: unexpected calls=%d errors=%d", carrier.name, add, calls, failures)
			}
		} else if recovery != "" {
			mock.InjectServerNotification(context.Background(), p.Notification{JSONRPC: "2.0", Method: carrier.method, Params: json.RawMessage(carrier.wrap(recovery))})
			if calls != 1 || failures != 1 {
				t.Fatalf("%s add=%v: recovery calls=%d errors=%d", carrier.name, add, calls, failures)
			}
		}
	}
	return rejected
}

func TestAccountEnumOccurrenceContract(t *testing.T) {
	for _, owner := range accountEnumOwners() {
		t.Run(owner.name, func(t *testing.T) {
			values := accountSchemaEnum(t, owner.schema, owner.enum)
			first, last := values[0], values[len(values)-1]
			key, _ := json.Marshal(owner.field)
			escaped := fmt.Sprintf(`"\u%04x%s"`, owner.field[0], owner.field[1:])
			final := fmt.Sprintf(`%s:%q`, key, last)
			validRecord := "{" + owner.prefix + final + "}"
			cases := []struct {
				name, fields, want string
				valid              bool
			}{
				{"valid duplicates", fmt.Sprintf(`%s:%q,%s`, key, first, final), last, true},
				{"escaped invalid", fmt.Sprintf(`%s:"future",%s`, escaped, final), "", false},
				{"case invalid", fmt.Sprintf(`%q:"future",%s`, strings.ToUpper(owner.field), final), "", false},
				{"invalid to null", fmt.Sprintf(`%s:"future",%s:null`, key, key), "", false},
				{"empty first", fmt.Sprintf(`%s:"",%s`, key, final), "", false},
				{"type first", fmt.Sprintf(`%s:1,%s`, key, final), "", false},
				{"long invalid", fmt.Sprintf(`%s:%q,%s`, key, strings.Repeat("x", 16000), final), "", false},
				{"missing", `"extra":true`, "", owner.nullable},
				{"null", fmt.Sprintf(`%s:null`, key), "", owner.nullable},
				{"valid to null", fmt.Sprintf(`%s,%s:null`, final, key), "", owner.nullable},
				{"null to valid", fmt.Sprintf(`%s:null,%s`, key, final), last, owner.nullable},
				{"nested extra", fmt.Sprintf(`"extra":{%s:"future"},%s`, key, final), last, true},
			}
			for _, value := range values {
				cases = append(cases, struct {
					name, fields, want string
					valid              bool
				}{value, fmt.Sprintf(`%s:%q`, key, value), value, true})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					record := owner.newRecord()
					if err := json.Unmarshal([]byte(validRecord), record); err != nil {
						t.Fatal(err)
					}
					before, _ := json.Marshal(record)
					wire := "{" + owner.prefix + tc.fields + "}"
					err := json.Unmarshal([]byte(wire), record)
					if (err == nil) != tc.valid {
						t.Fatalf("record error=%v, valid=%v", err, tc.valid)
					}
					after, _ := json.Marshal(record)
					if err != nil && string(before) != string(after) {
						t.Fatal("rejected metadata changed receiver")
					}
					if err != nil && len(err.Error()) > 4096 {
						t.Fatal("unbounded diagnostic")
					}
					if tc.valid {
						var fields map[string]json.RawMessage
						if err := json.Unmarshal(after, &fields); err != nil {
							t.Fatal(err)
						}
						want := ""
						if tc.want != "" {
							encoded, _ := json.Marshal(tc.want)
							want = string(encoded)
						}
						if string(fields[owner.field]) != want {
							t.Fatalf("member=%s, want %s", fields[owner.field], want)
						}
					}
					for _, carrier := range owner.carriers {
						if rejected := checkAccountEnumCarrier(t, carrier, wire, validRecord); rejected == tc.valid {
							t.Errorf("%s rejected=%v, valid=%v", carrier.name, rejected, tc.valid)
						}
					}
				})
			}
		})
	}
}

func accountSchemaEnum(t *testing.T, schema, name string) []string {
	t.Helper()
	wire, err := os.ReadFile("schema/json/v2/" + schema + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Definitions map[string]struct {
			Enum  []string `json:"enum"`
			OneOf []struct {
				Enum []string `json:"enum"`
			} `json:"oneOf"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(wire, &document); err != nil {
		t.Fatal(err)
	}
	definition := document.Definitions[name]
	values := definition.Enum
	for _, branch := range definition.OneOf {
		values = append(values, branch.Enum...)
	}
	if len(values) == 0 {
		t.Fatalf("schema enum %s missing", name)
	}
	return values
}

func TestAccountEnumPreservesExistingErrors(t *testing.T) {
	for _, tc := range []struct {
		name, wire, want string
		record           any
	}{
		{"account required", `{"type":"chatgpt","planType":"future","planType":"plus"}`, "email", new(p.ChatgptAccount)},
		{"notification final plan", `{"authMode":"future","authMode":"chatgpt","planType":"later"}`, `account.planType "later"`, new(p.AccountUpdatedNotification)},
		{"rate final reason", `{"planType":"future","planType":null,"rateLimitReachedType":"later"}`, `rateLimits.rateLimitReachedType "later"`, new(p.RateLimitSnapshot)},
		{"rate native type", `{"planType":"future","planType":null,"primary":{"usedPercent":"bad"}}`, "usedPercent", new(p.RateLimitSnapshot)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tc.wire), tc.record); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("existing error changed: %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAccountEnumsRemainPlainOutsideOwners(t *testing.T) {
	for _, scalar := range []any{new(p.PlanType), new(p.AuthMode), new(p.RateLimitReachedType)} {
		if err := json.Unmarshal([]byte(`"future"`), scalar); err != nil {
			t.Fatalf("scalar acquired record validation: %v", err)
		}
	}
	var envelope struct {
		p.PlanType
		Extra bool `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"PlanType":"future","extra":true}`), &envelope); err != nil || !envelope.Extra || envelope.PlanType != "future" {
		t.Fatalf("application envelope changed: %+v, %v", envelope, err)
	}
	for _, kind := range []string{"apiKey", "futureAccount"} {
		mock := NewMockTransport()
		mock.SetResponse("account/read", p.Response{Result: json.RawMessage(fmt.Sprintf(`{"requiresOpenaiAuth":false,"account":{"type":%q,"planType":"future"}}`, kind))})
		response, err := p.NewClient(mock).Account.Get(context.Background(), p.GetAccountParams{})
		if err != nil || response.Account == nil {
			t.Fatalf("unrelated account branch %s rejected: %v", kind, err)
		}
		if kind == "futureAccount" {
			unknown, ok := response.Account.Value.(*p.UnknownAccount)
			if !ok || !strings.Contains(string(unknown.Raw), `"planType":"future"`) {
				t.Fatal("unknown branch metadata lost")
			}
		}
	}
}
