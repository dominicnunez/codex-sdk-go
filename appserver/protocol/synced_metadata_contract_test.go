package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestSyncedSearchAndLoggingMetadata(t *testing.T) {
	for _, provider := range []string{`"standalone"`, `"search_mcp"`, `null`, `"invalid"`, `3`, `"invalid","client_search_provider":"standalone"`} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("config/read", p.Response{Result: json.RawMessage(`{"config":{"client_search_provider":` + provider + `},"origins":{}}`)})
		got, err := client.Config.Read(context.Background(), p.ConfigReadParams{})
		valid := provider == `"standalone"` || provider == `"search_mcp"` || provider == `null`
		if (err == nil) != valid {
			t.Errorf("provider %s: %v", provider, err)
			continue
		}
		if valid && provider != `null` {
			data, err := json.Marshal(got.Config)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["client_search_provider"]) != provider {
				t.Errorf("lost effective provider: %s", data)
			}
		}
	}
	for _, flag := range []string{"false", "true", "null", `"invalid"`} {
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"otel":{"log_sse_deltas":` + flag + `}}}`)})
		got, err := client.Config.ReadRequirements(context.Background())
		if (err == nil) != (flag != `"invalid"`) {
			t.Errorf("logging %s: %v", flag, err)
			continue
		}
		if err == nil && flag != "null" {
			data, err := json.Marshal(got.Requirements.Otel)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["log_sse_deltas"]) != flag {
				t.Errorf("lost logging flag: %s", data)
			}
		}
	}
}

func TestCurrentAccountFilterRoutes(t *testing.T) {
	for _, flag := range []string{"true", "false", "null"} {
		for _, complete := range []bool{false, true} {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("thread/list", p.Response{Result: json.RawMessage(`{"data":[]}`)})
			body := []byte(`{"currentAccountOnly":` + flag + `}`)
			var err error
			if complete {
				var params p.ThreadListParamsJSON
				if e := json.Unmarshal(body, &params); e != nil {
					t.Fatal(e)
				}
				_, err = client.Thread.ListJSON(context.Background(), params)
			} else {
				if flag == "null" {
					continue
				} // The legacy pointer representation omits null.
				var params p.ThreadListParams
				if e := json.Unmarshal(body, &params); e != nil {
					t.Fatal(e)
				}
				_, err = client.Thread.List(context.Background(), params)
			}
			if flag == "null" {
				if err == nil || mock.CallCount() != 0 {
					t.Error("nullable-only filter dispatched")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["currentAccountOnly"]) != flag {
				t.Errorf("filter disappeared: %s", mock.GetSentRequest(0).Params)
			}
		}
	}
}

func TestCompletionContentTokens(t *testing.T) {
	for _, token := range []string{"null", "0", "18446744073709551615", "-1", "18446744073709551616", "0.5", `"0"`, `-1,"contentTokenCount":0`} {
		mock := NewMockTransport()
		calls, reports := 0, 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, _ error) { reports++ }))
		t.Cleanup(func() { _ = client.Close() })
		check := func(n p.ItemCompletedNotification) {
			calls++
			data, err := json.Marshal(n)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if token != "null" && string(fields["contentTokenCount"]) != token {
				t.Errorf("completion token metadata lost: %s", data)
			}
		}
		client.OnItemCompleted(check)
		client.AddItemCompletedListener(check)
		mock.InjectServerNotification(context.Background(), p.Notification{Method: "item/completed", Params: json.RawMessage(`{"threadId":"t","turnId":"u","completedAtMs":0,"item":{"type":"agentMessage","id":"i","text":"done"},"contentTokenCount":` + token + `}`)})
		valid := token == "null" || token == "0" || token == "18446744073709551615"
		if valid && (calls != 2 || reports != 0) || !valid && (calls != 0 || reports != 2) {
			t.Errorf("tokens=%s calls=%d reports=%d", token, calls, reports)
		}
	}
}

func TestSearchProviderRestrictionOrder(t *testing.T) {
	for _, config := range []string{
		`{"client_search_provider":"invalid","sandbox_workspace_write":{"writable_roots":[null]}}`,
		`{"sandbox_workspace_write":{"writable_roots":[null]},"client_search_provider":"invalid"}`,
	} {
		var got p.Config
		err := json.Unmarshal([]byte(config), &got)
		if err == nil || !strings.Contains(err.Error(), "writable_roots") {
			t.Errorf("new enum restriction displaced existing field check: %v", err)
		}
	}
}

func TestSearchProviderNativeKindOrder(t *testing.T) {
	for _, body := range []string{
		`{"client_search_provider":3,"sandbox_workspace_write":{"writable_roots":[null]}}`,
		`{"sandbox_workspace_write":{"writable_roots":[null]},"client_search_provider":[]}`,
	} {
		var got p.Config
		err := json.Unmarshal([]byte(body), &got)
		var native *json.UnmarshalTypeError
		if !errors.As(err, &native) || native.Struct != "Config" || native.Field != "client_search_provider" || native.Type != reflect.TypeFor[string]() {
			t.Errorf("known scalar did not use native decode phase: %v", err)
		}
	}
}
