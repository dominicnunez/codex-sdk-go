package protocol

// ThreadSearchResult is a provisional search match. Final search results are authoritative.
type ThreadSearchResult struct {
	Snippet string `json:"snippet"`
	Thread  Thread `json:"thread"`
}

// ThreadSearchPartialNotification carries a best-effort batch correlated by request ID.
type ThreadSearchPartialNotification struct {
	Data      []ThreadSearchResult `json:"data"`
	RequestID RequestID            `json:"requestId"`
}

func (c *Client) OnThreadSearchPartial(h func(ThreadSearchPartialNotification)) {
	setTypedNotificationHandler(c, NotifyThreadSearchPartial, h)
}
func (c *Client) AddThreadSearchPartialListener(h func(ThreadSearchPartialNotification)) func() {
	return addTypedNotificationListener(c, NotifyThreadSearchPartial, h)
}
func (n *ThreadSearchPartialNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadSearchPartialNotification
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"data", "requestId"}, []string{"data", "requestId"}); err != nil {
		return err
	}
	state := searchResultPresence{}
	if err := validateFields(data, func(_ string, raw []byte) error { return state.observe(raw) }, false, "data"); err != nil {
		return err
	}
	rule := recordRule{required: []string{"snippet", "thread"}}
	for i := 0; i < state.visible; i++ {
		if err := state.rows[i].validate(rule, "search result"); err != nil {
			return err
		}
	}

	*n = ThreadSearchPartialNotification(decoded)
	return nil
}

// Match native array backing reuse, including hidden rows after truncation.
// An explicitly empty array replaces the backing slice and clears old presence.
type searchResultPresence struct {
	rows    []recordPresence
	visible int
}

func (s *searchResultPresence) observe(raw []byte) error {
	count := 0
	err := validateArrayElements(raw, func(i int, element []byte) error {
		count = i + 1
		if isNullJSONValue(element) {
			return ErrNullResultField
		}
		if len(s.rows) <= i {
			s.rows = append(s.rows, make([]recordPresence, i+1-len(s.rows))...)
		}
		return s.rows[i].observe(element, recordRule{required: []string{"snippet", "thread"}}, "search result")
	})
	if err != nil {
		return err
	}
	if count == 0 {
		s.rows = nil
	}
	s.visible = count
	return nil
}
