package protocol

// ThreadSectionChangedNotification reports a thread's destination section.
// SectionID is nil when the thread was removed from its section.
type ThreadSectionChangedNotification struct {
	ThreadID  string  `json:"threadId"`
	SectionID *string `json:"sectionId"`
}

// OnThreadSectionChanged sets the handler for thread/section/changed.
func (c *Client) OnThreadSectionChanged(h func(ThreadSectionChangedNotification)) {
	setTypedNotificationHandler(c, NotifyThreadSectionChanged, h)
}

// AddThreadSectionChangedListener adds a listener for thread/section/changed.
func (c *Client) AddThreadSectionChangedListener(h func(ThreadSectionChangedNotification)) func() {
	return addTypedNotificationListener(c, NotifyThreadSectionChanged, h)
}

func (n *ThreadSectionChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadSectionChangedNotification
	var decoded wire
	// A destination must be present, but null explicitly removes the section.
	if err := unmarshalInboundObject(data, &decoded, []string{"threadId", "sectionId"}, []string{"threadId"}); err != nil {
		return err
	}
	*n = ThreadSectionChangedNotification(decoded)
	return nil
}
