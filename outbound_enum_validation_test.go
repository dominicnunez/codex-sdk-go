package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestClientRejectsInvalidOutboundEnumsBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		call    func(*codex.Client) error
	}{
		{
			name:    "config write merge strategy",
			wantErr: `invalid mergeStrategy "broken"`,
			call: func(client *codex.Client) error {
				_, err := client.Config.Write(context.Background(), codex.ConfigValueWriteParams{
					KeyPath:       "model",
					MergeStrategy: codex.MergeStrategy("broken"),
					Value:         json.RawMessage(`"gpt-5"`),
				})
				return err
			},
		},
		{
			name:    "config batch write merge strategy",
			wantErr: `invalid mergeStrategy "broken"`,
			call: func(client *codex.Client) error {
				_, err := client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{
					Edits: []codex.ConfigEdit{{
						KeyPath:       "model",
						MergeStrategy: codex.MergeStrategy("broken"),
						Value:         json.RawMessage(`"gpt-5"`),
					}},
				})
				return err
			},
		},
		{
			name:    "review start delivery",
			wantErr: `invalid delivery "sideways"`,
			call: func(client *codex.Client) error {
				delivery := codex.ReviewDelivery("sideways")
				_, err := client.Review.Start(context.Background(), codex.ReviewStartParams{
					ThreadID: "thread-1",
					Target: codex.ReviewTargetWrapper{
						Value: &codex.UncommittedChangesReviewTarget{},
					},
					Delivery: &delivery,
				})
				return err
			},
		},
		{
			name:    "turn start approvals reviewer",
			wantErr: `invalid approvalsReviewer "bot"`,
			call: func(client *codex.Client) error {
				reviewer := codex.ApprovalsReviewer("bot")
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID:          "thread-1",
					Input:             []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					ApprovalsReviewer: &reviewer,
				})
				return err
			},
		},
		{
			name:    "turn start reasoning effort",
			wantErr: `invalid reasoningEffort "turbo"`,
			call: func(client *codex.Client) error {
				effort := codex.ReasoningEffort("turbo")
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-1",
					Input:    []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					Effort:   &effort,
				})
				return err
			},
		},
		{
			name:    "turn start personality",
			wantErr: `invalid personality "chaotic"`,
			call: func(client *codex.Client) error {
				personality := codex.Personality("chaotic")
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID:    "thread-1",
					Input:       []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					Personality: &personality,
				})
				return err
			},
		},
		{
			name:    "turn start reasoning summary",
			wantErr: `invalid reasoningSummary "verbose"`,
			call: func(client *codex.Client) error {
				summary := codex.ReasoningSummaryWrapper{Value: codex.ReasoningSummaryMode("verbose")}
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-1",
					Input:    []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					Summary:  &summary,
				})
				return err
			},
		},
		{
			name:    "turn start collaboration mode",
			wantErr: `invalid mode "pair"`,
			call: func(client *codex.Client) error {
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-1",
					Input:    []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					CollaborationMode: &codex.CollaborationMode{
						Mode: codex.ModeKind("pair"),
						Settings: codex.CollaborationModeSettings{
							Model: "o3",
						},
					},
				})
				return err
			},
		},
		{
			name:    "turn start collaboration reasoning effort",
			wantErr: `invalid reasoningEffort "turbo"`,
			call: func(client *codex.Client) error {
				effort := codex.ReasoningEffort("turbo")
				_, err := client.Turn.Start(context.Background(), codex.TurnStartParams{
					ThreadID: "thread-1",
					Input:    []codex.UserInput{&codex.TextUserInput{Text: "hello"}},
					CollaborationMode: &codex.CollaborationMode{
						Mode: codex.ModeKindPlan,
						Settings: codex.CollaborationModeSettings{
							Model:           "o3",
							ReasoningEffort: &effort,
						},
					},
				})
				return err
			},
		},
		{
			name:    "thread start approvals reviewer",
			wantErr: `invalid approvalsReviewer "bot"`,
			call: func(client *codex.Client) error {
				reviewer := codex.ApprovalsReviewer("bot")
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{
					ApprovalsReviewer: &reviewer,
				})
				return err
			},
		},
		{
			name:    "thread start personality",
			wantErr: `invalid personality "chaotic"`,
			call: func(client *codex.Client) error {
				personality := codex.Personality("chaotic")
				_, err := client.Thread.Start(context.Background(), codex.ThreadStartParams{
					Personality: &personality,
				})
				return err
			},
		},
		{
			name:    "thread list sort key",
			wantErr: `invalid sortKey "rank"`,
			call: func(client *codex.Client) error {
				sortKey := codex.ThreadSortKey("rank")
				_, err := client.Thread.List(context.Background(), codex.ThreadListParams{
					SortKey: &sortKey,
				})
				return err
			},
		},
		{
			name:    "thread list source kind",
			wantErr: `invalid sourceKinds "daemon"`,
			call: func(client *codex.Client) error {
				_, err := client.Thread.List(context.Background(), codex.ThreadListParams{
					SourceKinds: []codex.ThreadSourceKind{codex.ThreadSourceKind("daemon")},
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if got := mock.CallCount(); got != 0 {
				t.Fatalf("transport recorded %d requests, want 0", got)
			}
		})
	}
}
