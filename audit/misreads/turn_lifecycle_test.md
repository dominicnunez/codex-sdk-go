# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Turn backlog coverage for Run, Conversation.Turn, and RunStreamed already exists

**Location:** `turn_lifecycle_test.go:803`, `turn_lifecycle_test.go:887`, `turn_lifecycle_test.go:976`

**Reason:** The report says the suite only checks transport liveness under queue pressure and does
not drive real turn lifecycles. The current test suite already includes
`TestRunCompletesWithAllItemsUnderTurnNotificationBacklog`,
`TestConversationTurnCompletesWithAllItemsUnderTurnNotificationBacklog`, and
`TestRunStreamedCompletesWithAllItemsUnderTurnNotificationBacklog`, all backed by the real stdio
transport via `serveBurstLifecycleOverStdio`. Those tests exercise the user-facing turn flows
under heavy same-thread completion backlog and assert that items and turn completion are preserved.
