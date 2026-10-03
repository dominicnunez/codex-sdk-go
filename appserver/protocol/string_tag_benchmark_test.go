package protocol_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type stringTagBenchmarkEnvelope struct {
	Data  protocol.OptionalNullable[stringTagRecord[stringTagNamedInt[struct{}]]] `json:"data"`
	After bool                                                                    `json:"after"`
}

func BenchmarkStringTagPublicDecode(b *testing.B) {
	for _, size := range []int{1 << 10, 1 << 20} {
		for _, escaped := range []bool{false, true} {
			for _, saved := range []bool{false, true} {
				literal := strings.Repeat("x", size)
				if escaped {
					literal = strings.Repeat("\x00", size)
				}
				if saved {
					literal = "null" + literal
				}
				quoted, err := json.Marshal(literal)
				if err != nil {
					b.Fatal(err)
				}
				input := []byte(`{"data":{"before":7,"n":` + string(quoted) + `,"after":"new"},"after":true}`)
				for _, reused := range []bool{false, true} {
					name := fmt.Sprintf("bytes=%d/escaped=%t/saved=%t/reused=%t", size, escaped, saved, reused)
					b.Run(name, func(b *testing.B) {
						value := stringTagBenchmarkEnvelope{Data: protocol.OptionalNullable[stringTagRecord[stringTagNamedInt[struct{}]]]{Present: true, Value: &stringTagRecord[stringTagNamedInt[struct{}]]{N: 3}}}
						b.ReportAllocs()
						b.ResetTimer()
						for b.Loop() {
							if !reused {
								value = stringTagBenchmarkEnvelope{}
							}
							err := json.Unmarshal(input, &value)
							if err == nil {
								b.Fatal("invalid string-tag input admitted")
							}
							_ = err.Error()
						}
					})
				}
			}
		}
	}
}
