package protocol

import (
	"context"
	"encoding/json"
)

const (
	methodThreadItemsList = "thread/items/list"
	methodThreadTurnsList = "thread/turns/list"
	methodThreadRevert    = "thread/revert"
)

// ThreadHistoryMode describes how a thread's history is loaded.
type ThreadHistoryMode string

const (
	ThreadHistoryModeLegacy    ThreadHistoryMode = "legacy"
	ThreadHistoryModePaginated ThreadHistoryMode = "paginated"
)

var validThreadHistoryModes = map[ThreadHistoryMode]struct{}{
	ThreadHistoryModeLegacy: {}, ThreadHistoryModePaginated: {},
}

func (m ThreadHistoryMode) MarshalJSON() ([]byte, error) {
	return marshalEnumString("historyMode", m, validThreadHistoryModes)
}

func (m *ThreadHistoryMode) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "historyMode", validThreadHistoryModes, m)
}

// TurnItemsView describes how much item detail is included in a turn.
type TurnItemsView string

const (
	TurnItemsViewNotLoaded TurnItemsView = "notLoaded"
	TurnItemsViewSummary   TurnItemsView = "summary"
	TurnItemsViewFull      TurnItemsView = "full"
)

var validTurnItemsViews = map[TurnItemsView]struct{}{
	TurnItemsViewNotLoaded: {}, TurnItemsViewSummary: {}, TurnItemsViewFull: {},
}

func (v TurnItemsView) MarshalJSON() ([]byte, error) {
	return marshalEnumString("itemsView", v, validTurnItemsViews)
}

func (v *TurnItemsView) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "itemsView", validTurnItemsViews, v)
}

// ThreadItemsListParams selects a page of items, optionally restricted to one turn.
type ThreadItemsListParams struct {
	ThreadID      string         `json:"threadId"`
	TurnID        *string        `json:"turnId,omitempty"`
	Cursor        *string        `json:"cursor,omitempty"`
	Limit         *uint32        `json:"limit,omitempty"`
	SortDirection *SortDirection `json:"sortDirection,omitempty"`
}

func (p ThreadItemsListParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateOptionalEnumValue("sortDirection", p.SortDirection, validSortDirections); err != nil {
		return nil, err
	}
	return p, nil
}

// ThreadItemEntry associates a history item with its containing turn.
type ThreadItemEntry struct {
	Item   ThreadItemWrapper `json:"item"`
	TurnID string            `json:"turnId"`
}

func (e *ThreadItemEntry) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "item", "turnId"); err != nil {
		return err
	}
	type wire ThreadItemEntry
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = ThreadItemEntry(decoded)
	return nil
}

// ThreadItemsListResponse contains a page and opaque continuation cursors.
type ThreadItemsListResponse struct {
	Data            []ThreadItemEntry `json:"data"`
	NextCursor      *string           `json:"nextCursor,omitempty"`
	BackwardsCursor *string           `json:"backwardsCursor,omitempty"`
}

func (r *ThreadItemsListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ThreadItemsListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadItemsListResponse(decoded)
	return nil
}

// ItemsList retrieves persisted items. The server defaults to ascending order.
func (s *ThreadService) ItemsList(ctx context.Context, params ThreadItemsListParams) (ThreadItemsListResponse, error) {
	var response ThreadItemsListResponse
	err := s.client.sendRequest(ctx, methodThreadItemsList, params, &response)
	return response, err
}

// ThreadTurnsListParams selects a page of turns and their item detail level.
type ThreadTurnsListParams struct {
	ThreadID      string         `json:"threadId"`
	Cursor        *string        `json:"cursor,omitempty"`
	ItemsView     *TurnItemsView `json:"itemsView,omitempty"`
	Limit         *uint32        `json:"limit,omitempty"`
	SortDirection *SortDirection `json:"sortDirection,omitempty"`
}

func (p ThreadTurnsListParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateOptionalEnumValue("sortDirection", p.SortDirection, validSortDirections); err != nil {
		return nil, err
	}
	if err := validateOptionalEnumValue("itemsView", p.ItemsView, validTurnItemsViews); err != nil {
		return nil, err
	}
	return p, nil
}

// ThreadTurnsListResponse contains a page and opaque continuation cursors.
type ThreadTurnsListResponse struct {
	Data            []Turn  `json:"data"`
	NextCursor      *string `json:"nextCursor,omitempty"`
	BackwardsCursor *string `json:"backwardsCursor,omitempty"`
}

func (r *ThreadTurnsListResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "data"); err != nil {
		return err
	}
	type wire ThreadTurnsListResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadTurnsListResponse(decoded)
	return nil
}

// TurnsList retrieves persisted turns. The server defaults to descending order and summaries.
func (s *ThreadService) TurnsList(ctx context.Context, params ThreadTurnsListParams) (ThreadTurnsListResponse, error) {
	var response ThreadTurnsListResponse
	err := s.client.sendRequest(ctx, methodThreadTurnsList, params, &response)
	return response, err
}

// ThreadRevertParams selects the first turn to exclude from persisted history.
type ThreadRevertParams struct {
	ThreadID     string `json:"threadId"`
	BeforeTurnID string `json:"beforeTurnId"`
}

func (p ThreadRevertParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField("beforeTurnId", p.BeforeTurnID); err != nil {
		return nil, err
	}
	return p, nil
}

// ThreadRevertResponse contains updated metadata and cursors for retained history.
// Thread.Turns is empty; hydrate retained history using TurnsList or ItemsList.
type ThreadRevertResponse struct {
	Thread               Thread  `json:"thread"`
	ItemsBackwardsCursor *string `json:"itemsBackwardsCursor,omitempty"`
	TurnsBackwardsCursor *string `json:"turnsBackwardsCursor,omitempty"`
}

func (r *ThreadRevertResponse) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "thread"); err != nil {
		return err
	}
	type wire ThreadRevertResponse
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = ThreadRevertResponse(decoded)
	return nil
}

// Revert replaces a paginated thread's history with the prefix before a turn.
// It changes persisted conversation history, but does not revert local files.
func (s *ThreadService) Revert(ctx context.Context, params ThreadRevertParams) (ThreadRevertResponse, error) {
	var response ThreadRevertResponse
	if err := s.client.sendRequest(ctx, methodThreadRevert, params, &response); err != nil {
		return ThreadRevertResponse{}, err
	}
	s.client.cacheThreadState(response.Thread)
	return response, nil
}
