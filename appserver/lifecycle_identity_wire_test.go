package appserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

func lifecycleWireMetadata(prefix, suffix string, count int) json.RawMessage {
	var data strings.Builder
	data.WriteString(prefix)
	for i := range count {
		fmt.Fprintf(&data, `,"\u006detadata%d":0`, i)
	}
	data.WriteString(suffix)
	return json.RawMessage(data.String())
}

type denseLifecycleWriter struct {
	writer   io.Writer
	scenario string
	count    int
	prepared json.RawMessage
}

func (w denseLifecycleWriter) Write(p []byte) (int, error) {
	var notification codex.Notification
	if err := json.Unmarshal(p, &notification); err != nil {
		return 0, err
	}
	if notification.Method == "item/completed" {
		switch w.scenario {
		case "item-first":
			if w.prepared != nil {
				notification.Params = w.prepared
			} else {
				notification.Params = lifecycleWireMetadata(`{"threadId":"thread-1","ThreadID":"foreign","turnId":"turn-1","item":{"type":"agentMessage","id":99,"text":"bad"}`, `}`, w.count)
			}
		case "item-last":
			notification.Params = lifecycleWireMetadata(`{"threadId":"thread-1","ThreadID":"foreign","turnId":"turn-1"`, `,"item":{"type":"agentMessage","id":99,"text":"bad"}}`, w.count)
		case "delta":
			for _, thread := range []string{"foreign", "thread-1"} {
				params := lifecycleWireMetadata(fmt.Sprintf(`{"threadId":%q,"turnId":"turn-1"`, thread), `,"itemId":"item-1","delta":99}`, w.count)
				if err := writeStdioNotification(json.NewEncoder(w.writer), "item/agentMessage/delta", params); err != nil {
					return 0, err
				}
			}
		case "started":
			for _, thread := range []string{"foreign", "thread-1"} {
				params := lifecycleWireMetadata(fmt.Sprintf(`{"threadId":%q,"ThreadID":"foreign","turnId":"turn-1"`, thread), `,"startedAtMs":1,"item":{"type":"agentMessage","id":99,"text":"bad"}}`, w.count)
				if err := writeStdioNotification(json.NewEncoder(w.writer), "item/started", params); err != nil {
					return 0, err
				}
			}
		}
	}
	if notification.Method == "turn/completed" && w.scenario == "turn" {
		turn := lifecycleWireMetadata(`{"id":"turn-1","ID":"foreign","status":99`, `,"items":[]}`, w.count)
		notification.Params = append(json.RawMessage(`{"threadId":"thread-1","turn":`), turn...)
		notification.Params = append(notification.Params, '}')
	}
	if notification.Method != "" {
		if err := json.NewEncoder(w.writer).Encode(notification); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return w.writer.Write(p)
}

func TestRunDenseMalformedLifecycleOverStdio(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, scenario := range []string{"item-first", "item-last", "turn", "delta", "started"} {
			t.Run(fmt.Sprintf("streamed=%v/%s", streamed, scenario), func(t *testing.T) {
				if (scenario == "delta" || scenario == "started") && !streamed {
					t.Skip("delta lifecycle listeners belong to streamed runs")
				}
				clientReader, serverWriter := io.Pipe()
				serverReader, clientWriter := io.Pipe()
				t.Cleanup(func() {
					_ = clientReader.Close()
					_ = serverWriter.Close()
					_ = serverReader.Close()
					_ = clientWriter.Close()
				})
				serverDone := make(chan error, 1)
				go func() {
					serverDone <- serveLifecycleOverStdio(serverReader, denseLifecycleWriter{writer: serverWriter, scenario: scenario, count: 50000}, "thread-1", "turn-1", "item-1", "answer")
					_ = serverWriter.Close()
				}()
				tr := codextransport.NewStdioTransport(clientReader, clientWriter)
				t.Cleanup(func() { _ = tr.Close() })
				errorsReported := make(chan string, 10)
				client := codex.NewClient(tr, codex.WithRequestTimeout(10*time.Second), codex.WithHandlerErrorCallback(func(method string, _ error) { errorsReported <- method }))
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				process := codex.NewProcessFromClient(client)
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
				if scenario == "turn" {
					if runErr == nil || !strings.Contains(runErr.Error(), "turn error") {
						t.Fatalf("malformed nested completion error=%v", runErr)
					}
				} else {
					if runErr != nil || result == nil || len(result.Items) != 1 {
						t.Fatalf("result=%+v error=%v", result, runErr)
					}
					if scenario == "item-first" || scenario == "item-last" {
						if unknown, ok := result.Items[0].Value.(*codex.UnknownThreadItem); !ok || string(unknown.Raw) != `{"type":"agentMessage","id":99,"text":"bad"}` {
							t.Fatalf("item fallback=%#v", result.Items[0].Value)
						}
					}
				}
				select {
				case method := <-errorsReported:
					if method == "" {
						t.Fatal("missing error attribution")
					}
				default:
					t.Fatal("lifecycle decoding failure was not reported")
				}
				select {
				case err := <-serverDone:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal("server did not reach EOF")
				}
				if _, exists := client.ThreadStateSnapshot("foreign"); exists {
					t.Fatal("malformed foreign notification created unrelated thread state")
				}
			})
		}
	}
}

func BenchmarkRunDenseLifecycle(b *testing.B) {
	for _, streamed := range []bool{false, true} {
		for _, count := range []int{10, 500000} {
			b.Run(fmt.Sprintf("streamed=%v/fields=%d", streamed, count), func(b *testing.B) {
				var fixture strings.Builder
				fixture.WriteString(`{"threadId":"thread-1","turnId":"turn-1","item":{"type":"agentMessage","id":99,"text":"bad"}`)
				for i := range count {
					fmt.Fprintf(&fixture, `,"metadata%d":0`, i)
				}
				fixture.WriteByte('}')
				prepared := json.RawMessage(fixture.String())
				b.ReportAllocs()
				b.SetBytes(int64(len(prepared)))
				for b.Loop() {
					clientReader, serverWriter := io.Pipe()
					serverReader, clientWriter := io.Pipe()
					serverDone := make(chan error, 1)
					go func() {
						serverDone <- serveLifecycleOverStdio(serverReader, denseLifecycleWriter{writer: serverWriter, scenario: "item-first", prepared: prepared}, "thread-1", "turn-1", "item-1", "answer")
					}()
					tr := codextransport.NewStdioTransport(clientReader, clientWriter)
					client := codex.NewClient(tr)
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					process := codex.NewProcessFromClient(client)
					var result *codex.RunResult
					var err error
					if streamed {
						stream := process.RunStreamed(ctx, codex.RunOptions{Prompt: "hello"})
						for _, streamErr := range stream.Events() {
							if streamErr != nil {
								err = streamErr
							}
						}
						result = stream.Result()
					} else {
						result, err = process.Run(ctx, codex.RunOptions{Prompt: "hello"})
					}
					if err != nil || result == nil || len(result.Items) != 1 {
						b.Fatalf("result=%+v error=%v", result, err)
					}
					if err := <-serverDone; err != nil {
						b.Fatal(err)
					}
					cancel()
					_ = tr.Close()
					_ = serverWriter.Close()
					_ = clientReader.Close()
					_ = serverReader.Close()
					_ = clientWriter.Close()
				}
			})
		}
	}
}
