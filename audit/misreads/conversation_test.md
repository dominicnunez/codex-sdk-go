### Conversation tests never cover server-side thread closure

**Location:** `292`

**Reason:** The reported coverage gap is stale. `TestConversationRejectsTurnsAfterThreadClosedNotification`
starts a conversation, injects `thread/closed`, asserts `Turn` returns `conversation is closed`,
asserts `TurnStreamed` yields the same error, and verifies no `turn/start` request is sent after
the closure notification.

### Multi-turn state accumulation claimed to be untested

**Location:** `N/A`

**Reason:** Already in known exceptions. `TestConversationMultiTurn` (conversation_test.go:13-96)
executes two turns on the same Conversation, then asserts `len(thread.Turns) == 2` at line 93-94.
The multi-turn accumulation path is tested.

### Multi-turn state accumulation claimed to be untested

**Location:** `N/A`

**Reason:** Already in known exceptions. `TestConversationMultiTurn` (conversation_test.go:13-96)
executes two turns on the same Conversation, then asserts `len(thread.Turns) == 2` at line 93-94.
The multi-turn accumulation path is tested.
