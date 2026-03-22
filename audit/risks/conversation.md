### Conversation thread history grows unboundedly

**Location:** `229-233`

**Reason:** Adding a cap or compaction strategy changes the observable behavior of `Conversation` —
callers may depend on accessing the full turn history. A max-turns option would add a configuration
knob for a scenario that is unlikely in practice (the SDK targets ephemeral single-turn or
short multi-turn interactions, not long-lived chat sessions). Memory growth is linear in completed
turns, and each turn's items are small Go structs. For the expected usage patterns, this is not
a practical concern.
