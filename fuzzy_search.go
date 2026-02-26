package codex

import (
	"context"
	"encoding/json"
)

// FuzzyFileSearchSessionCompletedNotification is sent when a fuzzy file search session completes.
type FuzzyFileSearchSessionCompletedNotification struct {
	SessionID string `json:"sessionId"`
}

// FuzzyFileSearchSessionUpdatedNotification is sent when a fuzzy file search session has new results.
type FuzzyFileSearchSessionUpdatedNotification struct {
	SessionID string                    `json:"sessionId"`
	Query     string                    `json:"query"`
	Files     []FuzzyFileSearchResult   `json:"files"`
}

// OnFuzzyFileSearchSessionCompleted registers a listener for fuzzyFileSearch/sessionCompleted notifications.
func (c *Client) OnFuzzyFileSearchSessionCompleted(handler func(FuzzyFileSearchSessionCompletedNotification)) {
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
	c.OnNotification("fuzzyFileSearch/sessionUpdated", func(ctx context.Context, notif Notification) {
		var params FuzzyFileSearchSessionUpdatedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			return
		}
		handler(params)
	})
}
