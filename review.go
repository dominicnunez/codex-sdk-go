package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// ReviewDelivery specifies where to run the review.
type ReviewDelivery string

const (
	// ReviewDeliveryInline runs the review on the current thread (default).
	ReviewDeliveryInline ReviewDelivery = "inline"
	// ReviewDeliveryDetached runs the review on a new thread.
	ReviewDeliveryDetached ReviewDelivery = "detached"
)

// ReviewTarget is a discriminated union for review target types.
type ReviewTarget interface {
	reviewTarget()
}

// UncommittedChangesReviewTarget reviews the working tree: staged, unstaged, and untracked files.
type UncommittedChangesReviewTarget struct{}

func (*UncommittedChangesReviewTarget) reviewTarget() {}

func (u *UncommittedChangesReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "uncommittedChanges",
	})
}

// BaseBranchReviewTarget reviews changes between the current branch and the given base branch.
type BaseBranchReviewTarget struct {
	Branch string `json:"branch"`
}

func (*BaseBranchReviewTarget) reviewTarget() {}

func (b *BaseBranchReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   "baseBranch",
		"branch": b.Branch,
	})
}

// CommitReviewTarget reviews the changes introduced by a specific commit.
type CommitReviewTarget struct {
	SHA   string  `json:"sha"`
	Title *string `json:"title,omitempty"`
}

func (*CommitReviewTarget) reviewTarget() {}

func (c *CommitReviewTarget) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"type": "commit",
		"sha":  c.SHA,
	}
	if c.Title != nil {
		data["title"] = *c.Title
	}
	return json.Marshal(data)
}

// CustomReviewTarget represents arbitrary instructions, equivalent to the old free-form prompt.
type CustomReviewTarget struct {
	Instructions string `json:"instructions"`
}

func (*CustomReviewTarget) reviewTarget() {}

func (c *CustomReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":         "custom",
		"instructions": c.Instructions,
	})
}

// ReviewTargetWrapper wraps a ReviewTarget for JSON marshaling/unmarshaling.
type ReviewTargetWrapper struct {
	Value ReviewTarget
}

func (w ReviewTargetWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Value)
}

func (w *ReviewTargetWrapper) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	typeStr := raw.Type
	if typeStr == "" {
		return fmt.Errorf("review target: missing or empty type key")
	}

	switch typeStr {
	case "uncommittedChanges":
		w.Value = &UncommittedChangesReviewTarget{}
	case "baseBranch":
		var target BaseBranchReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	case "commit":
		var target CommitReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	case "custom":
		var target CustomReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	default:
		return fmt.Errorf("unknown review target type: %s", typeStr)
	}

	return nil
}

// ReviewStartParams contains parameters for starting a review.
type ReviewStartParams struct {
	ThreadID string              `json:"threadId"`
	Target   ReviewTargetWrapper `json:"target"`
	Delivery *ReviewDelivery     `json:"delivery,omitempty"`
}

// ReviewStartResponse is the response from starting a review.
type ReviewStartResponse struct {
	// ReviewThreadID identifies the thread where the review runs.
	// For inline reviews, this is the original thread id.
	// For detached reviews, this is the id of the new review thread.
	ReviewThreadID string `json:"reviewThreadId"`
	Turn           Turn   `json:"turn"`
}

// ReviewService provides methods for code review operations.
type ReviewService struct {
	client *Client
}

func newReviewService(client *Client) *ReviewService {
	return &ReviewService{client: client}
}

// Start initiates a code review based on the provided parameters.
func (s *ReviewService) Start(ctx context.Context, params ReviewStartParams) (ReviewStartResponse, error) {
	var resp ReviewStartResponse
	if err := s.client.sendRequest(ctx, "review/start", params, &resp); err != nil {
		return ReviewStartResponse{}, err
	}
	return resp, nil
}
