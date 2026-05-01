### Realtime audio notification test accepts a non-base64 data string

**Line:** `623`

**Reason:** The production behavior is intentionally schema-aligned: `ThreadRealtimeAudioChunk.data` is an unconstrained string in the protocol source of truth, not a schema-defined base64 field. The listener dispatch test verifies that arbitrary string payloads accepted by the protocol are delivered to handlers.

Changing this fixture to prove base64 rejection would narrow behavior beyond the current protocol contract.
