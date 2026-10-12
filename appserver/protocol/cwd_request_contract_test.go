package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestCwdRequestsForwardValuesUnchanged(t *testing.T) {
	methods := []struct {
		name   string
		method string
		call   func(*codex.Client, *string) error
	}{
		{
			name:   "command exec",
			method: "command/exec",
			call: func(client *codex.Client, cwd *string) error {
				_, err := client.Command.Exec(context.Background(), codex.CommandExecParams{Command: []string{"pwd"}, Cwd: cwd})
				return err
			},
		},
		{
			name:   "turn start",
			method: "turn/start",
			call: func(client *codex.Client, cwd *string) error {
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{ThreadID: "thread", Input: []codex.UserInput{&codex.TextUserInput{Text: "hello"}}, Cwd: cwd})
				return err
			},
		},
		{
			name:   "thread start",
			method: "thread/start",
			call: func(client *codex.Client, cwd *string) error {
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{Cwd: cwd})
				return err
			},
		},
		{
			name:   "thread resume",
			method: "thread/resume",
			call: func(client *codex.Client, cwd *string) error {
				_, err := client.Thread.Resume(context.Background(), codex.ThreadResumeParams{ThreadID: "thread", Cwd: cwd})
				return err
			},
		},
		{
			name:   "thread fork",
			method: "thread/fork",
			call: func(client *codex.Client, cwd *string) error {
				_, err := client.Thread.Fork(context.Background(), codex.ThreadForkParams{ThreadID: "thread", Cwd: cwd})
				return err
			},
		},
	}
	cwds := []struct {
		name string
		cwd  *string
		want *string
	}{
		{name: "relative", cwd: codex.Ptr("nested/../repo"), want: codex.Ptr("nested/../repo")},
		{name: "absolute spelling", cwd: codex.Ptr("/tmp/../workspace"), want: codex.Ptr("/tmp/../workspace")},
		{name: "omitted", cwd: nil, want: nil},
	}

	for _, method := range methods {
		for _, tcwd := range cwds {
			t.Run(method.name+"/"+tcwd.name, func(t *testing.T) {
				transport := NewMockTransport()
				defer transport.Close()
				if err := setCwdMethodResponse(t, transport, method.method); err != nil {
					t.Fatal(err)
				}
				client := codex.NewClient(transport)
				defer client.Close()
				if err := method.call(client, tcwd.cwd); err != nil {
					t.Fatalf("request failed: %v", err)
				}
				request := transport.GetSentRequest(0)
				if request == nil || request.Method != method.method {
					t.Fatalf("request = %v, want method %q", request, method.method)
				}
				var params map[string]json.RawMessage
				if err := json.Unmarshal(request.Params, &params); err != nil {
					t.Fatalf("decode params %s: %v", request.Params, err)
				}
				got, exists := params["cwd"]
				if tcwd.want == nil {
					if exists {
						t.Fatalf("cwd = %s, want omitted", got)
					}
					return
				}
				var gotValue string
				if !exists || json.Unmarshal(got, &gotValue) != nil || gotValue != *tcwd.want {
					t.Fatalf("cwd = %s, want original value %q", got, *tcwd.want)
				}
			})
		}
	}
}

func setCwdMethodResponse(t *testing.T, transport *MockTransport, method string) error {
	t.Helper()
	var response interface{}
	switch method {
	case "command/exec":
		response = map[string]interface{}{"exitCode": 0, "stdout": "", "stderr": ""}
	case "turn/start":
		response = map[string]interface{}{"turn": map[string]interface{}{"id": "turn", "status": "inProgress", "items": []interface{}{}}}
	case "thread/start", "thread/resume", "thread/fork":
		response = validThreadLifecycleResponse(validThreadPayload("thread"))
	default:
		t.Fatalf("unrecognized cwd request method %q", method)
	}
	return transport.SetResponseData(method, response)
}
