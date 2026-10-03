package appserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
	codextransport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

type runHeldWriter struct {
	io.Writer
	frames  int
	held    chan struct{}
	release chan struct{}
}

func (w *runHeldWriter) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	if len(data) == 1 && data[0] == '\n' && err == nil {
		w.frames++
		if w.frames == 3 { // initialize, thread/start, then turn/start
			close(w.held)
			<-w.release
		}
	}
	return n, err
}

func TestRunAcceptedStartResponseBeforeWriterReturnAndEOF(t *testing.T) {
	t.Run("blocking", func(t *testing.T) { runAcceptedStartAtEOF(t, false, true) })
	t.Run("streamed", func(t *testing.T) { runAcceptedStartAtEOF(t, true, true) })
	t.Run("blocking without completion", func(t *testing.T) { runAcceptedStartAtEOF(t, false, false) })
	t.Run("streamed without completion", func(t *testing.T) { runAcceptedStartAtEOF(t, true, false) })
}

func runAcceptedStartAtEOF(t *testing.T, streamed, complete bool) {
	t.Helper()
	reader, peerOutput := io.Pipe()
	peerInput, writer := io.Pipe()
	held := &runHeldWriter{Writer: writer, held: make(chan struct{}), release: make(chan struct{})}
	tr := codextransport.NewStdioTransport(reader, held)
	t.Cleanup(func() {
		close(held.release)
		_ = tr.Close()
		_ = peerOutput.Close()
		_ = peerInput.Close()
		_ = writer.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !complete {
		go func() {
			select {
			case <-tr.ReaderStopped():
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	peerDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(peerInput)
		for scanner.Scan() {
			var req codex.Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				peerDone <- err
				return
			}
			var result interface{}
			switch req.Method {
			case "initialize":
				result = validInitializeResponseData("test")
			case "thread/start":
				result = validProcessThreadStartResponse(validProcessThreadPayload("thread-1"))
			case "turn/start":
				result = map[string]interface{}{"turn": map[string]interface{}{"id": "turn-1", "status": "inProgress", "items": []interface{}{}}}
			default:
				peerDone <- fmt.Errorf("unexpected method %s", req.Method)
				return
			}
			data, err := json.Marshal(map[string]interface{}{"id": req.ID.Value, "result": result})
			if err != nil {
				peerDone <- err
				return
			}
			if _, err := peerOutput.Write(append(data, '\n')); err != nil {
				peerDone <- err
				return
			}
			if req.Method == "turn/start" && complete {
				if _, err := io.WriteString(peerOutput, "{\"method\":\"item/completed\",\"params\":{\"completedAtMs\":1,\"threadId\":\"thread-1\",\"turnId\":\"turn-1\",\"item\":{\"type\":\"agentMessage\",\"id\":\"item-1\",\"text\":\"complete\"}}}\n"); err != nil {
					peerDone <- err
					return
				}
				if _, err := io.WriteString(peerOutput, "{\"method\":\"turn/completed\",\"params\":{\"threadId\":\"thread-1\",\"turn\":{\"id\":\"turn-1\",\"status\":\"completed\",\"items\":[]}}}\n"); err != nil {
					peerDone <- err
					return
				}
			}
			if req.Method == "turn/start" {
				peerDone <- peerOutput.Close()
				return
			}
		}
		peerDone <- scanner.Err()
	}()
	process := codex.NewProcessFromClient(codex.NewClient(tr))
	var result *codex.RunResult
	var runErr error
	if streamed {
		stream := process.RunStreamed(ctx, codex.RunOptions{Prompt: "complete"})
		var events []codex.Event
		for event, err := range stream.Events() {
			if err != nil {
				runErr = err
				break
			}
			events = append(events, event)
		}
		if complete && len(events) != 2 {
			t.Fatalf("events=%v", events)
		}
		if complete {
			if _, ok := events[0].(*codex.ItemCompleted); !ok {
				t.Fatalf("first event=%T", events[0])
			}
			if _, ok := events[1].(*codex.TurnCompleted); !ok {
				t.Fatalf("second event=%T", events[1])
			}
		} else if len(events) != 0 {
			t.Fatalf("events without completion=%v", events)
		}
		result = stream.Result()
	} else {
		result, runErr = process.Run(ctx, codex.RunOptions{Prompt: "complete"})
	}
	if !complete {
		if result != nil || !errors.Is(runErr, context.Canceled) {
			t.Fatalf("start became completion: result=%+v err=%v", result, runErr)
		}
	} else if runErr != nil || result == nil || result.Thread.ID != "thread-1" || result.Turn.ID != "turn-1" || result.Turn.Status != codex.TurnStatusCompleted || result.Response != "complete" || len(result.Items) != 1 {
		t.Fatalf("wrong result: %+v", result)
	}
	select {
	case <-held.held:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not reach held return")
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("peer did not finish")
	}
}
