package protocol

const notifyAuthRecoveryStarted = "modelProvider/authRecoveryStarted"

// OnAuthRecoveryStarted sets the handler for modelProvider/authRecoveryStarted.
func (c *Client) OnAuthRecoveryStarted(h func(AuthRecoveryNotification)) {
	setTypedNotificationHandler(c, notifyAuthRecoveryStarted, h)
}

// AddAuthRecoveryStartedListener adds a listener for modelProvider/authRecoveryStarted.
func (c *Client) AddAuthRecoveryStartedListener(h func(AuthRecoveryNotification)) func() {
	return addTypedNotificationListener(c, notifyAuthRecoveryStarted, h)
}

const notifyAuthRecoveryCompleted = "modelProvider/authRecoveryCompleted"

// OnAuthRecoveryCompleted sets the handler for modelProvider/authRecoveryCompleted.
func (c *Client) OnAuthRecoveryCompleted(h func(AuthRecoveryNotification)) {
	setTypedNotificationHandler(c, notifyAuthRecoveryCompleted, h)
}

// AddAuthRecoveryCompletedListener adds a listener for modelProvider/authRecoveryCompleted.
func (c *Client) AddAuthRecoveryCompletedListener(h func(AuthRecoveryNotification)) func() {
	return addTypedNotificationListener(c, notifyAuthRecoveryCompleted, h)
}

const notifyMcpServerEventStream = "mcpServer/event/stream/notification"

// OnMcpServerEventStream sets the handler for mcpServer/event/stream/notification.
func (c *Client) OnMcpServerEventStream(h func(McpServerEventStreamNotification)) {
	setTypedNotificationHandler(c, notifyMcpServerEventStream, h)
}

// AddMcpServerEventStreamListener adds a listener for mcpServer/event/stream/notification.
func (c *Client) AddMcpServerEventStreamListener(h func(McpServerEventStreamNotification)) func() {
	return addTypedNotificationListener(c, notifyMcpServerEventStream, h)
}

const notifyThreadRealtimeItemStarted = "thread/realtime/item/started"

// OnThreadRealtimeItemStarted sets the handler for thread/realtime/item/started.
func (c *Client) OnThreadRealtimeItemStarted(h func(ThreadRealtimeItemStartedNotification)) {
	setTypedNotificationHandler(c, notifyThreadRealtimeItemStarted, h)
}

// AddThreadRealtimeItemStartedListener adds a listener for thread/realtime/item/started.
func (c *Client) AddThreadRealtimeItemStartedListener(h func(ThreadRealtimeItemStartedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadRealtimeItemStarted, h)
}

const notifyThreadRealtimeItemCompleted = "thread/realtime/item/completed"

// OnThreadRealtimeItemCompleted sets the handler for thread/realtime/item/completed.
func (c *Client) OnThreadRealtimeItemCompleted(h func(ThreadRealtimeItemCompletedNotification)) {
	setTypedNotificationHandler(c, notifyThreadRealtimeItemCompleted, h)
}

// AddThreadRealtimeItemCompletedListener adds a listener for thread/realtime/item/completed.
func (c *Client) AddThreadRealtimeItemCompletedListener(h func(ThreadRealtimeItemCompletedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadRealtimeItemCompleted, h)
}

const notifyThreadRealtimeItemTranscriptDelta = "thread/realtime/item/transcript/delta"

// OnThreadRealtimeItemTranscriptDelta sets the handler for thread/realtime/item/transcript/delta.
func (c *Client) OnThreadRealtimeItemTranscriptDelta(h func(ThreadRealtimeItemTranscriptDeltaNotification)) {
	setTypedNotificationHandler(c, notifyThreadRealtimeItemTranscriptDelta, h)
}

// AddThreadRealtimeItemTranscriptDeltaListener adds a listener for thread/realtime/item/transcript/delta.
func (c *Client) AddThreadRealtimeItemTranscriptDeltaListener(h func(ThreadRealtimeItemTranscriptDeltaNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadRealtimeItemTranscriptDelta, h)
}
