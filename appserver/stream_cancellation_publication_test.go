package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/appserver"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type pendingPublicationTransport struct {
	*MockTransport
	entered     chan struct{}
	release     chan struct{}
	injected    chan struct{}
	releaseOnce sync.Once
	count       int
}

func (m *pendingPublicationTransport) openGate() { m.releaseOnce.Do(func() { close(m.release) }) }
func (m *pendingPublicationTransport) Send(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	response, err := m.MockTransport.Send(ctx, req)
	if err != nil || req.Method != "turn/start" {
		return response, err
	}
	close(m.entered)
	select {
	case <-m.release:
	case <-ctx.Done():
	}
	for range m.count {
		m.InjectServerNotification(ctx, sdk.Notification{Method: "item/plan/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}`)})
	}
	close(m.injected)
	return response, nil
}
func startPublicationStream(t *testing.T, ctx context.Context, route string, count int) (*sdk.Stream, *pendingPublicationTransport, *sdk.StreamCollector) {
	t.Helper()
	base := NewMockTransport()
	for method, value := range map[string]any{"initialize": validInitializeResponseData("publication-test"), "thread/start": validProcessThreadStartResponse(validProcessThreadPayload("thread-1")), "turn/start": map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}}} {
		if err := base.SetResponseData(method, value); err != nil {
			t.Fatal(err)
		}
	}
	transport := &pendingPublicationTransport{MockTransport: base, entered: make(chan struct{}), release: make(chan struct{}), injected: make(chan struct{}), count: count}
	client := sdk.NewClient(transport)
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(transport.openGate)
	process := sdk.NewProcessFromClient(client)
	collector := sdk.NewStreamCollector()
	switch route {
	case "stream":
		return process.RunStreamed(ctx, sdk.RunOptions{Prompt: "publication"}), transport, collector
	case "collector":
		return process.RunStreamedWithCollector(ctx, sdk.RunOptions{Prompt: "publication"}, collector), transport, collector
	default:
		conversation, err := process.StartConversation(ctx, sdk.ConversationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conversation.Close() })
		return conversation.TurnStreamed(ctx, sdk.TurnOptions{Prompt: "publication"}), transport, collector
	}
}
func waitPublicationSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal(message)
	}
}

func TestStreamCancellationDuringPendingReplay(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		for _, paused := range []bool{false, true} {
			mode := "reserved"
			count := 65
			if paused {
				mode = "paused"
				count = 66
			}
			t.Run(route+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stream, transport, collector := startPublicationStream(t, ctx, route, count)
				waitPublicationSignal(t, transport.entered, "turn start not entered")
				seq := stream.Events()
				releaseConsumer := make(chan struct{})
				resume := sync.OnceFunc(func() { close(releaseConsumer) })
				defer resume()
				consumerDone, first := make(chan struct{}), make(chan struct{})
				if paused {
					go func() {
						defer close(consumerDone)
						seq(func(sdk.Event, error) bool { close(first); <-releaseConsumer; return false })
					}()
				}
				transport.openGate()
				waitPublicationSignal(t, transport.injected, "startup notifications not admitted")
				if paused {
					waitPublicationSignal(t, first, "consumer did not receive first event")
				}
				// There are more accepted startup events than available queue slots, even
				// if cancellation precedes the last send. Replay cannot finish while the
				// reserved/paused consumer stays still; no scheduling delay arranges this.
				cancel()
				done := make(chan *sdk.RunResult, 1)
				go func() { done <- stream.Result() }()
				finished := false
				select {
				case result := <-done:
					finished = true
					if result != nil {
						t.Error("canceled replay returned a result")
					}
				case <-time.After(time.Second):
					t.Error("cancellation did not release startup publication")
				}
				if finished && route == "collector" {
					before := collector.Summary()
					transport.InjectServerNotification(context.Background(), sdk.Notification{Method: "item/plan/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"late"}`)})
					after := collector.Summary()
					if before.LatestPlanText == nil || after.LatestPlanText == nil || *before.LatestPlanText != *after.LatestPlanText {
						t.Error("completed cancellation retained a lifecycle listener")
					}
				}
				if paused {
					resume()
					waitPublicationSignal(t, consumerDone, "paused consumer cleanup blocked")
				} else {
					var terminal error
					seq(func(_ sdk.Event, err error) bool {
						if !finished {
							return false
						}
						if err != nil {
							terminal = err
						}
						return true
					})
					if finished && !errors.Is(terminal, context.Canceled) {
						t.Errorf("terminal error = %v, want cancellation", terminal)
					}
				}
				if !finished {
					select {
					case <-done:
					case <-time.After(3 * time.Second):
						t.Fatal("lifecycle cleanup remained blocked")
					}
				}
			})
		}
	}
}

func TestStreamCancellationWhilePublishingCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, transport, collector := startPublicationStream(t, ctx, "collector", 0)
	seq := stream.Events()
	transport.openGate()
	waitPublicationSignal(t, transport.injected, "turn start not returned")
	waitForRunStreamedReady(t, transport.MockTransport)
	for range 64 {
		transport.InjectServerNotification(ctx, sdk.Notification{Method: "item/plan/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}`)})
	}
	transport.InjectServerNotification(ctx, sdk.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","items":[],"error":{"message":"remote failure"}}}`)})
	deadline := time.Now().Add(3 * time.Second)
	for {
		summary := collector.Summary()
		if len(summary.NormalizedErrors) == 1 && summary.NormalizedErrors[0].Kind == "turn_error" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("completion was not selected before publication")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	done := make(chan *sdk.RunResult, 1)
	go func() { done <- stream.Result() }()
	finished := false
	select {
	case result := <-done:
		finished = true
		if result != nil {
			t.Error("failed completion returned a result")
		}
	case <-time.After(time.Second):
		t.Error("cancellation did not release completion publication")
	}
	var terminal error
	seq(func(_ sdk.Event, err error) bool {
		if !finished {
			return false
		}
		if err != nil {
			terminal = err
		}
		return true
	})
	if !finished {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("completion cleanup blocked")
		}
		return
	}
	var remote *p.TurnError
	if !errors.As(terminal, &remote) || remote.Message != "remote failure" || errors.Is(terminal, context.Canceled) {
		t.Errorf("accepted completion error replaced: %v", terminal)
	}
}

func TestStreamCompletedBeforeCancellation(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		t.Run(route, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, transport, _ := startPublicationStream(t, ctx, route, 0)
			seq := stream.Events()
			transport.openGate()
			waitPublicationSignal(t, transport.injected, "turn start not returned")
			transport.InjectServerNotification(ctx, sdk.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			done := make(chan *sdk.RunResult, 1)
			go func() { done <- stream.Result() }()
			select {
			case result := <-done:
				if result == nil || result.Turn.Status != sdk.TurnStatusCompleted {
					t.Fatal("successful completion lost")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("completion did not finish")
			}
			cancel()
			for _, err := range seq {
				if err != nil {
					t.Fatalf("later cancellation changed completed stream: %v", err)
				}
			}
		})
	}
}

func TestStreamDeadlineDuringPendingReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, transport, _ := startPublicationStream(t, ctx, "collector", 65)
	seq := stream.Events()
	transport.openGate()
	waitPublicationSignal(t, transport.injected, "startup notifications not admitted")
	var result *sdk.RunResult
	done := make(chan struct{})
	go func() { result = stream.Result(); close(done) }()
	drained := false
	defer func() {
		if !drained {
			seq(func(sdk.Event, error) bool { return false })
		}
		waitPublicationSignal(t, done, "deadline lifecycle cleanup blocked")
	}()
	<-ctx.Done()
	select {
	case <-done:
		if result != nil {
			t.Error("expired replay returned a result")
		}
	case <-time.After(time.Second):
		t.Fatal("deadline did not release startup publication")
	}
	var terminal error
	for _, err := range seq {
		if err != nil {
			terminal = err
		}
	}
	drained = true
	if !errors.Is(terminal, context.DeadlineExceeded) {
		t.Errorf("terminal error = %v, want deadline", terminal)
	}
}
