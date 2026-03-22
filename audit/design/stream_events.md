### newCollabEvent copies AgentsStates map on every collab event

**Location:** `99-116`

**Reason:** The defensive copy ensures event isolation — consumers cannot mutate the
internal state by modifying an emitted event's map or slice. The copy allocates on every
call, but collab events are infrequent (twice per item lifecycle) and the maps are small
(one entry per concurrent agent). The GC pressure is negligible for realistic session sizes.
Removing the copy would require a documented no-mutation contract that callers cannot
violate at compile time, trading correctness for a speculative performance gain. The
current approach is correct-by-construction and consistent with how Thread() and other
snapshot methods work in the codebase.
