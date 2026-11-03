package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

type benchmarkStartTransport struct {
	*startStateTransport
	entered chan struct{}
	traffic int
}

func (tr *benchmarkStartTransport) Send(ctx context.Context, req Request) (Response, error) {
	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{}, err
	}
	if params.Model == "hold" {
		tr.entered <- struct{}{}
		<-ctx.Done()
		return Response{}, ctx.Err()
	}
	tr.started("target", "notification")
	for i := range tr.traffic {
		tr.started(fmt.Sprint("other-", i), "notification")
	}
	return Response{ID: req.ID, Result: startStateResponse("target")}, nil
}

// Measure the complete public acquisition and release, with both candidate
// traffic and other pending public starts scaled independently.
func BenchmarkThreadStartObservation(b *testing.B) {
	for _, pending := range []int{0, 8, 64} {
		for _, traffic := range []int{0, 32, 64, 65} {
			b.Run(fmt.Sprintf("pending=%d/traffic=%d", pending, traffic), func(b *testing.B) {
				tr := &benchmarkStartTransport{startStateTransport: &startStateTransport{}, entered: make(chan struct{}, pending), traffic: traffic}
				client := NewClient(tr)
				ctx, cancel := context.WithCancel(context.Background())
				var wg sync.WaitGroup
				for range pending {
					wg.Go(func() {
						model := "hold"
						_, _, remove, _ := client.Thread.StartWithStateListener(ctx, ThreadStartParams{Model: &model}, nil, nil)
						remove()
					})
				}
				for range pending {
					<-tr.entered
				}
				b.Cleanup(func() { cancel(); wg.Wait(); _ = client.Close() })
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_, _, remove, err := client.Thread.StartWithStateListener(ctx, ThreadStartParams{}, func(Thread) {}, nil)
					if err != nil {
						b.Fatal(err)
					}
					remove()
				}
			})
		}
	}
}
