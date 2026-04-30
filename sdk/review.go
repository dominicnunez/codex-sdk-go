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

var validReviewDeliveries = map[ReviewDelivery]struct{}{
	ReviewDeliveryInline:   {},
	ReviewDeliveryDetached: {},
}

func (d ReviewDelivery) MarshalJSON() ([]byte, error) {
	return marshalEnumString("delivery", d, validReviewDeliveries)
}

func (d *ReviewDelivery) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "delivery", validReviewDeliveries, d)
}

const (
	reviewTargetTypeUncommittedChanges = "uncommittedChanges"
	reviewTargetTypeBaseBranch         = "baseBranch"
	reviewTargetTypeCommit             = "commit"
	reviewTargetTypeCustom             = "custom"
)

// ReviewTarget is a discriminated union for review target types.
type ReviewTarget interface {
	reviewTarget()
}

// UncommittedChangesReviewTarget reviews the working tree: staged, unstaged, and untracked files.
type UncommittedChangesReviewTarget struct{}

func (*UncommittedChangesReviewTarget) reviewTarget() {}

func (u *UncommittedChangesReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{Type: reviewTargetTypeUncommittedChanges})
}

func (u *UncommittedChangesReviewTarget) UnmarshalJSON(data []byte) error {
	if err := validateReviewTargetVariantFields(data, reviewTargetTypeUncommittedChanges); err != nil {
		return err
	}
	*u = UncommittedChangesReviewTarget{}
	return nil
}

// BaseBranchReviewTarget reviews changes between the current branch and the given base branch.
type BaseBranchReviewTarget struct {
	Branch string `json:"branch"`
}

func (*BaseBranchReviewTarget) reviewTarget() {}

func (b *BaseBranchReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string `json:"type"`
		Branch string `json:"branch"`
	}{Type: reviewTargetTypeBaseBranch, Branch: b.Branch})
}

func (b *BaseBranchReviewTarget) UnmarshalJSON(data []byte) error {
	if err := validateReviewTargetVariantFields(data, reviewTargetTypeBaseBranch, "branch"); err != nil {
		return err
	}
	type wire BaseBranchReviewTarget
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*b = BaseBranchReviewTarget(decoded)
	return nil
}

// CommitReviewTarget reviews the changes introduced by a specific commit.
type CommitReviewTarget struct {
	SHA   string  `json:"sha"`
	Title *string `json:"title,omitempty"`
}

func (*CommitReviewTarget) reviewTarget() {}

func (c *CommitReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  string  `json:"type"`
		SHA   string  `json:"sha"`
		Title *string `json:"title,omitempty"`
	}{Type: reviewTargetTypeCommit, SHA: c.SHA, Title: c.Title})
}

func (c *CommitReviewTarget) UnmarshalJSON(data []byte) error {
	if err := validateReviewTargetVariantFields(data, reviewTargetTypeCommit, "sha"); err != nil {
		return err
	}
	type wire CommitReviewTarget
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = CommitReviewTarget(decoded)
	return nil
}

// CustomReviewTarget represents arbitrary instructions, equivalent to the old free-form prompt.
type CustomReviewTarget struct {
	Instructions string `json:"instructions"`
}

func (*CustomReviewTarget) reviewTarget() {}

func (c *CustomReviewTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string `json:"type"`
		Instructions string `json:"instructions"`
	}{Type: reviewTargetTypeCustom, Instructions: c.Instructions})
}

func (c *CustomReviewTarget) UnmarshalJSON(data []byte) error {
	if err := validateReviewTargetVariantFields(data, reviewTargetTypeCustom, "instructions"); err != nil {
		return err
	}
	type wire CustomReviewTarget
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = CustomReviewTarget(decoded)
	return nil
}

// UnknownReviewTarget represents an unrecognized review target type from a newer protocol version.
type UnknownReviewTarget struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

func (*UnknownReviewTarget) reviewTarget() {}

func (u *UnknownReviewTarget) MarshalJSON() ([]byte, error) {
	if u.Raw == nil {
		return []byte("null"), nil
	}
	return u.Raw, nil
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

	if raw.Type == "" {
		return fmt.Errorf("review target: missing or empty type field")
	}

	switch raw.Type {
	case reviewTargetTypeUncommittedChanges:
		var target UncommittedChangesReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	case reviewTargetTypeBaseBranch:
		var target BaseBranchReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	case reviewTargetTypeCommit:
		var target CommitReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	case reviewTargetTypeCustom:
		var target CustomReviewTarget
		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
		w.Value = &target
	default:
		w.Value = &UnknownReviewTarget{Type: raw.Type, Raw: append(json.RawMessage(nil), data...)}
	}

	return nil
}

func validateReviewTarget(target ReviewTarget) error {
	if isNilInterfaceValue(target) {
		return invalidParamsError("target must not be null")
	}

	switch t := target.(type) {
	case *UncommittedChangesReviewTarget:
		return nil
	case *BaseBranchReviewTarget:
		return validateRequiredNonEmptyStringField("target.branch", t.Branch)
	case *CommitReviewTarget:
		return validateRequiredNonEmptyStringField("target.sha", t.SHA)
	case *CustomReviewTarget:
		return validateRequiredNonEmptyStringField("target.instructions", t.Instructions)
	default:
		return validateRequiredJSONObjectField("target", target)
	}
}

func validateReviewTargetVariantFields(data []byte, wantType string, requiredFields ...string) error {
	if err := validateRequiredTaggedObjectFields(data, requiredFields...); err != nil {
		return err
	}

	var raw struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Type != wantType {
		return fmt.Errorf("review target: type %q does not match %q", raw.Type, wantType)
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

func (r *ReviewStartResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "reviewThreadId", "turn"); err != nil {
		return err
	}
	type wire ReviewStartResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ReviewStartResponse(decoded)
	return nil
}

func (p ReviewStartParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateRequiredJSONObjectField("target", p.Target); err != nil {
		return nil, err
	}
	if err := validateReviewTarget(p.Target.Value); err != nil {
		return nil, err
	}
	return p, nil
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
	if err := s.client.sendRequest(ctx, methodReviewStart, params, &resp); err != nil {
		return ReviewStartResponse{}, err
	}
	return resp, nil
}
