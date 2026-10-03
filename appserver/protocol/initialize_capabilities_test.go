package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

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

func TestInitializeEachSchemaOptInReachesWire(t *testing.T) {
	for _, declaration := range []string{`{"requestAttestation":true}`, `{"mcpServerOpenaiFormElicitation":true}`, `{"extensions":{"openai/form":{}}}`} {
		t.Run(declaration, func(t *testing.T) {
			client, mock := capabilityClient(t)
			params := capabilityParams(t, declaration)
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
			for name, value := range want {
				if !bytes.Equal(sent.Capabilities[name], value) {
					t.Fatalf("opt-in %s lost: got=%s want=%s", name, sent.Capabilities[name], value)
				}
			}
		})
	}
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
		{"empty named setting", `{"extensions":{"openai/form":{}}}`, `{"extensions":{"openai/form":{}}}`, true},
		{"named null present", `{"extensions":{"future":null}}`, `{}`, false},
		{"duplicate object merge", `{"extensions":{"a":1},"extensions":{"b":2}}`, `{"extensions":{"a":1,"b":2}}`, true},
		{"duplicate object overlap", `{"extensions":{"a":{"x":1},"b":true},"extensions":{"a":{"y":2}}}`, `{"extensions":{"a":{"y":2},"b":true}}`, true},
		{"duplicate null reset", `{"extensions":{"old":true},"extensions":null,"extensions":{"new":true}}`, `{"extensions":{"new":true}}`, true},
		{"duplicate alias merge", `{"Extensions":{"a":1},"extensions":{"b":2}}`, `{"extensions":{"a":1,"b":2}}`, true},
		{"duplicate null setting", `{"extensions":{"a":{},"b":true},"extensions":{"a":null}}`, `{"extensions":{"a":null,"b":true}}`, true},
		{"empty root", `{"extensions":{}}`, `{"extensions":null}`, true},
		{"absent root", `{"extensions":null}`, `{}`, true},
		{"key order", `{"extensions":{"future":{"a":true,"b":[null,"x",2]}}}`, `{"extensions":{"future":{"b":[null,"x",2],"a":true}}}`, true},
		{"nested value", `{"extensions":{"future":{"a":true}}}`, `{"extensions":{"future":{"a":false}}}`, false},
		{"array order", `{"extensions":{"future":[1,2]}}`, `{"extensions":{"future":[2,1]}}`, false},
		{"extension name", `{"extensions":{"one":{}}}`, `{"extensions":{"two":{}}}`, false},
		{"large integer", `{"extensions":{"n":9007199254740992}}`, `{"extensions":{"n":9007199254740993}}`, false},
		{"number spelling", `{"extensions":{"n":1}}`, `{"extensions":{"n":1.0}}`, false},
		{"large exponent", `{"extensions":{"n":1e400}}`, `{"extensions":{"n":1e400}}`, true},
		{"old fields", `{"explicitGatewayOauth":true,"extensions":{"future":{}},"optOutNotificationMethods":["b","a","a"]}`, `{"explicitGatewayOauth":true,"extensions":{"future":{}},"optOutNotificationMethods":["a","b"]}`, true},
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
			if mismatch != nil {
				compareInitializeJSON(t, "requested", mismatch.Requested, second)
			}
			if mock.CallCount() != 1 {
				t.Fatalf("calls=%d", mock.CallCount())
			}
		})
	}
}

func TestInitializeCapabilitiesReceiverCompatibility(t *testing.T) {
	declarations := []string{
		`{"extensions":{"b":2},"extensions":null,"extensions":{"c":3}}`,
		`{"experimentalApi":"bad","requestAttestation":true}`,
		`{"requestAttestation":true,"experimentalApi":"bad","explicitGatewayOauth":true}`,
		`{"extensions":[],"extensions":{"b":2},"requestAttestation":true}`,
		`{"optOutNotificationMethods":["before",1,"after"],"requestAttestation":true}`,
		`{"requestAttestation":true} {}`, `{"requestAttestation":true,`, `[]`, `null`, `{}`,
	}
	fragments := []string{
		`"experimentalApi":true`, `"experimentalApi":null`, `"experimentalApi":"bad"`,
		`"explicitGatewayOauth":true`, `"requestAttestation":true`, `"requestAttestation":null`,
		`"mcpServerOpenaiFormElicitation":true`, `"mcpServerOpenaiFormElicitation":false`,
		`"extensions":{"b":2}`, `"Extensions":{"b":{"nested":[1,true,null]}}`,
		`"extensions":null`, `"extensions":[]`, `"extensions":{"n":1e400}`,
		`"optOutNotificationMethods":["new"]`, `"optOutNotificationMethods":null`,
		`"optOutNotificationMethods":["before",1,"after"]`, `"unknown":{"ignored":true}`,
	}
	for _, first := range fragments {
		for _, second := range fragments {
			declarations = append(declarations, "{"+first+","+second+"}")
		}
	}
	for _, nilMap := range []bool{false, true} {
		for _, declaration := range declarations {
			t.Run(declaration+"/nil="+strconv.FormatBool(nilMap), func(t *testing.T) {
				retained := map[string]json.RawMessage{"a": json.RawMessage(`1`)}
				wantRetained := map[string]json.RawMessage{"a": json.RawMessage(`1`)}
				if nilMap {
					retained = nil
					wantRetained = nil
				}
				methods, wantMethods := []string{"old", "values", "here"}, []string{"old", "values", "here"}
				actual := codex.InitializeCapabilities{Extensions: retained, OptOutNotificationMethods: methods}
				expected := initializeCapsReference{Extensions: wantRetained, OptOutNotificationMethods: wantMethods}
				err := json.Unmarshal([]byte(declaration), &actual)
				wantErr := json.Unmarshal([]byte(declaration), &expected)
				compareInitializeJSON(t, "receiver", actual, expected)
				compareInitializeJSON(t, "retained map", retained, wantRetained)
				compareInitializeJSON(t, "retained slice", methods, wantMethods)
				compareInitializeDecodeError(t, err, wantErr)
			})
		}
	}
}

func TestInitializeExtensionsSnapshotOwnership(t *testing.T) {
	client, mock := capabilityClient(t)
	declaration := `{"extensions":{"future":{"nested":[{"n":9007199254740993},null,true]}}}`
	params := capabilityParams(t, declaration)
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	raw := params.Capabilities.Extensions["future"]
	copy(raw, bytes.Replace(raw, []byte("9007199254740993"), []byte("9007199254740994"), 1))
	snapshot, ok := client.InitializedParams()
	if !ok {
		t.Fatal("not initialized")
	}
	if !bytes.Contains(snapshot.Capabilities.Extensions["future"], []byte("9007199254740993")) {
		t.Fatal("snapshot changed with caller buffer")
	}
	output := snapshot.Capabilities.Extensions["future"]
	copy(output, bytes.Replace(output, []byte("9007199254740993"), []byte("9007199254740995"), 1))
	delete(snapshot.Capabilities.Extensions, "future")
	want := capabilityParams(t, declaration)
	if _, err := client.Initialize(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	_, err := client.Initialize(context.Background(), params)
	var mismatch *codex.InitializeParamsMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	existing := mismatch.Existing.Capabilities.Extensions["future"]
	copy(existing, bytes.Replace(existing, []byte("9007199254740993"), []byte("9007199254740996"), 1))
	requested := mismatch.Requested.Capabilities.Extensions["future"]
	copy(requested, bytes.Replace(requested, []byte("9007199254740994"), []byte("9007199254740997"), 1))
	mismatch.Requested.Capabilities.Extensions["other"] = json.RawMessage(`true`)
	if _, err := client.Initialize(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("9007199254740994")) {
		t.Fatal("requested diagnostic aliases caller buffer")
	}
	if _, present := params.Capabilities.Extensions["other"]; present {
		t.Fatal("requested diagnostic aliases caller map")
	}
	if mock.CallCount() != 1 {
		t.Fatalf("calls=%d", mock.CallCount())
	}
}

func TestInitializeExtensionsEquivalentJSON(t *testing.T) {
	client, _ := capabilityClient(t)
	params := capabilityParams(t, `{"extensions":{"future":{"n":2,"a":["x",null]}}}`)
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Extensions = map[string]json.RawMessage{"future": json.RawMessage(" { \"a\": [\"x\", null], \"n\": 2 } ")}
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		N int           `json:"n"`
		A []interface{} `json:"a"`
	}{N: 2, A: []interface{}{"x", nil}})
	if err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Extensions["future"] = encoded
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeInvalidExtensionsDoNotLatch(t *testing.T) {
	for _, value := range []json.RawMessage{json.RawMessage(`{`), json.RawMessage(``), json.RawMessage(`NaN`), json.RawMessage(`{} {}`)} {
		client, mock := capabilityClient(t)
		params := capabilityParams(t, `{"requestAttestation":true}`)
		params.Capabilities.Extensions = map[string]json.RawMessage{"bad": value}
		if _, err := client.Initialize(context.Background(), params); err == nil {
			t.Fatal("invalid extension accepted")
		}
		if _, ok := client.InitializedParams(); ok {
			t.Fatal("failed snapshot latched")
		}
		if mock.CallCount() != 0 {
			t.Fatal("invalid JSON reached transport")
		}
		params.Capabilities.Extensions = map[string]json.RawMessage{"future": nil}
		if _, err := client.Initialize(context.Background(), params); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitializeExtensionSnapshotDepthFailureDoesNotDispatch(t *testing.T) {
	client, mock := capabilityClient(t)
	params := capabilityParams(t, `{}`)
	nested := json.RawMessage(strings.Repeat("[", 10000) + "true" + strings.Repeat("]", 10000))
	params.Capabilities.Extensions = map[string]json.RawMessage{"future": nested}
	if _, err := json.Marshal(params.Capabilities.Extensions); err != nil {
		t.Fatalf("fixture must marshal: %v", err)
	}
	if _, err := client.Initialize(context.Background(), params); err == nil {
		t.Fatal("snapshot decode depth failure ignored")
	}
	if _, ok := client.InitializedParams(); ok {
		t.Fatal("snapshot depth failure latched")
	}
	if mock.CallCount() != 0 {
		t.Fatal("snapshot depth failure dispatched")
	}
	params.Capabilities.Extensions = map[string]json.RawMessage{"future": json.RawMessage(`true`)}
	if _, err := client.Initialize(context.Background(), params); err != nil {
		t.Fatal(err)
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
	}{{name: "matching"}, {name: "different", different: true}, {name: "failed owner retry", failFirst: true}, {name: "cancelled waiter", cancelWaiter: true}} {
		t.Run(tc.name, func(t *testing.T) {
			_, mock := capabilityClient(t)
			transport := &capabilityBlockingTransport{MockTransport: mock, entered: make(chan struct{}), release: make(chan struct{}), failFirst: tc.failFirst}
			client := codex.NewClient(transport)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			declaration := `{"extensions":{"future":{"n":1}},"requestAttestation":true}`
			owner := capabilityParams(t, declaration)
			ownerDone := make(chan error, 1)
			go func() { _, err := client.Initialize(ctx, owner); ownerDone <- err }()
			select {
			case <-transport.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			waiter := capabilityParams(t, declaration)
			if tc.different {
				waiter.Capabilities.Extensions["future"] = json.RawMessage(`{"n":2}`)
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
			copy(owner.Capabilities.Extensions["future"], []byte(`{"n":9}`))
			copy(waiter.Capabilities.Extensions["future"], []byte(`{"n":9}`))
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
					if string(mismatch.Requested.Capabilities.Extensions["future"]) != `{"n":2}` {
						t.Fatal("requested admission changed")
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
			if _, err := client.Initialize(ctx, capabilityParams(t, declaration)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitializeExtensionsResponseFailureRetry(t *testing.T) {
	for _, response := range []codex.Response{{Error: &codex.Error{Code: -32600, Message: "denied"}}, {Result: json.RawMessage(`{"userAgent":"test"}`)}, {Result: json.RawMessage(`null`)}} {
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

func BenchmarkInitializeExtensions(b *testing.B) {
	for _, size := range []int{1, 1000} {
		for _, cached := range []bool{false, true} {
			b.Run(strconv.Itoa(size)+"/cached="+strconv.FormatBool(cached), func(b *testing.B) {
				params := codex.InitializeParams{ClientInfo: codex.ClientInfo{Name: "test", Version: "1"}, Capabilities: &codex.InitializeCapabilities{Extensions: make(map[string]json.RawMessage, size)}}
				for i := 0; i < size; i++ {
					params.Capabilities.Extensions[strconv.Itoa(i)] = json.RawMessage(`{"nested":[9007199254740993,true,"setting",null]}`)
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
