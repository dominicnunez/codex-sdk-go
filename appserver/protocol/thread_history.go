package protocol

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
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
	ThreadID string  `json:"threadId"`
	TurnID   *string `json:"turnId,omitempty"`
	Cursor   *string `json:"cursor,omitempty"`
	// CursorAnchor selects an item-relative position. It is mutually exclusive with Cursor.
	CursorAnchor  *ThreadItemsListAnchor `json:"-"`
	Limit         *uint32                `json:"limit,omitempty"`
	SortDirection *SortDirection         `json:"sortDirection,omitempty"`
}

func (p ThreadItemsListParams) MarshalJSON() ([]byte, error) {
	if p.Cursor != nil && p.CursorAnchor != nil {
		return nil, errors.New("cursor and cursor anchor are mutually exclusive")
	}
	type wire struct {
		ThreadID      string         `json:"threadId"`
		TurnID        *string        `json:"turnId,omitempty"`
		Cursor        interface{}    `json:"cursor,omitempty"`
		Limit         *uint32        `json:"limit,omitempty"`
		SortDirection *SortDirection `json:"sortDirection,omitempty"`
	}
	var cursor interface{}
	if p.Cursor != nil {
		cursor = *p.Cursor
	}
	if p.CursorAnchor != nil {
		cursor = p.CursorAnchor
	}
	return json.Marshal(wire{p.ThreadID, p.TurnID, cursor, p.Limit, p.SortDirection})
}

// UnmarshalJSON distinguishes the schema's string and item-anchor cursor variants.
func (p *ThreadItemsListParams) UnmarshalJSON(data []byte) error {
	type wire struct {
		ThreadID      string          `json:"threadId"`
		TurnID        *string         `json:"turnId,omitempty"`
		Cursor        json.RawMessage `json:"cursor,omitempty"`
		Limit         *uint32         `json:"limit,omitempty"`
		SortDirection *SortDirection  `json:"sortDirection,omitempty"`
	}
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = ThreadItemsListParams{ThreadID: decoded.ThreadID, TurnID: decoded.TurnID, Limit: decoded.Limit, SortDirection: decoded.SortDirection}
	if len(decoded.Cursor) == 0 || string(decoded.Cursor) == "null" {
		return nil
	}
	if decoded.Cursor[0] == '"' {
		return jsondecode.Unmarshal(decoded.Cursor, &p.Cursor)
	}
	var anchor ThreadItemsListAnchor
	if err := jsondecode.Unmarshal(decoded.Cursor, &anchor); err != nil {
		return err
	}
	p.CursorAnchor = &anchor
	return nil
}

// ThreadItemsListAnchor identifies an exclusive item position in a visible turn.
type ThreadItemsListAnchor struct {
	ItemID string `json:"itemId"`
}

func (a ThreadItemsListAnchor) MarshalJSON() ([]byte, error) {
	if a.ItemID == "" {
		return nil, errors.New("thread items anchor requires itemId")
	}
	return json.Marshal(struct {
		Type   string `json:"type"`
		ItemID string `json:"itemId"`
	}{Type: "item", ItemID: a.ItemID})
}

func (a *ThreadItemsListAnchor) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type   string `json:"type"`
		ItemID string `json:"itemId"`
	}
	if err := jsondecode.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Type != "item" {
		return errors.New("thread items anchor requires type item")
	}
	if wire.ItemID == "" {
		return errors.New("thread items anchor requires itemId")
	}
	*a = ThreadItemsListAnchor{ItemID: wire.ItemID}
	return nil
}

// ThreadItemsListCursor supports either an opaque continuation token or an item anchor.
func (p ThreadItemsListParams) prepareRequest() (interface{}, error) {
	if err := validateThreadScopedRequest(p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateOptionalEnumValue("sortDirection", p.SortDirection, validSortDirections); err != nil {
		return nil, err
	}
	if p.Cursor != nil && p.CursorAnchor != nil {
		return nil, invalidParamsError("cursor and cursor anchor are mutually exclusive")
	}
	if p.CursorAnchor != nil {
		if p.TurnID == nil || *p.TurnID == "" {
			return nil, invalidParamsError("cursor anchor requires turnId")
		}
		if p.CursorAnchor.ItemID == "" {
			return nil, invalidParamsError("cursor anchor requires itemId")
		}
	}
	return p, nil
}

// ThreadItemEntry associates a history item with its containing turn.
type ThreadItemEntry struct {
	Item          ThreadItemWrapper `json:"item"`
	TurnID        string            `json:"turnId"`
	StartedAtMs   *int64            `json:"startedAtMs,omitempty"`
	CompletedAtMs *int64            `json:"completedAtMs,omitempty"`
}

func (e *ThreadItemEntry) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "item", "turnId"); err != nil {
		return err
	}
	type wire ThreadItemEntry
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
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
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
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
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
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
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
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
	s.client.cacheThreadStateForMethod(methodThreadRevert, response.Thread)
	return response, nil
}
