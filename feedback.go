package codex

import "context"

// FeedbackUploadParams represents parameters for uploading feedback.
type FeedbackUploadParams struct {
	// Classification of the feedback (e.g., "bug", "feature-request").
	Classification string `json:"classification"`
	// IncludeLogs indicates whether to include log files with the feedback.
	IncludeLogs bool `json:"includeLogs"`
	// Reason provides additional context for the feedback (optional).
	Reason *string `json:"reason,omitempty"`
	// ThreadID is the ID of the thread related to this feedback (optional).
	ThreadID *string `json:"threadId,omitempty"`
	// ExtraLogFiles is a list of additional log file paths to include (optional).
	ExtraLogFiles *[]string `json:"extraLogFiles,omitempty"`
}

// FeedbackUploadResponse represents the response from uploading feedback.
type FeedbackUploadResponse struct {
	// ThreadID is the ID of the thread created for this feedback.
	ThreadID string `json:"threadId"`
}

// FeedbackService provides methods for submitting user feedback.
type FeedbackService struct {
	client *Client
}

func newFeedbackService(client *Client) *FeedbackService {
	return &FeedbackService{client: client}
}

// Upload submits user feedback to the server.
func (s *FeedbackService) Upload(ctx context.Context, params FeedbackUploadParams) (FeedbackUploadResponse, error) {
	var resp FeedbackUploadResponse
	if err := s.client.sendRequest(ctx, methodFeedbackUpload, params, &resp); err != nil {
		return FeedbackUploadResponse{}, err
	}
	return resp, nil
}
