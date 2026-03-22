### Zero-field union variants skip unmarshal while variants with fields do not

**Location:** `186-204`

**Reason:** The "add" and "delete" PatchChangeKind branches (and notLoaded/idle/systemError
ThreadStatus branches) construct zero-value structs directly without unmarshaling, while
"update" and "active" unmarshal to capture their fields. This asymmetry is intentional:
unmarshaling into a zero-field struct is wasted work that parses the entire JSON payload
only to discard every field. If the spec adds fields to these types, the struct definitions
will gain fields and the unmarshal call must be added — but that's a spec change that
requires code updates regardless. The current code is correct for the current spec and
avoids unnecessary work.

### Zero-field union variants skip unmarshal while variants with fields do not

**Location:** `186-204`

**Reason:** The "add" and "delete" PatchChangeKind branches (and notLoaded/idle/systemError
ThreadStatus branches) construct zero-value structs directly without unmarshaling, while
"update" and "active" unmarshal to capture their fields. This asymmetry is intentional:
unmarshaling into a zero-field struct is wasted work that parses the entire JSON payload
only to discard every field. If the spec adds fields to these types, the struct definitions
will gain fields and the unmarshal call must be added — but that's a spec change that
requires code updates regardless. The current code is correct for the current spec and
avoids unnecessary work.
