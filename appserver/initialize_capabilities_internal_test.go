package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"sync"
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
				capabilities.Extensions["changed"] = json.RawMessage(`true`)
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
				latched.Capabilities.Extensions["changed"] = json.RawMessage(`false`)
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

func TestProcessOptionsOwnRawExtensionValues(t *testing.T) {
	raw := json.RawMessage(`{"n":9007199254740993}`)
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]json.RawMessage{"future": raw}}}
	owned, err := resolveProcessInitializeParams(&ProcessOptions{InitializeParams: &params})
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-2] = '4'
	params.Capabilities.Extensions["other"] = json.RawMessage(`true`)
	if string(owned.Capabilities.Extensions["future"]) != `{"n":9007199254740993}` {
		t.Fatalf("options retained buffer: %s", owned.Capabilities.Extensions["future"])
	}
	if _, exists := owned.Capabilities.Extensions["other"]; exists {
		t.Fatal("options retained map")
	}
}

func TestProcessRejectsExtensionSnapshotFailureBeforeLaunch(t *testing.T) {
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]json.RawMessage{"future": json.RawMessage(`{`)}}}
	opts := &ProcessOptions{BinaryPath: filepath.Join(t.TempDir(), "absent-codex"), InitializeParams: &params}
	_, err := StartProcess(context.Background(), opts)
	var marshalErr *json.MarshalerError
	if !errors.As(err, &marshalErr) {
		t.Fatalf("snapshot failure must precede launch: %v", err)
	}
	raw := json.RawMessage(`{"n":2}`)
	params.Capabilities.Extensions["future"] = raw
	owned, err := resolveProcessInitializeParams(opts)
	if err != nil {
		t.Fatal(err)
	}
	raw[5] = '9'
	if string(owned.Capabilities.Extensions["future"]) != `{"n":2}` {
		t.Fatalf("retry not owned: %s", owned.Capabilities.Extensions["future"])
	}
}

type cancelAfterContextCheck struct {
	context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cancelAfterContextCheck) Err() error {
	err := c.Context.Err()
	c.once.Do(c.cancel)
	return err
}

func TestProcessCancellationDuringPreparationPreventsLaunch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &cancelAfterContextCheck{Context: ctx, cancel: cancel}
	params := InitializeParams{Capabilities: &InitializeCapabilities{Extensions: map[string]json.RawMessage{"future": json.RawMessage(`{}`)}}}
	opts := &ProcessOptions{BinaryPath: filepath.Join(t.TempDir(), "absent-codex"), InitializeParams: &params}
	if _, err := StartProcess(observed, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preparation must precede launch: %v", err)
	}
}
