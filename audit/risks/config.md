### ConfigLayerSource type discriminator strings are hardcoded in each MarshalJSON

**Location:** `134-218`

**Reason:** Each variant hardcodes its type string (e.g. `"mdm"`, `"system"`) in an anonymous
struct literal. The audit suggests extracting named constants so marshal and unmarshal reference
the same value. However, the type strings are trivial string literals that appear exactly twice
each (once in MarshalJSON, once in the UnmarshalJSON switch), and the roundtrip is covered by
tests. Introducing constants for seven single-use pairs adds indirection without meaningful
safety gain.
