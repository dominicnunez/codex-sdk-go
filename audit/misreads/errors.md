### RPCError.Is nil-nil path flagged as unreachable dead code

**Location:** `61-70`

**Reason:** This is covered by the existing exception for RPCError.Is which states: "The nil-nil
comparison path is unreachable since `NewRPCError` is never called with nil, but the nil guard is
a defensive correctness check, not dead logic worth removing." Defensive nil checks in `Is()`
implementations are standard Go practice — they prevent panics if the type is ever constructed
outside the canonical constructor.
