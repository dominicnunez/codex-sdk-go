package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadListLegacyFilterWireContract(t *testing.T) {
	tests := []struct {
		name   string
		params codex.ThreadListParams
		want   string
	}{
		{
			name:   "nil provider list stays omitted",
			params: codex.ThreadListParams{},
			want:   `{}`,
		},
		{
			name:   "empty provider list is explicit",
			params: codex.ThreadListParams{ModelProviders: []string{}},
			want:   `{"modelProviders":[]}`,
		},
		{
			name:   "populated provider list is preserved",
			params: codex.ThreadListParams{ModelProviders: []string{"openai"}},
			want:   `{"modelProviders":["openai"]}`,
		},
		{
			name:   "relative cwd spelling reaches server",
			params: codex.ThreadListParams{Cwd: codex.Ptr("repo/../work")},
			want:   `{"cwd":"repo/../work"}`,
		},
		{
			name:   "absolute cwd spelling reaches server",
			params: codex.ThreadListParams{Cwd: codex.Ptr("/workspace/../project")},
			want:   `{"cwd":"/workspace/../project"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			if err := transport.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{}}); err != nil {
				t.Fatalf("set response: %v", err)
			}
			client := codex.NewClient(transport)
			if _, err := client.Thread.List(context.Background(), tt.params); err != nil {
				t.Fatalf("Thread.List() error = %v", err)
			}
			request := transport.GetSentRequest(0)
			var got map[string]json.RawMessage
			if err := json.Unmarshal(request.Params, &got); err != nil {
				t.Fatalf("unmarshal params %s: %v", request.Params, err)
			}
			var want map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatalf("unmarshal expected params: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("Thread.List params = %s, want %s", request.Params, tt.want)
			}
			for key, expected := range want {
				if string(got[key]) != string(expected) {
					t.Errorf("Thread.List params[%q] = %s, want %s", key, got[key], expected)
				}
			}
		})
	}
}
