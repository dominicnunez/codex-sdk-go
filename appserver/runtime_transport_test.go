package appserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	c "github.com/dominicnunez/codex-sdk-go/appserver"
	tr "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type runtimeTerminalTransport struct {
	*tr.StdioTransport
	started chan struct{}
}

func (a *runtimeTerminalTransport) Send(ctx context.Context, r c.Request) (c.Response, error) {
	resp, err := a.StdioTransport.Send(ctx, r)
	if r.Method == "turn/start" && err == nil {
		close(a.started)
	}
	return resp, err
}

type runtimeTerminalResult struct {
	result *c.RunResult
	err    error
}

// Hide the optional lifetime capability to exercise Client.Close's fallback.
type legacyRuntimeTransport struct{ c.Transport }

func TestRuntimeTransportTermination(t *testing.T) {
	for _, route := range []string{"run", "stream", "collector", "conversation", "conversation-stream"} {
		for _, ending := range []string{"eof", "read-error", "close", "legacy-close", "completion-eof", "completion-close", "legacy-completion-close", "pressure-eof", "pressure-completion-eof"} {
			pressure := strings.HasPrefix(ending, "pressure-")
			if pressure && (route == "run" || route == "conversation") {
				continue
			}
			t.Run(route+"/"+ending, func(t *testing.T) {
				clientReader, serverWriter := io.Pipe()
				serverReader, clientWriter := io.Pipe()
				t.Cleanup(func() {
					_ = clientReader.Close()
					_ = serverWriter.Close()
					_ = serverReader.Close()
					_ = clientWriter.Close()
				})
				transport := &runtimeTerminalTransport{StdioTransport: tr.NewStdioTransport(clientReader, clientWriter), started: make(chan struct{})}
				var clientTransport c.Transport = transport
				if strings.HasPrefix(ending, "legacy-") {
					clientTransport = legacyRuntimeTransport{transport}
				}
				client := c.NewClient(clientTransport)
				if strings.HasSuffix(ending, "completion-close") {
					client.AddTurnCompletedListener(func(c.TurnCompletedNotification) { _ = client.Close() })
				}
				t.Cleanup(func() { _ = client.Close() })
				process := c.NewProcessFromClient(client)
				end := make(chan struct{})
				finishPeer := sync.OnceFunc(func() { close(end) })
				serverDone := make(chan error, 1)
				serverExited := make(chan struct{})
				go func() {
					defer close(serverExited)
					scanner := bufio.NewScanner(serverReader)
					for scanner.Scan() {
						var request struct {
							ID     json.RawMessage
							Method string
						}
						if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
							serverDone <- err
							return
						}
						if len(request.ID) == 0 {
							continue
						}
						var body any
						switch request.Method {
						case "initialize":
							body = validInitializeResponseData("synthetic")
						case "thread/start":
							body = validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))
						case "turn/start":
							body = map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}}
						default:
							serverDone <- fmt.Errorf("unexpected method %s", request.Method)
							return
						}
						raw, _ := json.Marshal(map[string]any{"id": request.ID, "result": body})
						if _, err := fmt.Fprintln(serverWriter, string(raw)); err != nil {
							serverDone <- err
							return
						}
						if request.Method == "turn/start" {
							<-end
							if pressure {
								for range 100 {
									if _, err := fmt.Fprintln(serverWriter, `{"method":"item/plan/delta","params":{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}}`); err != nil {
										serverDone <- err
										return
									}
								}
							}
							if strings.Contains(ending, "completion-") {
								_, err := fmt.Fprintln(serverWriter, `{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}}`)
								if err != nil {
									serverDone <- err
									return
								}
							}
							if ending == "read-error" {
								_ = serverWriter.CloseWithError(io.ErrUnexpectedEOF)
							} else {
								_ = serverWriter.Close()
							}
							serverDone <- nil
							return
						}
					}
					serverDone <- scanner.Err()
				}()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				finished := make(chan runtimeTerminalResult, 1)
				runtimeDone := make(chan struct{})
				collector := c.NewStreamCollector()
				reserved, consume := make(chan struct{}), make(chan struct{})
				startConsumer := sync.OnceFunc(func() { close(consume) })
				var deltas strings.Builder
				go func() {
					defer close(runtimeDone)
					var stream *c.Stream
					switch route {
					case "run":
						result, err := process.Run(ctx, c.RunOptions{Prompt: "local pipe fixture"})
						finished <- runtimeTerminalResult{result, err}
						return
					case "stream":
						stream = process.RunStreamed(ctx, c.RunOptions{Prompt: "local pipe fixture"})
					case "collector":
						stream = process.RunStreamedWithCollector(ctx, c.RunOptions{Prompt: "local pipe fixture"}, collector)
					default:
						conversation, err := process.StartConversation(ctx, c.ConversationOptions{})
						if err != nil {
							finished <- runtimeTerminalResult{nil, err}
							return
						}
						defer conversation.Close()
						if route == "conversation" {
							result, err := conversation.Turn(ctx, c.TurnOptions{Prompt: "local pipe fixture"})
							finished <- runtimeTerminalResult{result, err}
							return
						}
						stream = conversation.TurnStreamed(ctx, c.TurnOptions{Prompt: "local pipe fixture"})
					}
					var streamErr error
					seq := stream.Events()
					close(reserved)
					if pressure {
						<-consume
					}
					for event, err := range seq {
						if err != nil {
							streamErr = err
						}
						if delta, ok := event.(*c.PlanDelta); ok {
							deltas.WriteString(delta.Delta)
						}
					}
					finished <- runtimeTerminalResult{stream.Result(), streamErr}
				}()
				defer func() {
					cancel()
					startConsumer()
					finishPeer()
					_ = client.Close()
					_ = clientReader.Close()
					_ = serverWriter.Close()
					_ = serverReader.Close()
					_ = clientWriter.Close()
					waitPublicationSignal(t, runtimeDone, "runtime fixture cleanup blocked")
					waitPublicationSignal(t, serverExited, "peer fixture cleanup blocked")
				}()
				select {
				case <-transport.started:
				case err := <-serverDone:
					t.Fatalf("server stopped before start: %v", err)
				case <-time.After(3 * time.Second):
					t.Fatal("turn/start never succeeded")
				}
				if ending == "close" || ending == "legacy-close" {
					if err := client.Close(); err != nil {
						t.Fatal(err)
					}
				}
				if pressure {
					waitPublicationSignal(t, reserved, "stream iterator not reserved")
				}
				finishPeer()
				select {
				case <-transport.ReaderStopped():
				case <-time.After(3 * time.Second):
					t.Fatal("reader did not stop")
				}
				select {
				case err := <-serverDone:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("local fixture did not stop")
				}
				if pressure {
					select {
					case <-runtimeDone:
						t.Error("EOF bypassed backpressure and accepted event delivery")
					default:
					}
					startConsumer()
				}
				select {
				case got := <-finished:
					if strings.Contains(ending, "completion-") {
						if got.err != nil || got.result == nil {
							t.Fatalf("accepted completion lost: %+v", got)
						}
					} else if got.err == nil {
						t.Fatalf("terminal run succeeded without completion: %+v", got)
					} else {
						var terminal *c.TransportError
						if !errors.As(got.err, &terminal) {
							t.Errorf("terminal failure is not a transport error: %v", got.err)
						}
						if strings.HasSuffix(ending, "eof") && !errors.Is(got.err, io.EOF) || ending == "read-error" && !errors.Is(got.err, io.ErrUnexpectedEOF) {
							t.Errorf("terminal failure lost transport cause: %v", got.err)
						}
						if route == "collector" && len(collector.Summary().NormalizedErrors) != 1 {
							t.Error("collector did not receive exactly one terminal error")
						}
					}
					if pressure && deltas.String() != strings.Repeat("x", 100) {
						t.Errorf("EOF lost accepted deltas: received%d", deltas.Len())
					}
				case <-time.After(500 * time.Millisecond):
					t.Errorf("runtime still waiting after transport %s", ending)
					cancel()
					select {
					case <-finished:
					case <-time.After(3 * time.Second):
						t.Fatal("caller cancellation failed cleanup")
					}
				}
			})
		}
	}
}
