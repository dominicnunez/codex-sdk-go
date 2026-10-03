# Reading workspace login restrictions

`Client.Config.Read` returns the effective app-server configuration. The
`forced_chatgpt_workspace_id` property accepts a string or an array of strings.
The SDK retains the selected branch:

- `Config.ForcedChatgptWorkspaceID` is the existing `*string` field for a single
  string restriction.
- `Config.ForcedChatgptWorkspaceIDs` is a `*[]string` for the array branch. A
  nonnil pointer to an empty slice retains an explicit `[]`. One-element arrays
  remain arrays; the SDK does not select their first ID as a single restriction.
  A nonnil pointer to a nil slice also selects the array branch and marshals as
  `[]`, without changing the caller's slice.
- Null and absence produce nil pointers on a fresh Config and are omitted when
  marshaled. Decoding an omitted field into a reused Config retains its previous
  value; explicit null clears both branches.

Use whichever branch is populated when displaying the returned restrictions.
To construct configuration, set one branch. Setting both makes marshaling fail
instead of silently choosing a narrower restriction. IDs are opaque strings;
empty strings, repeated IDs, and empty lists are permitted by the schema. The
server and embedding application define their meaning and enforce membership.
The independent `login` package does not consume these app-server restrictions.

Config implements JSON marshaling and unmarshaling to preserve both forms while
retaining the existing single-ID field. Use a named Config member in application
JSON envelopes:

```go
type Envelope struct {
    Config protocol.Config `json:"config"`
    Label  string          `json:"label"`
}
```

Anonymous embedding promotes Config's JSON methods; an embedding envelope needs
its own JSON implementation to include its extra fields. A separately defined
type such as `type LocalConfig protocol.Config` does not inherit Config's methods
and must implement its own handling to support the array branch. A type alias
such as `type LocalConfig = protocol.Config` retains the methods.

Invalid restrictions reject decoding, including null or nonstring array entries.
A later valid duplicate does not conceal an earlier invalid restriction. As with
other schema custom JSON decoders, a malformed restriction stops at its occurrence:
later Config fields are not decoded, and its error can supersede an earlier saved
type error in a different Config field. This differs from the old string-only
decoder's error accumulation for invalid scalar restrictions. A Config error can
also stop an enclosing object's decode before later outer fields are updated;
custom error offsets can be relative to the rejected field or Config value.
Unrelated direct Config field type errors retain their standard error context
and receiver updates when the restriction is valid or absent.
Treat an error as failed admission rather than consuming a
partially decoded configuration. The public `Config.Read` returns no successful
typed configuration on these errors.
