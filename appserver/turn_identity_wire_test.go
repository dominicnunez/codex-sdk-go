package appserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type turnIdentityWriter struct {
	writer                 io.Writer
	fields                 string
	activeID               string
	beforeStart, malformed bool
	pendingResponse        json.RawMessage
}

func (w *turnIdentityWriter) Write(p []byte) (int, error) {
	var message struct {
		Method string          `json:"method"`
		ID     codex.RequestID `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(p, &message); err != nil {
		return 0, err
	}
	enc := json.NewEncoder(w.writer)
	if message.Method == "" {
		var shape struct {
			Turn json.RawMessage `json:"turn"`
		}
		if json.Unmarshal(message.Result, &shape) == nil && shape.Turn != nil {
			response := codex.Response{JSONRPC: "2.0", ID: message.ID, Result: json.RawMessage(`{"turn":{` + w.fields + `,"status":"inProgress","items":[]}}`)}
			if w.beforeStart {
				var err error
				w.pendingResponse, err = json.Marshal(response)
				if err != nil {
					return 0, err
				}
			} else if err := enc.Encode(response); err != nil {
				return 0, err
			}
			return len(p), nil
		}
	}
	if message.Method == "turn/completed" {
		// A foreign completion whose canonical field names the active turn
		// must not terminate it after another semantic field fails.
		foreign := json.RawMessage(fmt.Sprintf(`{"threadId":"thread-1","turn":{"id":%q,"ID":"other","status":"completed","items":null}}`, w.activeID))
		if err := writeStdioNotification(enc, "turn/completed", foreign); err != nil {
			return 0, err
		}
		items := "[]"
		if w.malformed {
			items = "null"
		}
		current := json.RawMessage(`{"threadId":"thread-1","turn":{` + w.fields + `,"status":"completed","items":` + items + `}}`)
		if err := writeStdioNotification(enc, "turn/completed", current); err != nil {
			return 0, err
		}
		if w.pendingResponse != nil {
			if err := enc.Encode(w.pendingResponse); err != nil {
				return 0, err
			}
			w.pendingResponse = nil
		}
		return len(p), nil
	}
	return w.writer.Write(p)
}

func TestRunNestedTurnIdentityOverStdio(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, before := range []bool{false, true} {
			for _, malformed := range []bool{false, true} {
				for _, fields := range []string{`"id":"turn-1","ID":"current"`, `"ID":"other","id":"turn-1"`, `"id":"turn-1","ID":null`, `"id":"old","\u0069d":"turn-1"`} {
					t.Run(fmt.Sprintf("streamed=%v/before=%v/malformed=%v/%s", streamed, before, malformed, fields), func(t *testing.T) {
						var reference struct {
							ID string `json:"id"`
						}
						if err := json.Unmarshal([]byte(`{`+fields+`}`), &reference); err != nil {
							t.Fatal(err)
						}
						clientReader, serverWriter := io.Pipe()
						serverReader, clientWriter := io.Pipe()
						t.Cleanup(func() {
							_ = clientReader.Close()
							_ = serverReader.Close()
							_ = serverWriter.Close()
							_ = clientWriter.Close()
						})
						serverDone := make(chan error, 1)
						go func() {
							serverDone <- serveLifecycleOverStdio(serverReader, &turnIdentityWriter{writer: serverWriter, fields: fields, activeID: reference.ID, beforeStart: before, malformed: malformed}, "thread-1", reference.ID, "item-1", "answer")
						}()
						transport := codextransport.NewStdioTransport(clientReader, clientWriter)
						t.Cleanup(func() { _ = transport.Close() })
						client := codex.NewClient(transport)
						process := codex.NewProcessFromClient(client)
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						var result *codex.RunResult
						var runErr error
						if streamed {
							stream := process.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})
							for _, err := range stream.Events() {
								if err != nil {
									runErr = err
								}
							}
							result = stream.Result()
						} else {
							result, runErr = process.Run(ctx, codex.RunOptions{Prompt: "hello"})
						}
						if (runErr != nil) != malformed || (runErr != nil && ctx.Err() != nil) {
							t.Fatalf("completion error=%v context=%v", runErr, ctx.Err())
						}
						if !malformed && (result == nil || result.Turn.ID != reference.ID) {
							t.Fatalf("result=%+v want turn=%q", result, reference.ID)
						}
						snapshot, ok := client.ThreadStateSnapshot("thread-1")
						if !ok || len(snapshot.Turns) == 0 || snapshot.Turns[len(snapshot.Turns)-1].ID != reference.ID {
							t.Fatalf("cached turn identity=%+v want=%q", snapshot, reference.ID)
						}
						select {
						case err := <-serverDone:
							if err != nil {
								t.Fatal(err)
							}
						case <-ctx.Done():
							t.Fatal("server did not complete lifecycle")
						}
					})
				}
			}
		}
	}
}
