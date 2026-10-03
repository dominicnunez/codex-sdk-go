package transport_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

// The peer has consumed the delimiter, but Write has not returned to the SDK.
type responseHeldWriter struct {
	io.Writer
	held    chan struct{}
	release chan struct{}
}

func (w *responseHeldWriter) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	if len(data) == 1 && data[0] == '\n' && err == nil {
		close(w.held)
		<-w.release
	}
	return n, err
}

func TestStdioAcceptedResponseBeforeWriterReturnAndEOF(t *testing.T) {
	reader, peerOutput := io.Pipe()
	peerInput, writer := io.Pipe()
	held := &responseHeldWriter{Writer: writer, held: make(chan struct{}), release: make(chan struct{})}
	tr := codex.NewStdioTransport(reader, held)
	t.Cleanup(func() {
		close(held.release)
		_ = tr.Close()
		_ = peerOutput.Close()
		_ = peerInput.Close()
		_ = writer.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		resp codex.Response
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		resp, err := tr.Send(ctx, codex.Request{ID: codex.RequestID{Value: "accepted"}, Method: "test"})
		done <- outcome{resp, err}
	}()
	request := make(chan []byte, 1)
	go func() {
		line, _ := bufio.NewReader(peerInput).ReadBytes('\n')
		request <- line
	}()
	select {
	case line := <-request:
		var req codex.Request
		if err := json.Unmarshal(line, &req); err != nil || req.ID.Value != "accepted" {
			t.Fatalf("request=%s error=%v", line, err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-held.held:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := io.WriteString(peerOutput, "{\"id\":\"accepted\",\"result\":{\"ok\":true}}\n"); err != nil {
		t.Fatal(err)
	}
	_ = peerOutput.Close()
	select {
	case <-tr.ReaderStopped(): // response processing precedes read-loop termination
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case got := <-done:
		if got.err != nil || string(got.resp.Result) != `{"ok":true}` {
			t.Fatalf("accepted response lost: result=%s error=%v", got.resp.Result, got.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
