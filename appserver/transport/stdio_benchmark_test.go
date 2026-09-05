package transport

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type benchmarkInboundPayload struct {
	Content string `json:"content"`
}

// This comparator knows the payload type before routing. It is a lower-cost
// reference, not a replacement for the transport's arbitrary method dispatch.
type benchmarkConcreteFrame struct {
	JSONRPC inboundProtocolVersion  `json:"jsonrpc"`
	ID      inboundID               `json:"id"`
	Method  string                  `json:"method"`
	Params  benchmarkInboundPayload `json:"params"`
	Result  benchmarkInboundPayload `json:"result"`
	Error   inboundError            `json:"error"`
}

func BenchmarkInboundLarge(b *testing.B) {
	for _, kind := range []string{"Notification", "Request", "Response"} {
		for _, shape := range []string{"Plain", "Escaped"} {
			for _, size := range []int{64 << 10, 1 << 20, 5 << 20, 9 << 20} {
				b.Run(fmt.Sprintf("%s/%s/%dKiB", kind, shape, size>>10), func(b *testing.B) {
					unit := "QUJDREVGR0hJSktMTU5PUA=="
					if shape == "Escaped" {
						unit = "diff\n+\t\"path\\file\"\n-old\n"
					}
					encodedUnit, err := json.Marshal(unit)
					if err != nil {
						b.Fatal(err)
					}
					// Size denotes approximate encoded content bytes, so even the
					// escape-heavy 9 MiB case stays under the transport limit.
					payload, err := json.Marshal(benchmarkInboundPayload{Content: strings.Repeat(unit, size/(len(encodedUnit)-2))})
					if err != nil {
						b.Fatal(err)
					}
					prefix := `{"jsonrpc":"2.0","method":"benchmark/data","params":`
					if kind == "Request" {
						prefix = `{"jsonrpc":"2.0","id":1,"method":"benchmark/data","params":`
					}
					if kind == "Response" {
						prefix = `{"jsonrpc":"2.0","id":1,"result":`
					}
					frame := append(append([]byte(prefix), payload...), '}')
					if len(frame) >= maxInboundMessageSizeBytes {
						b.Fatal("fixture exceeds inbound limit")
					}
					for _, mode := range []string{"RouteOnly", "PayloadOnly", "RouteAndUnmarshal", "SingleConcreteParse"} {
						b.Run(mode, func(b *testing.B) {
							b.ReportAllocs()
							b.SetBytes(int64(len(frame)))
							if mode == "PayloadOnly" {
								b.SetBytes(int64(len(payload)))
							}
							for b.Loop() {
								switch mode {
								case "RouteOnly":
									if _, err := decodeInboundFrame(frame); err != nil {
										b.Fatal(err)
									}
								case "PayloadOnly":
									var value benchmarkInboundPayload
									if err := json.Unmarshal(payload, &value); err != nil {
										b.Fatal(err)
									}
								case "RouteAndUnmarshal":
									routed, err := decodeInboundFrame(frame)
									if err != nil {
										b.Fatal(err)
									}
									data := routed.Params
									if kind == "Response" {
										data = routed.Result
									}
									var value benchmarkInboundPayload
									if err := json.Unmarshal(data, &value); err != nil {
										b.Fatal(err)
									}
								case "SingleConcreteParse":
									var value benchmarkConcreteFrame
									if err := json.Unmarshal(frame, &value); err != nil {
										b.Fatal(err)
									}
								}
							}
						})
					}
				})
			}
		}
	}
}
