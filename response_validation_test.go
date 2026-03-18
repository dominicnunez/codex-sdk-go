package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestClientMethodsRejectMalformedSuccessResponses(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		missingObject interface{}
		invalidObject interface{}
		call          func(*codex.Client) error
	}{
		{
			name:          "feedback upload",
			method:        "feedback/upload",
			missingObject: map[string]interface{}{},
			call: func(client *codex.Client) error {
				_, err := client.Feedback.Upload(context.Background(), codex.FeedbackUploadParams{
					Classification: "bug",
					IncludeLogs:    true,
				})
				return err
			},
		},
		{
			name:          "experimental feature list",
			method:        "experimentalFeature/list",
			missingObject: map[string]interface{}{},
			invalidObject: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{},
				},
			},
			call: func(client *codex.Client) error {
				_, err := client.Experimental.FeatureList(context.Background(), codex.ExperimentalFeatureListParams{})
				return err
			},
		},
		{
			name:          "windows sandbox setup start",
			method:        "windowsSandbox/setupStart",
			missingObject: map[string]interface{}{},
			call: func(client *codex.Client) error {
				_, err := client.System.WindowsSandboxSetupStart(context.Background(), codex.WindowsSandboxSetupStartParams{
					Mode: codex.WindowsSandboxSetupModeElevated,
				})
				return err
			},
		},
		{
			name:          "turn start",
			method:        "turn/start",
			missingObject: map[string]interface{}{},
			invalidObject: map[string]interface{}{
				"turn": map[string]interface{}{},
			},
			call: func(client *codex.Client) error {
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-123",
					Input: []codex.UserInput{
						&codex.TextUserInput{Text: "hello"},
					},
				})
				return err
			},
		},
		{
			name:          "turn steer",
			method:        "turn/steer",
			missingObject: map[string]interface{}{},
			call: func(client *codex.Client) error {
				_, err := client.Turn.Steer(context.Background(), codex.TurnSteerParams{
					ThreadID:       "thread-123",
					ExpectedTurnID: "turn-456",
					Input: []codex.UserInput{
						&codex.TextUserInput{Text: "actually do this"},
					},
				})
				return err
			},
		},
		{
			name:          "review start",
			method:        "review/start",
			missingObject: map[string]interface{}{},
			invalidObject: map[string]interface{}{
				"reviewThreadId": "review-thread-123",
				"turn":           map[string]interface{}{},
			},
			call: func(client *codex.Client) error {
				_, err := client.Review.Start(context.Background(), codex.ReviewStartParams{
					ThreadID: "thread-123",
					Target: codex.ReviewTargetWrapper{
						Value: &codex.UncommittedChangesReviewTarget{},
					},
				})
				return err
			},
		},
		{
			name:          "thread unsubscribe",
			method:        "thread/unsubscribe",
			missingObject: map[string]interface{}{},
			call: func(client *codex.Client) error {
				_, err := client.Thread.Unsubscribe(context.Background(), codex.ThreadUnsubscribeParams{
					ThreadID: "thread-123",
				})
				return err
			},
		},
		{
			name:          "fuzzy file search",
			method:        "fuzzyFileSearch",
			missingObject: map[string]interface{}{},
			invalidObject: map[string]interface{}{
				"files": []interface{}{
					map[string]interface{}{},
				},
			},
			call: func(client *codex.Client) error {
				_, err := client.FuzzyFileSearch.Search(context.Background(), codex.FuzzyFileSearchParams{
					Query: "main",
					Roots: []string{"/tmp/project"},
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/null result", func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			mock.SetResponse(tt.method, codex.Response{
				JSONRPC: "2.0",
				Result:  json.RawMessage(`null`),
			})

			err := tt.call(client)
			if !errors.Is(err, codex.ErrEmptyResult) {
				t.Fatalf("error = %v; want ErrEmptyResult", err)
			}
		})

		t.Run(tt.name+"/missing required fields", func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			if err := mock.SetResponseData(tt.method, tt.missingObject); err != nil {
				t.Fatalf("SetResponseData(%q): %v", tt.method, err)
			}

			err := tt.call(client)
			if !errors.Is(err, codex.ErrMissingResultField) {
				t.Fatalf("error = %v; want ErrMissingResultField", err)
			}
		})

		if tt.invalidObject != nil {
			t.Run(tt.name+"/nested missing required fields", func(t *testing.T) {
				mock := NewMockTransport()
				client := codex.NewClient(mock)
				if err := mock.SetResponseData(tt.method, tt.invalidObject); err != nil {
					t.Fatalf("SetResponseData(%q): %v", tt.method, err)
				}

				err := tt.call(client)
				if !errors.Is(err, codex.ErrMissingResultField) {
					t.Fatalf("error = %v; want ErrMissingResultField", err)
				}
			})
		}
	}
}
