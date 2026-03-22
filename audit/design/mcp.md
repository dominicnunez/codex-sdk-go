### AuthorizationUrl field uses spec casing instead of Go acronym convention

**Location:** `83`

**Reason:** The spec schema (`McpServerOauthLoginResponse.json`) defines the wire field as
`"authorizationUrl"`. The Go field name `AuthorizationUrl` mirrors the spec. Renaming to
`AuthorizationURL` would be more idiomatic Go, but the project's spec compliance rules
prohibit renaming public fields that map to spec schemas. The JSON struct tag preserves
wire compatibility regardless.
