package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestInitializeEachSchemaOptInReachesWire(t *testing.T) {
	for _, declaration := range []string{
		`{"requestAttestation":true}`,
		`{"mcpServerOpenaiFormElicitation":true}`,
		`{"extensions":{"openai/form":{}}}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponse("initialize", codex.Response{Result: json.RawMessage(`{"codexHome":"/home","platformFamily":"unix","platformOs":"linux","userAgent":"test"}`)})
			var capabilities codex.InitializeCapabilities
			if err := json.Unmarshal([]byte(declaration), &capabilities); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			params := codex.InitializeParams{ClientInfo: codex.ClientInfo{Name: "test", Version: "1"}, Capabilities: &capabilities}
			if _, err := client.Initialize(context.Background(), params); err != nil {
				t.Fatal(err)
			}
			if mock.CallCount() != 1 {
				t.Fatalf("calls=%d", mock.CallCount())
			}
			var sent struct {
				Capabilities map[string]json.RawMessage `json:"capabilities"`
			}
			if err := json.Unmarshal(mock.GetSentRequest(0).Params, &sent); err != nil {
				t.Fatal(err)
			}
			var want map[string]json.RawMessage
			if err := json.Unmarshal([]byte(declaration), &want); err != nil {
				t.Fatal(err)
			}
			for field, expected := range want {
				if string(sent.Capabilities[field]) != string(expected) {
					t.Fatalf("opt-in %s lost: got=%s want=%s", field, sent.Capabilities[field], expected)
				}
			}
		})
	}
}

func capabilityParams(t *testing.T, declaration string) codex.InitializeParams {
	t.Helper()
	var capabilities codex.InitializeCapabilities
	if err := json.Unmarshal([]byte(declaration), &capabilities); err != nil {
		t.Fatal(err)
	}
	return codex.InitializeParams{ClientInfo: codex.ClientInfo{Name: "test", Version: "1"}, Capabilities: &capabilities}
}

func capabilityClient(t *testing.T) (*codex.Client, *MockTransport) {
	t.Helper()
	mock := NewMockTransport()
	if err := mock.SetResponseData("initialize", validInitializeResponseData("test")); err != nil {
		t.Fatal(err)
	}
	return codex.NewClient(mock), mock
}

func TestInitializeCapabilityIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		equal               bool
	}{
		{"attestation retained", `{"requestAttestation":true}`, `{"requestAttestation":true}`, true},
		{"attestation changed", `{"requestAttestation":true}`, `{}`, false},
		{"legacy form retained", `{"mcpServerOpenaiFormElicitation":true}`, `{"mcpServerOpenaiFormElicitation":true}`, true},
		{"legacy form changed", `{}`, `{"mcpServerOpenaiFormElicitation":true}`, false},
		{"empty extension declaration", `{"extensions":{"openai/form":{}}}`, `{"extensions":{"openai/form":{}}}`, true},
		{"named null present", `{"extensions":{"future":null}}`, `{}`, false},
		{"duplicate null then object", `{"extensions":null,"extensions":{"openai/form":{}}}`, `{"extensions":{"openai/form":{}}}`, true},
		{"duplicate object null object", `{"extensions":{"old":true},"extensions":null,"extensions":{"new":true}}`, `{"extensions":{"new":true}}`, true},
		{"empty root", `{"extensions":{}}`, `{"extensions":null}`, true},
		{"absent root", `{"extensions":null}`, `{}`, true},
		{"key order", `{"extensions":{"future":{"a":true,"b":[null,"x",2]}}}`, `{"extensions":{"future":{"b":[null,"x",2],"a":true}}}`, true},
		{"nested value", `{"extensions":{"future":{"a":true}}}`, `{"extensions":{"future":{"a":false}}}`, false},
		{"array order", `{"extensions":{"future":[1,2]}}`, `{"extensions":{"future":[2,1]}}`, false},
		{"extension name", `{"extensions":{"one":{}}}`, `{"extensions":{"two":{}}}`, false},
		{"large integer", `{"extensions":{"n":9007199254740992}}`, `{"extensions":{"n":9007199254740993}}`, false},
		{"number spelling", `{"extensions":{"n":1}}`, `{"extensions":{"n":1.0}}`, false},
		{"case alias after null", `{"extensions":null,"Extensions":{"openai/form":{}}}`, `{"extensions":{"openai/form":{}}}`, true},
		{"existing fields", `{"explicitGatewayOauth":true,"extensions":{"future":{}},"optOutNotificationMethods":["b","a","a"]}`, `{"explicitGatewayOauth":true,"extensions":{"future":{}},"optOutNotificationMethods":["a","b"]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, mock := capabilityClient(t)
			first, second := capabilityParams(t, tc.first), capabilityParams(t, tc.second)
			if _, err := client.Initialize(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			_, err := client.Initialize(context.Background(), second)
			var mismatch *codex.InitializeParamsMismatchError
			if tc.equal && err != nil {
				t.Fatal(err)
			}
			if !tc.equal && !errors.As(err, &mismatch) {
				t.Fatalf("want mismatch, got %v", err)
			}
			if mismatch != nil && !reflect.DeepEqual(mismatch.Requested, second) {
				t.Fatalf("requested snapshot=%#v, want %#v", mismatch.Requested, second)
			}
			if mock.CallCount() != 1 {
				t.Fatalf("calls=%d", mock.CallCount())
			}
		})
	}
}

type capabilityBlockingTransport struct {
	*MockTransport
	entered, release chan struct{}
	calls            atomic.Int32
	failFirst        bool
}

func (m *capabilityBlockingTransport) Send(ctx context.Context, req codex.Request) (codex.Response, error) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return codex.Response{}, ctx.Err()
		}
		if m.failFirst {
			return codex.Response{}, errors.New("first send failed")
		}
	}
	return m.MockTransport.Send(ctx, req)
}

type capabilityWaitingContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *capabilityWaitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

func TestInitializeExtensionsConcurrentAdmission(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		different, failFirst, cancelWaiter bool
	}{
		{name: "matching"}, {name: "different", different: true}, {name: "failed owner retry", failFirst: true}, {name: "cancelled waiter", cancelWaiter: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mock := capabilityClient(t)
			transport := &capabilityBlockingTransport{MockTransport: mock, entered: make(chan struct{}), release: make(chan struct{}), failFirst: tc.failFirst}
			client := codex.NewClient(transport)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			owner := capabilityParams(t, `{"extensions":{"future":{"n":1}},"requestAttestation":true}`)
			ownerDone := make(chan error, 1)
			go func() { _, err := client.Initialize(ctx, owner); ownerDone <- err }()
			select {
			case <-transport.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			waiter := capabilityParams(t, `{"extensions":{"future":{"n":1}},"requestAttestation":true}`)
			if tc.different {
				waiter.Capabilities.Extensions["future"].(map[string]interface{})["n"] = json.Number("2")
			}
			waitCtx, cancelWaiter := context.WithCancel(ctx)
			defer cancelWaiter()
			observed := &capabilityWaitingContext{Context: waitCtx, entered: make(chan struct{})}
			waiterDone := make(chan error, 1)
			go func() { _, err := client.Initialize(observed, waiter); waiterDone <- err }()
			select {
			case <-observed.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// Both callers have passed admission; subsequent mutation must affect neither.
			owner.Capabilities.Extensions["future"].(map[string]interface{})["n"] = 99
			waiter.Capabilities.Extensions["future"].(map[string]interface{})["n"] = 99
			if tc.cancelWaiter {
				cancelWaiter()
				if err := <-waiterDone; !errors.Is(err, context.Canceled) {
					t.Fatalf("waiter error=%v", err)
				}
			}
			close(transport.release)
			if err := <-ownerDone; (err != nil) != tc.failFirst {
				t.Fatalf("owner error=%v", err)
			}
			if !tc.cancelWaiter {
				err := <-waiterDone
				if tc.different {
					var mismatch *codex.InitializeParamsMismatchError
					if !errors.As(err, &mismatch) {
						t.Fatalf("want mismatch, got %v", err)
					}
					got := mismatch.Requested.Capabilities.Extensions["future"].(map[string]interface{})["n"]
					if got != json.Number("2") {
						t.Fatalf("requested admission changed: %v", got)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
			wantCalls := int32(1)
			if tc.failFirst {
				wantCalls = 2
			}
			if transport.calls.Load() != wantCalls {
				t.Fatalf("sends=%d want=%d", transport.calls.Load(), wantCalls)
			}
			want := capabilityParams(t, `{"extensions":{"future":{"n":1}},"requestAttestation":true}`)
			if _, err := client.Initialize(ctx, want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitializeExtensionsResponseFailureRetry(t *testing.T) {
	for _, response := range []codex.Response{
		{Error: &codex.Error{Code: -32600, Message: "denied"}},
		{Result: json.RawMessage(`{"userAgent":"test"}`)},
		{Result: json.RawMessage(`null`)},
	} {
		client, mock := capabilityClient(t)
		mock.SetResponse("initialize", response)
		params := capabilityParams(t, `{"extensions":{"openai/form":{}},"requestAttestation":true}`)
		if _, err := client.Initialize(context.Background(), params); err == nil {
			t.Fatal("bad response accepted")
		}
		if _, ok := client.InitializedParams(); ok {
			t.Fatal("failed response latched")
		}
		if err := mock.SetResponseData("initialize", validInitializeResponseData("test")); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Initialize(context.Background(), params); err != nil {
			t.Fatal(err)
		}
		if mock.CallCount() != 2 {
			t.Fatalf("calls=%d", mock.CallCount())
		}
	}
}

func TestInitializeExtensionsSnapshotOwnership(t *testing.T) {
	client, mock := capabilityClient(t)
	params := capabilityParams(t, `{"extensions":{"future":{"nested":[{"n":9007199254740993},null,true]}}}`)
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Extensions["future"].(map[string]interface{})["nested"].([]interface{})[0].(map[string]interface{})["n"] = 0
	snapshot, ok := client.InitializedParams()
	if !ok {
		t.Fatal("not initialized")
	}
	nested := snapshot.Capabilities.Extensions["future"].(map[string]interface{})["nested"].([]interface{})
	if got := nested[0].(map[string]interface{})["n"]; got != json.Number("9007199254740993") {
		t.Fatalf("number changed: %v", got)
	}
	nested[0].(map[string]interface{})["n"] = 1
	delete(snapshot.Capabilities.Extensions, "future")
	want := capabilityParams(t, `{"extensions":{"future":{"nested":[{"n":9007199254740993},null,true]}}}`)
	if _, err := client.Initialize(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	_, err := client.Initialize(context.Background(), params)
	var mismatch *codex.InitializeParamsMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	delete(mismatch.Existing.Capabilities.Extensions, "future")
	mismatch.Requested.Capabilities.Extensions["other"] = true
	if _, err := client.Initialize(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if _, present := params.Capabilities.Extensions["other"]; present {
		t.Fatal("requested diagnostic aliases caller")
	}
	if mock.CallCount() != 1 {
		t.Fatalf("calls=%d", mock.CallCount())
	}
}

func TestInitializeExtensionsEquivalentGoValues(t *testing.T) {
	client, _ := capabilityClient(t)
	params := capabilityParams(t, `{"extensions":{"future":{"n":2,"a":["x",null]}}}`)
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Extensions = map[string]interface{}{"future": json.RawMessage(`{"a":["x",null],"n":2}`)}
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Extensions = map[string]interface{}{"future": map[string]interface{}{"n": float64(2), "a": []interface{}{"x", nil}}}
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeExtensionsOwnSerializableStructs(t *testing.T) {
	type settings struct {
		private bool
		Values  map[string]interface{} `json:"values"`
	}
	client, _ := capabilityClient(t)
	values := map[string]interface{}{"n": 2}
	params := capabilityParams(t, `{}`)
	params.Capabilities.Extensions = map[string]interface{}{"future": settings{private: true, Values: values}}
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	values["n"] = 3
	want := capabilityParams(t, `{"extensions":{"future":{"values":{"n":2}}}}`)
	if _, err := client.Initialize(context.Background(), want); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeInvalidExtensionsDoNotLatch(t *testing.T) {
	for _, value := range []interface{}{make(chan int), json.RawMessage(`{`), json.Number("invalid")} {
		client, mock := capabilityClient(t)
		params := capabilityParams(t, `{"requestAttestation":true}`)
		params.Capabilities.Extensions = map[string]interface{}{"bad": value}
		if _, err := client.Initialize(context.Background(), params); err == nil {
			t.Fatal("invalid extension accepted")
		}
		if _, ok := client.InitializedParams(); ok {
			t.Fatal("failed request latched")
		}
		if mock.CallCount() != 0 {
			t.Fatal("invalid JSON reached transport")
		}
		params.Capabilities.Extensions = map[string]interface{}{"future": nil}
		if _, err := client.Initialize(context.Background(), params); err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkInitializeExtensions(b *testing.B) {
	for _, size := range []int{1, 1000} {
		for _, cached := range []bool{false, true} {
			b.Run(strconv.Itoa(size)+"/cached="+strconv.FormatBool(cached), func(b *testing.B) {
				params := codex.InitializeParams{ClientInfo: codex.ClientInfo{Name: "test", Version: "1"}, Capabilities: &codex.InitializeCapabilities{Extensions: make(map[string]interface{}, size)}}
				for i := 0; i < size; i++ {
					params.Capabilities.Extensions[strconv.Itoa(i)] = map[string]interface{}{"nested": []interface{}{json.Number("9007199254740993"), true, "setting", nil}}
				}
				newClient := func() *codex.Client {
					mock := NewMockTransport()
					if err := mock.SetResponseData("initialize", validInitializeResponseData("test")); err != nil {
						b.Fatal(err)
					}
					return codex.NewClient(mock)
				}
				client := newClient()
				if cached {
					if _, err := client.Initialize(context.Background(), params); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if !cached {
						client = newClient()
					}
					if _, err := client.Initialize(context.Background(), params); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
