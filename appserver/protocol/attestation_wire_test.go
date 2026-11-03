package protocol_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	tr "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

func TestAttestationWire(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{}`, true}, {`null`, false}, {`[]`, false}, {" \t null \t", false}, {`{"future":{"n":1e999}}`, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			reader, peer := io.Pipe()
			output, writer := io.Pipe()
			transport := tr.NewStdioTransport(reader, writer)
			c := p.NewClient(transport)
			t.Cleanup(func() { _ = peer.Close(); _ = output.Close(); _ = writer.Close(); _ = c.Close() })
			var calls atomic.Int32
			c.SetApprovalHandlers(p.ApprovalHandlers{OnAttestationGenerate: func(context.Context, p.AttestationGenerateParams) (p.AttestationGenerateResponse, error) {
				calls.Add(1)
				return p.AttestationGenerateResponse{Token: "synthetic-attestation"}, nil
			}})
			type outcome struct {
				response p.Response
				raw      []byte
				err      error
			}
			done := make(chan outcome, 2)
			go func() {
				buffered := bufio.NewReader(output)
				for range 2 {
					line, err := buffered.ReadBytes('\n')
					var response p.Response
					if err == nil {
						err = json.Unmarshal(line, &response)
					}
					done <- outcome{response, line, err}
					if err != nil {
						return
					}
				}
			}()
			// The second request verifies recovery on the same real transport after
			// either a valid result or a classified rejection, with a distinct ID.
			for phase := range 2 {
				requestBody, id, valid := tc.body, "fixture", tc.valid
				if phase == 1 {
					requestBody, id, valid = `{}`, "recovery", true
				}
				before := calls.Load()
				fed := make(chan error, 1)
				go func() {
					_, err := io.WriteString(peer, `{"jsonrpc":"2.0","id":"`+id+`","method":"attestation/generate","params":`+requestBody+`}`+"\n")
					fed <- err
				}()
				select {
				case got := <-done:
					if got.err != nil || got.response.ID.Value != id {
						t.Fatalf("wire identity/parse failed: %+v %v", got.response, got.err)
					}
					if valid {
						if calls.Load() != before+1 || got.response.Error != nil || string(got.response.Result) != `{"token":"synthetic-attestation"}` {
							t.Fatal("valid wire reply failed")
						}
					} else if calls.Load() != before || len(got.response.Result) != 0 || got.response.Error == nil || got.response.Error.Code != p.ErrCodeInvalidParams || got.response.Error.Message != "invalid params" || strings.Contains(string(got.raw), "synthetic-attestation") {
						t.Fatalf("invalid wire request invoked callback or exposed token: callbacks=%d reply=%s", calls.Load(), got.raw)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("wire reply stuck")
				}
				select {
				case err := <-fed:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("input stuck")
				}
			}
		})
	}
}
