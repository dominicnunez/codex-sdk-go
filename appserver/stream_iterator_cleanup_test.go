package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	sdk "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestStreamIteratorExitReleasesConsumer(t *testing.T) {
	for _, route := range []string{"stream", "collector", "conversation"} {
		for _, exit := range []string{"panic", "goexit", "early-return"} {
			t.Run(route+"/"+exit, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stream, transport, collector := startPublicationStream(t, ctx, route, 0)
				seq := stream.Events()
				transport.openGate()
				waitPublicationSignal(t, transport.injected, "turn start not returned")
				waitForRunStreamedReady(t, transport.MockTransport)
				delta := sdk.Notification{Method: "item/plan/delta", Params: json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"plan","delta":"x"}`)}
				transport.InjectServerNotification(ctx, delta)
				marker := &struct{ message string }{"application panic"}
				var caught any
				calls := 0
				iteratorDone := make(chan struct{})
				go func() {
					defer close(iteratorDone)
					defer func() { caught = recover() }()
					seq(func(sdk.Event, error) bool {
						calls++
						switch exit {
						case "panic":
							panic(marker)
						case "goexit":
							runtime.Goexit()
						}
						return false
					})
				}()
				waitPublicationSignal(t, iteratorDone, "iterator did not exit")
				if (exit == "panic" && caught != marker) || (exit != "panic" && caught != nil) {
					t.Fatalf("original panic changed: %v", caught)
				}
				for _, err := range stream.Events() {
					if !errors.Is(err, sdk.ErrStreamConsumed) {
						t.Fatalf("second Events error = %v", err)
					}
				}
				producerDone := make(chan struct{})
				go func() {
					defer close(producerDone)
					for range 100 {
						transport.InjectServerNotification(ctx, delta)
					}
					transport.InjectServerNotification(ctx, sdk.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
				}()
				done := make(chan struct{})
				var result *sdk.RunResult
				go func() { result = stream.Result(); close(done) }()
				defer func() {
					cancel()
					waitPublicationSignal(t, producerDone, "producer cleanup blocked")
					waitPublicationSignal(t, done, "lifecycle cleanup blocked")
				}()
				select {
				case <-done:
					if result == nil || result.Turn.Status != sdk.TurnStatusCompleted {
						t.Fatal("iterator exit lost successful completion")
					}
				case <-time.After(time.Second):
					t.Fatal("Result blocked after iterator exit")
				}
				waitPublicationSignal(t, producerDone, "producer stayed blocked after completion")
				if calls != 1 {
					t.Errorf("callback called %d times after exit", calls)
				}
				if route == "collector" {
					summary := collector.Summary()
					if summary.LatestPlanText == nil || len(*summary.LatestPlanText) != 101 {
						t.Error("iterator detachment interrupted collection")
					}
				}
			})
		}
	}
}
