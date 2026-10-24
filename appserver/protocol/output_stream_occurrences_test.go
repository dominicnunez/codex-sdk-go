package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestOutputStreamOccurrences(t *testing.T) {
	t.Run("process", func(t *testing.T) {
		checkOutputStreamOccurrences(t, "process/outputDelta", "processHandle", func(c *p.Client, h func(p.ProcessOutputDeltaNotification), add bool) {
			if add {
				c.AddProcessOutputDeltaListener(h)
			} else {
				c.OnProcessOutputDelta(h)
			}
		}, func(n p.ProcessOutputDeltaNotification) string { return string(n.Stream) })
	})
	t.Run("command", func(t *testing.T) {
		checkOutputStreamOccurrences(t, "command/exec/outputDelta", "processId", func(c *p.Client, h func(p.CommandExecOutputDeltaNotification), add bool) {
			if add {
				c.AddCommandExecOutputDeltaListener(h)
			} else {
				c.OnCommandExecOutputDelta(h)
			}
		}, func(n p.CommandExecOutputDeltaNotification) string { return string(n.Stream) })
	})
}

func checkOutputStreamOccurrences[T any](t *testing.T, method, id string, register func(*p.Client, func(T), bool), stream func(T) string) {
	t.Helper()
	prefix := fmt.Sprintf(`{"capReached":false,"deltaBase64":"YQ==",%q:"p",`, id)
	for _, tc := range []struct {
		name, fields, want string
	}{
		{"stdout", `"stream":"stdout"`, "stdout"},
		{"stderr", `"stream":"stderr"`, "stderr"},
		{"valid duplicates", `"stream":"stdout","str\u0065am":"stderr"`, "stderr"},
		{"invalid first", `"stream":"future","stream":"stdout"`, ""},
		{"invalid escaped", `"str\u0065am":"future","stream":"stdout"`, ""},
		{"invalid long", fmt.Sprintf(`"stream":%q,"stream":"stdout"`, strings.Repeat("x", 16000)), ""},
		{"invalid last", `"stream":"stdout","stream":"future"`, ""},
		{"wrong type first", `"stream":1,"stream":"stdout"`, ""},
		{"null first", `"stream":null,"stream":"stdout"`, ""},
		{"missing", `"extra":true`, ""},
		{"unknown case extra", `"STREAM":"future","stream":"stdout"`, "stdout"},
		{"nested extra", `"extra":{"stream":"future"},"stream":"stderr"`, "stderr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := json.RawMessage(prefix + tc.fields + "}")
			var decoded T
			// Seed the receiver to verify failures do not partially replace it.
			if err := json.Unmarshal([]byte(prefix+`"stream":"stderr"}`), &decoded); err != nil {
				t.Fatal(err)
			}
			before := decoded
			err := json.Unmarshal(wire, &decoded)
			if (err == nil) != (tc.want != "") {
				t.Errorf("decode error = %v, want stream %q", err, tc.want)
			}
			if err != nil && !reflect.DeepEqual(decoded, before) {
				t.Fatal("rejected notification changed receiver")
			}
			if err != nil && len(err.Error()) > 4096 {
				t.Fatalf("unbounded stream diagnostic: %d bytes", len(err.Error()))
			}
			if err == nil && tc.want != "" && stream(decoded) != tc.want {
				t.Fatalf("stream = %q, want %q", stream(decoded), tc.want)
			}
			for _, add := range []bool{false, true} {
				mock := NewMockTransport()
				calls, failures := 0, 0
				want := tc.want
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, _ error) { failures++ }))
				register(client, func(n T) {
					calls++
					if stream(n) != want {
						t.Errorf("callback stream = %q, want %q", stream(n), want)
					}
				}, add)
				mock.InjectServerNotification(context.Background(), p.Notification{JSONRPC: "2.0", Method: method, Params: wire})
				if tc.want == "" {
					if calls != 0 || failures != 1 {
						t.Fatalf("add=%v: rejected notification calls=%d errors=%d", add, calls, failures)
					}
					want = "stderr"
					mock.InjectServerNotification(context.Background(), p.Notification{JSONRPC: "2.0", Method: method, Params: json.RawMessage(prefix + `"stream":"stderr"}`)})
					if calls != 1 || failures != 1 {
						t.Fatalf("add=%v: recovery calls=%d errors=%d", add, calls, failures)
					}
				} else if calls != 1 || failures != 0 {
					t.Fatalf("add=%v: valid notification calls=%d errors=%d", add, calls, failures)
				}
			}
		})
	}
	// Preserve the existing base64 error when an earlier invalid enum would
	// previously have been overwritten by a valid stream.
	var decoded T
	wire := strings.Replace(prefix, `"YQ=="`, `"!"`, 1) + `"stream":"future","stream":"stdout"}`
	if err := json.Unmarshal([]byte(wire), &decoded); err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("existing base64 error replaced: %v", err)
	}
}
