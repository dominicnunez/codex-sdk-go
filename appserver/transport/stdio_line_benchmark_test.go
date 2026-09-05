package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

func BenchmarkInboundReadLine(b *testing.B) {
	for _, size := range []int{64, 64 << 10, 1 << 20, 5 << 20, 9 << 20} {
		b.Run(fmt.Sprintf("%dBytes", size), func(b *testing.B) {
			frame := append(bytes.Repeat([]byte{'x'}, size), '\n')
			input := bytes.NewReader(frame)
			reader := bufio.NewReaderSize(input, readBufferSizeBytes)
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			for b.Loop() {
				input.Reset(frame)
				reader.Reset(input)
				line, oversized, err := readLimitedLine(reader, maxInboundMessageSizeBytes)
				if err != nil || oversized != nil || len(line) != size {
					b.Fatalf("read: length=%d oversized=%v error=%v", len(line), oversized, err)
				}
			}
		})
	}
}

// Compare stdlib streaming envelope decoding with the existing Unmarshal path.
// Both decode the same envelope and require a single complete JSON document.
func BenchmarkInboundEnvelopeDecoder(b *testing.B) {
	frame := append([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"`), bytes.Repeat([]byte{'x'}, 9<<20)...)
	frame = append(frame, []byte(`"}}`)...)
	for _, mode := range []string{"Unmarshal", "Decoder"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			for b.Loop() {
				if mode == "Unmarshal" {
					if _, err := decodeInboundFrame(frame); err != nil {
						b.Fatal(err)
					}
					continue
				}
				decoder := json.NewDecoder(bytes.NewReader(frame))
				var value inboundFrame
				if err := decoder.Decode(&value); err != nil {
					b.Fatal(err)
				}
				if _, err := decoder.Token(); err != io.EOF {
					b.Fatalf("trailing data: %v", err)
				}
			}
		})
	}
}
