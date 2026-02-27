package codex_test

import (
	"context"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestExperimentalFeatureList(t *testing.T) {
	tests := []struct {
		name           string
		params         codex.ExperimentalFeatureListParams
		responseData   map[string]interface{}
		wantDataLen    int
		wantNextCursor *string
	}{
		{
			name:   "minimal list",
			params: codex.ExperimentalFeatureListParams{},
			responseData: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"name":           "feature-x",
						"defaultEnabled": true,
						"enabled":        true,
						"stage":          "beta",
						"displayName":    "Feature X",
						"description":    "An experimental feature",
						"announcement":   "Try out Feature X!",
					},
					map[string]interface{}{
						"name":           "feature-y",
						"defaultEnabled": false,
						"enabled":        false,
						"stage":          "underDevelopment",
					},
				},
			},
			wantDataLen:    2,
			wantNextCursor: nil,
		},
		{
			name: "paginated list with cursor",
			params: codex.ExperimentalFeatureListParams{
				Cursor: ptr("cursor123"),
				Limit:  ptr(uint32(10)),
			},
			responseData: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"name":           "stable-feature",
						"defaultEnabled": true,
						"enabled":        true,
						"stage":          "stable",
					},
				},
				"nextCursor": "cursor456",
			},
			wantDataLen:    1,
			wantNextCursor: ptr("cursor456"),
		},
		{
			name:   "empty list",
			params: codex.ExperimentalFeatureListParams{},
			responseData: map[string]interface{}{
				"data": []interface{}{},
			},
			wantDataLen:    0,
			wantNextCursor: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_ = mock.SetResponseData("experimentalFeature/list", tt.responseData)

			resp, err := client.Experimental.FeatureList(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("FeatureList failed: %v", err)
			}

			if len(resp.Data) != tt.wantDataLen {
				t.Errorf("got %d features, want %d", len(resp.Data), tt.wantDataLen)
			}

			if (resp.NextCursor == nil) != (tt.wantNextCursor == nil) {
				t.Errorf("NextCursor presence mismatch: got %v, want %v", resp.NextCursor, tt.wantNextCursor)
			}

			if resp.NextCursor != nil && tt.wantNextCursor != nil && *resp.NextCursor != *tt.wantNextCursor {
				t.Errorf("NextCursor = %v, want %v", *resp.NextCursor, *tt.wantNextCursor)
			}

			// Verify first feature in paginated test
			if tt.name == "paginated list with cursor" && len(resp.Data) > 0 {
				f := resp.Data[0]
				if f.Name != "stable-feature" {
					t.Errorf("feature.Name = %v, want stable-feature", f.Name)
				}
				if f.Stage != codex.ExperimentalFeatureStageStable {
					t.Errorf("feature.Stage = %v, want stable", f.Stage)
				}
			}

			// Verify all 5 feature stages work
			if tt.name == "minimal list" && len(resp.Data) >= 2 {
				f0 := resp.Data[0]
				if f0.Stage != codex.ExperimentalFeatureStageBeta {
					t.Errorf("feature 0 stage = %v, want beta", f0.Stage)
				}
				if f0.DisplayName == nil || *f0.DisplayName != "Feature X" {
					t.Errorf("feature 0 displayName = %v, want Feature X", f0.DisplayName)
				}

				f1 := resp.Data[1]
				if f1.Stage != codex.ExperimentalFeatureStageUnderDevelopment {
					t.Errorf("feature 1 stage = %v, want underDevelopment", f1.Stage)
				}
				if f1.DisplayName != nil {
					t.Errorf("feature 1 displayName = %v, want nil", f1.DisplayName)
				}
			}
		})
	}
}

func TestExperimentalServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Compile-time verification that ExperimentalService has all required methods
	var _ interface {
		FeatureList(context.Context, codex.ExperimentalFeatureListParams) (codex.ExperimentalFeatureListResponse, error)
	} = client.Experimental
}
