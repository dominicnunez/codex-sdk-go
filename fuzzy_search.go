package codex

import (
	"context"
	"encoding/json"
	"fmt"
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

func (r *FuzzyFileSearchResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "files"); err != nil {
		return err
	}
	type wire FuzzyFileSearchResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FuzzyFileSearchResponse(decoded)
	return nil
}

// FuzzyFileSearchResult represents a single file search result.
type FuzzyFileSearchResult struct {
	Path     string    `json:"path"`
	FileName string    `json:"file_name"`
	Root     string    `json:"root"`
	Score    uint32    `json:"score"`
	Indices  *[]uint32 `json:"indices,omitempty"`
}

func (r *FuzzyFileSearchResult) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "path", "file_name", "root", "score"); err != nil {
		return err
	}
	type wire FuzzyFileSearchResult
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = FuzzyFileSearchResult(decoded)
	return nil
}

// FuzzyFileSearchSessionCompletedNotification is sent when a fuzzy file search session completes.
type FuzzyFileSearchSessionCompletedNotification struct {
	SessionID string `json:"sessionId"`
}

func (n *FuzzyFileSearchSessionCompletedNotification) UnmarshalJSON(data []byte) error {
	type wire FuzzyFileSearchSessionCompletedNotification
	var decoded wire
	required := []string{"sessionId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = FuzzyFileSearchSessionCompletedNotification(decoded)
	return nil
}

// FuzzyFileSearchSessionUpdatedNotification is sent when a fuzzy file search session has new results.
type FuzzyFileSearchSessionUpdatedNotification struct {
	SessionID string                  `json:"sessionId"`
	Query     string                  `json:"query"`
	Files     []FuzzyFileSearchResult `json:"files"`
}

func (n *FuzzyFileSearchSessionUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire FuzzyFileSearchSessionUpdatedNotification
	var decoded wire
	required := []string{"files", "query", "sessionId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = FuzzyFileSearchSessionUpdatedNotification(decoded)
	return nil
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
	if err := s.client.sendRequest(ctx, methodFuzzyFileSearch, params, &resp); err != nil {
		return FuzzyFileSearchResponse{}, err
	}
	return resp, nil
}

// OnFuzzyFileSearchSessionCompleted registers a listener for fuzzyFileSearch/sessionCompleted notifications.
func (c *Client) OnFuzzyFileSearchSessionCompleted(handler func(FuzzyFileSearchSessionCompletedNotification)) {
	if handler == nil {
		c.OnNotification(notifyFuzzyFileSearchSessionCompleted, nil)
		return
	}
	c.OnNotification(notifyFuzzyFileSearchSessionCompleted, func(ctx context.Context, notif Notification) {
		var params FuzzyFileSearchSessionCompletedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyFuzzyFileSearchSessionCompleted, fmt.Errorf("unmarshal %s: %w", notifyFuzzyFileSearchSessionCompleted, err))
			return
		}
		handler(params)
	})
}

// OnFuzzyFileSearchSessionUpdated registers a listener for fuzzyFileSearch/sessionUpdated notifications.
func (c *Client) OnFuzzyFileSearchSessionUpdated(handler func(FuzzyFileSearchSessionUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyFuzzyFileSearchSessionUpdated, nil)
		return
	}
	c.OnNotification(notifyFuzzyFileSearchSessionUpdated, func(ctx context.Context, notif Notification) {
		var params FuzzyFileSearchSessionUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyFuzzyFileSearchSessionUpdated, fmt.Errorf("unmarshal %s: %w", notifyFuzzyFileSearchSessionUpdated, err))
			return
		}
		handler(params)
	})
}
