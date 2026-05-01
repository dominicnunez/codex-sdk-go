### Realtime audio payloads skip base64 validation

**Line:** `111`

**Reason:** The behavior is real, but the protocol source of truth does not constrain `ThreadRealtimeAudioChunk.data` to base64. The checked-in JSON schemas define `data` as a plain string with no `format`, `contentEncoding`, or base64 description, and the upstream Codex protocol/generated models also expose it as an unconstrained string.

Adding inbound base64 validation here would reject schema-valid realtime audio notifications, so forwarding the string payload is a correct-by-design exception rather than a repo bug.
