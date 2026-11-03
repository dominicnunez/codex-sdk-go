package appserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/appserver"
	"github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type publicationStdioTransport struct {
	*transport.StdioTransport
	accepted chan struct{}
	release  chan struct{}
}

func (t *publicationStdioTransport) Send(ctx context.Context, request sdk.Request) (sdk.Response, error) {
	response, err := t.StdioTransport.Send(ctx, request)
	if err == nil && request.Method == "turn/start" {
		close(t.accepted)
		select {
		case <-t.release:
		case <-ctx.Done():
		}
	}
	return response, err
}

func TestStreamCancellationDuringStdioReplay(t *testing.T) {
	peerRead, sdkWrite := io.Pipe()
	sdkRead, peerWrite := io.Pipe()
	stdio := transport.NewStdioTransport(sdkRead, sdkWrite)
	gated := &publicationStdioTransport{StdioTransport: stdio, accepted: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(gated.release) })
	client := sdk.NewClient(gated)
	process := sdk.NewProcessFromClient(client)
	ctx, cancel := context.WithCancel(context.Background())
	var writeMu sync.Mutex
	writeFrame := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return json.NewEncoder(peerWrite).Encode(value)
	}
	var peerErr error
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		scanner := bufio.NewScanner(peerRead)
		for scanner.Scan() {
			var request struct {
				Method string          `json:"method"`
				ID     json.RawMessage `json:"id"`
			}
			if peerErr = json.Unmarshal(scanner.Bytes(), &request); peerErr != nil {
				return
			}
			if len(request.ID) == 0 {
				continue
			}
			var result any
			switch request.Method {
			case "initialize":
				result = validInitializeResponseData("stdio-publication")
			case "thread/start":
				result = validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))
			case "turn/start":
				result = map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}}
			default:
				peerErr = fmt.Errorf("unexpected method %s", request.Method)
				return
			}
			if peerErr = writeFrame(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); peerErr != nil {
				return
			}
			if request.Method == "turn/start" {
				return
			}
		}
		peerErr = scanner.Err()
	}()
	defer func() {
		cancel()
		release()
		_ = client.Close()
		_ = sdkWrite.Close()
		_ = sdkRead.Close()
		_ = peerRead.Close()
		_ = peerWrite.Close()
		waitPublicationSignal(t, peerDone, "stdio fixture did not exit")
	}()
	stream := process.RunStreamed(ctx, sdk.RunOptions{Prompt: "stdio publication"})
	seq := stream.Events()
	waitPublicationSignal(t, gated.accepted, "stdio did not accept turn-start response")
	waitPublicationSignal(t, peerDone, "stdio peer did not finish handshake")
	if peerErr != nil {
		t.Fatal(peerErr)
	}
	// Register after the runtime listener, so observing a callback proves that
	// the runtime has already admitted that notification into its startup queue.
	var admitted atomic.Int32
	remove := client.AddNotificationListener("item/plan/delta", func(context.Context, sdk.Notification) { admitted.Add(1) })
	defer remove()
	for range 65 {
		if err := writeFrame(sdk.Notification{JSONRPC: "2.0", Method: "item/plan/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for admitted.Load() != 65 {
		if time.Now().After(deadline) {
			t.Fatal("stdio callbacks did not admit all startup events")
		}
		time.Sleep(time.Millisecond)
	}
	release()
	cancel()
	done := make(chan struct{})
	var result *sdk.RunResult
	go func() { result = stream.Result(); close(done) }()
	finished := false
	select {
	case <-done:
		finished = true
		if result != nil {
			t.Error("canceled stdio replay returned a result")
		}
	case <-time.After(time.Second):
		t.Error("stdio replay remained blocked after cancellation")
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
	waitPublicationSignal(t, done, "stdio lifecycle cleanup blocked")
	if finished && !errors.Is(terminal, context.Canceled) {
		t.Errorf("terminal error=%v, want cancellation", terminal)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	waitPublicationSignal(t, stdio.ReaderStopped(), "stdio reader did not stop")
}
