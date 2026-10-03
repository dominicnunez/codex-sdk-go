package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type capabilityProcessTransport struct {
	requests []protocol.Request
	cancel   context.CancelFunc
}

func (m *capabilityProcessTransport) Send(_ context.Context, req protocol.Request) (protocol.Response, error) {
	m.requests = append(m.requests, req)
	if m.cancel != nil {
		m.cancel()
	}
	return protocol.Response{ID: req.ID, Result: json.RawMessage(`{"codexHome":"/home","platformFamily":"unix","platformOs":"linux","userAgent":"test"}`)}, nil
}

func (*capabilityProcessTransport) Notify(context.Context, protocol.Notification) error { return nil }
func (*capabilityProcessTransport) OnRequest(protocol.RequestHandler)                   {}
func (*capabilityProcessTransport) OnNotify(protocol.NotificationHandler)               {}
func (*capabilityProcessTransport) Close() error                                        { return nil }

func TestProcessCapabilitySnapshotAndNotificationRetry(t *testing.T) {
	for _, declaration := range []string{
		`{"requestAttestation":true}`, `{"mcpServerOpenaiFormElicitation":true}`, `{"extensions":{"openai/form":{}}}`,
		`{"experimentalApi":true,"explicitGatewayOauth":true,"requestAttestation":true,"mcpServerOpenaiFormElicitation":true,"extensions":{"future":{"nested":[9007199254740993,null]}} ,"optOutNotificationMethods":["b","a"]}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			var capabilities protocol.InitializeCapabilities
			if err := json.Unmarshal([]byte(declaration), &capabilities); err != nil {
				t.Fatal(err)
			}
			params := InitializeParams{ClientInfo: ClientInfo{Name: "test", Version: "1"}, Capabilities: &capabilities}
			owned, err := resolveProcessInitializeParams(&ProcessOptions{InitializeParams: &params})
			if err != nil {
				t.Fatal(err)
			}
			// Options may be changed after resolution, before the first handshake.
			capabilities.RequestAttestation = false
			capabilities.McpServerOpenaiFormElicitation = false
			if capabilities.Extensions != nil {
				capabilities.Extensions["changed"] = true
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			transport := &capabilityProcessTransport{cancel: cancel}
			reader, peer := io.Pipe()
			defer peer.Close()
			var output bytes.Buffer
			stdio := codextransport.NewStdioTransport(reader, &output)
			defer stdio.Close()
			process := &Process{Client: protocol.NewClient(transport), transport: stdio, initializeParams: owned}
			if _, err := process.Initialize(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("first notification cancellation: %v", err)
			}
			latched, ok := process.Client.InitializedParams()
			if !ok {
				t.Fatal("successful handshake must remain latched")
			}
			if latched.Capabilities.Extensions != nil {
				latched.Capabilities.Extensions["changed"] = false
			}
			if _, err := process.Initialize(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := process.Initialize(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(transport.requests) != 1 || bytes.Count(output.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("requests=%d notification output=%s", len(transport.requests), output.String())
			}
			var notification protocol.Notification
			if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &notification); err != nil || notification.Method != protocol.NotifyInitialized {
				t.Fatalf("unexpected notification: %s error=%v", output.String(), err)
			}
			var wire struct {
				Capabilities map[string]interface{} `json:"capabilities"`
			}
			if err := json.Unmarshal(transport.requests[0].Params, &wire); err != nil {
				t.Fatal(err)
			}
			var want map[string]interface{}
			if err := json.Unmarshal([]byte(declaration), &want); err != nil {
				t.Fatal(err)
			}
			// Existing opt-out identity is a set, so it is normalized on the wire.
			if _, exists := want["optOutNotificationMethods"]; exists {
				want["optOutNotificationMethods"] = []interface{}{"a", "b"}
			}
			for field, expected := range want {
				if !reflect.DeepEqual(wire.Capabilities[field], expected) {
					t.Fatalf("%s=%v, want=%v", field, wire.Capabilities[field], expected)
				}
			}
			if extensions, ok := wire.Capabilities["extensions"].(map[string]interface{}); ok && extensions["changed"] != nil {
				t.Fatal("wire settings alias options")
			}
		})
	}
}

func TestProcessOptionsOwnSerializableExtensionValues(t *testing.T) {
	type settings struct {
		private bool
		Values  map[string]interface{} `json:"values"`
	}
	values := map[string]interface{}{"n": 2}
	raw := json.RawMessage(`{"n":3}`)
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]interface{}{"future": settings{private: true, Values: values}, "raw": raw}}}
	owned, err := resolveProcessInitializeParams(&ProcessOptions{InitializeParams: &params})
	if err != nil {
		t.Fatal(err)
	}
	values["n"] = 99
	raw[5] = '9'
	if got := owned.Capabilities.Extensions["future"].(map[string]interface{})["values"].(map[string]interface{})["n"]; got != json.Number("2") {
		t.Fatalf("options retained reference: %v", got)
	}
	if got := owned.Capabilities.Extensions["raw"].(map[string]interface{})["n"]; got != json.Number("3") {
		t.Fatalf("raw options retained reference: %v", got)
	}
}

type failingProcessSettings struct {
	calls   *int
	failure error
	values  map[string]interface{}
}

func (s failingProcessSettings) MarshalJSON() ([]byte, error) {
	*s.calls++
	if *s.calls == 1 {
		return nil, s.failure
	}
	return json.Marshal(s.values)
}

func TestProcessRejectsExtensionSnapshotFailureBeforeLaunch(t *testing.T) {
	calls := 0
	failure := errors.New("snapshot failed")
	values := map[string]interface{}{"n": 2}
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]interface{}{"future": failingProcessSettings{calls: &calls, failure: failure, values: values}}}}
	opts := &ProcessOptions{BinaryPath: filepath.Join(t.TempDir(), "absent-codex"), InitializeParams: &params}
	if _, err := StartProcess(context.Background(), opts); !errors.Is(err, failure) {
		t.Fatalf("snapshot failure must precede launch: %v", err)
	}
	if calls != 1 {
		t.Fatalf("marshal calls=%d", calls)
	}
	owned, err := resolveProcessInitializeParams(opts)
	if err != nil {
		t.Fatal(err)
	}
	values["n"] = 99
	if got := owned.Capabilities.Extensions["future"].(map[string]interface{})["n"]; got != json.Number("2") {
		t.Fatalf("retry not owned: %v", got)
	}
}

type cancellingProcessSettings struct{ cancel context.CancelFunc }

func (s cancellingProcessSettings) MarshalJSON() ([]byte, error) {
	s.cancel()
	return []byte(`{}`), nil
}

func TestProcessCancellationDuringSnapshotPreventsLaunch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]interface{}{"future": cancellingProcessSettings{cancel: cancel}}}}
	opts := &ProcessOptions{BinaryPath: filepath.Join(t.TempDir(), "absent-codex"), InitializeParams: &params}
	if _, err := StartProcess(ctx, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preparation must precede launch: %v", err)
	}
}
