# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### ConfigLayerSource type discriminator strings are hardcoded in each MarshalJSON

**Location:** `config.go:134-218` — seven ConfigLayerSource MarshalJSON methods
**Date:** 2026-02-27

**Reason:** Each variant hardcodes its type string (e.g. `"mdm"`, `"system"`) in an anonymous
struct literal. The audit suggests extracting named constants so marshal and unmarshal reference
the same value. However, the type strings are trivial string literals that appear exactly twice
each (once in MarshalJSON, once in the UnmarshalJSON switch), and the roundtrip is covered by
tests. Introducing constants for seven single-use pairs adds indirection without meaningful
safety gain.
