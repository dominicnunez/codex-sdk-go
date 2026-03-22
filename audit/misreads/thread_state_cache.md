### Server-driven thread closure leaves conversations usable after the server closes the thread

**Location:** `261`

**Reason:** This does not match the current code. `installThreadStateCache` registers a
`notifyThreadClosed` listener, `closeThreadState` marks the cached thread closed and notifies
thread-state listeners, and `StartConversation` registers `state.close` as the close callback for
that thread. After a `thread/closed` notification, both `Conversation.Turn` and
`Conversation.TurnStreamed` fail locally with `conversation is closed` instead of sending another
`turn/start`.
