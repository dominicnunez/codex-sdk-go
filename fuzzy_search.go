package codex

import (
	"context"
	"encoding/json"
)

// FuzzyFileSearchParams represents parameters for fuzzy file search.
type FuzzyFileSearchParams struct {
	Query             string   `json:"query"`
	Roots             []string `json:"roots"`
	CancellationToken *string  `json:"cancellationToken,omitempty"`
}

// FuzzyFileSearchResponse represents the response containing search results.
type FuzzyFileSearchResponse struct {
	Files []FuzzyFileSearchResult `json:"files"`
}

// FuzzyFileSearchResult represents a single file search result.
type FuzzyFileSearchResult struct {
	Path     string    `json:"path"`
	FileName string    `json:"file_name"`
	Root     string    `json:"root"`
	Score    uint32    `json:"score"`
	Indices  *[]uint32 `json:"indices,omitempty"`
}

// FuzzyFileSearchSessionCompletedNotification is sent when a fuzzy file search session completes.
type FuzzyFileSearchSessionCompletedNotification struct {
	SessionID string `json:"sessionId"`
}

// FuzzyFileSearchSessionUpdatedNotification is sent when a fuzzy file search session has new results.
type FuzzyFileSearchSessionUpdatedNotification struct {
	SessionID string                  `json:"sessionId"`
	Query     string                  `json:"query"`
	Files     []FuzzyFileSearchResult `json:"files"`
}

// FuzzyFileSearchService provides fuzzy file search operations.
type FuzzyFileSearchService struct {
	client *Client
}

func newFuzzyFileSearchService(client *Client) *FuzzyFileSearchService {
	return &FuzzyFileSearchService{client: client}
}

// Search performs a fuzzy file search.
func (s *FuzzyFileSearchService) Search(ctx context.Context, params FuzzyFileSearchParams) (FuzzyFileSearchResponse, error) {
	var resp FuzzyFileSearchResponse
	if err := s.client.sendRequest(ctx, "fuzzyFileSearch", params, &resp); err != nil {
		return FuzzyFileSearchResponse{}, err
	}
	return resp, nil
}

// OnFuzzyFileSearchSessionCompleted registers a listener for fuzzyFileSearch/sessionCompleted notifications.
func (c *Client) OnFuzzyFileSearchSessionCompleted(handler func(FuzzyFileSearchSessionCompletedNotification)) {
	if handler == nil {
		c.OnNotification("fuzzyFileSearch/sessionCompleted", nil)
		return
	}
	c.OnNotification("fuzzyFileSearch/sessionCompleted", func(ctx context.Context, notif Notification) {
		var params FuzzyFileSearchSessionCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}

// OnFuzzyFileSearchSessionUpdated registers a listener for fuzzyFileSearch/sessionUpdated notifications.
func (c *Client) OnFuzzyFileSearchSessionUpdated(handler func(FuzzyFileSearchSessionUpdatedNotification)) {
	if handler == nil {
		c.OnNotification("fuzzyFileSearch/sessionUpdated", nil)
		return
	}
	c.OnNotification("fuzzyFileSearch/sessionUpdated", func(ctx context.Context, notif Notification) {
		var params FuzzyFileSearchSessionUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}
