### Turn backlog coverage for Run, Conversation.Turn, and RunStreamed already exists

**Location:** `803`

**Reason:** The report says the suite only checks transport liveness under queue pressure and does
not drive real turn lifecycles. The current test suite already includes
`TestRunCompletesWithAllItemsUnderTurnNotificationBacklog`,
`TestConversationTurnCompletesWithAllItemsUnderTurnNotificationBacklog`, and
`TestRunStreamedCompletesWithAllItemsUnderTurnNotificationBacklog`, all backed by the real stdio
transport via `serveBurstLifecycleOverStdio`. Those tests exercise the user-facing turn flows
under heavy same-thread completion backlog and assert that items and turn completion are preserved.

### Turn backlog coverage for Run, Conversation.Turn, and RunStreamed already exists

**Location:** `887`

**Reason:** The report says the suite only checks transport liveness under queue pressure and does
not drive real turn lifecycles. The current test suite already includes
`TestRunCompletesWithAllItemsUnderTurnNotificationBacklog`,
`TestConversationTurnCompletesWithAllItemsUnderTurnNotificationBacklog`, and
`TestRunStreamedCompletesWithAllItemsUnderTurnNotificationBacklog`, all backed by the real stdio
transport via `serveBurstLifecycleOverStdio`. Those tests exercise the user-facing turn flows
under heavy same-thread completion backlog and assert that items and turn completion are preserved.

### Turn backlog coverage for Run, Conversation.Turn, and RunStreamed already exists

**Location:** `976`

**Reason:** The report says the suite only checks transport liveness under queue pressure and does
not drive real turn lifecycles. The current test suite already includes
`TestRunCompletesWithAllItemsUnderTurnNotificationBacklog`,
`TestConversationTurnCompletesWithAllItemsUnderTurnNotificationBacklog`, and
`TestRunStreamedCompletesWithAllItemsUnderTurnNotificationBacklog`, all backed by the real stdio
transport via `serveBurstLifecycleOverStdio`. Those tests exercise the user-facing turn flows
under heavy same-thread completion backlog and assert that items and turn completion are preserved.
