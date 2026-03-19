# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Conversation thread history grows unboundedly

**Location:** `conversation.go:229-233` — turn append in addCompletedTurn
**Date:** 2026-03-01

**Reason:** Adding a cap or compaction strategy changes the observable behavior of `Conversation` —
callers may depend on accessing the full turn history. A max-turns option would add a configuration
knob for a scenario that is unlikely in practice (the SDK targets ephemeral single-turn or
short multi-turn interactions, not long-lived chat sessions). Memory growth is linear in completed
turns, and each turn's items are small Go structs. For the expected usage patterns, this is not
a practical concern.
