package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type initializeHeldWriter struct {
	bytes.Buffer
	held    chan struct{}
	release chan struct{}
}

func (w *initializeHeldWriter) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	if len(data) == 1 && data[0] == '\n' {
		close(w.held)
		<-w.release
	}
	return n, err
}

func TestProcessAcceptedInitializeAtEOFCannotReplaceNotification(t *testing.T) {
	reader, peer := io.Pipe()
	writer := &initializeHeldWriter{held: make(chan struct{}), release: make(chan struct{})}
	tr := codextransport.NewStdioTransport(reader, writer)
	t.Cleanup(func() { close(writer.release); _ = tr.Close(); _ = peer.Close() })
	p := &Process{Client: protocol.NewClient(tr), transport: tr, initializeParams: defaultInitializeParams()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := p.Initialize(ctx); done <- err }()
	select {
	case <-writer.held:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var req protocol.Request
	if err := json.Unmarshal(bytes.TrimSpace(writer.Bytes()), &req); err != nil {
		t.Fatal(err)
	}
	if req.Method != "initialize" {
		t.Fatalf("method=%s", req.Method)
	}
	data, err := json.Marshal(map[string]interface{}{"id": req.ID.Value, "result": map[string]interface{}{"codexHome": "/home", "platformFamily": "unix", "platformOs": "linux", "userAgent": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	_ = peer.Close()
	select {
	case <-tr.ReaderStopped():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "initialized notification") {
			t.Fatalf("handshake skipped notification: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, ok := p.Client.InitializedParams(); !ok {
		t.Fatal("accepted initialize was not latched")
	}
	if p.initNotifyDone {
		t.Fatal("failed notification was latched")
	}
	if _, err := p.Initialize(ctx); err == nil || !strings.Contains(err.Error(), "initialized notification") {
		t.Fatalf("retry discarded handshake identity: %v", err)
	}
	if bytes.Count(writer.Bytes(), []byte("\n")) != 1 {
		t.Fatal("retry renegotiated accepted initialize")
	}
}
