package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestShellCommandTimeoutValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout *int64
		valid   bool
	}{
		{"omitted", nil, true},
		{"immediate", codex.Ptr(int64(0)), true},
		{"positive", codex.Ptr(int64(1500)), true},
		{"negative", codex.Ptr(int64(-1)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			t.Cleanup(func() { _ = client.Close() })
			_, err := client.Thread.ShellCommand(context.Background(), codex.ThreadShellCommandParams{ThreadID: "t1", Command: "echo test", TimeoutMs: tc.timeout})
			if !tc.valid {
				if err == nil {
					t.Fatal("negative timeout accepted")
				}
				if transport.GetSentRequest(0) != nil {
					t.Fatal("invalid timeout reached transport")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var params map[string]json.RawMessage
			if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
				t.Fatal(err)
			}
			raw, present := params["timeoutMs"]
			if tc.timeout == nil {
				if present {
					t.Fatal("omitted timeout was sent")
				}
			} else {
				var timeout int64
				if !present {
					t.Fatal("explicit timeout omitted")
				}
				if err := json.Unmarshal(raw, &timeout); err != nil {
					t.Fatal(err)
				}
				if timeout != *tc.timeout {
					t.Fatalf("timeout = %d", timeout)
				}
			}
		})
	}
}
