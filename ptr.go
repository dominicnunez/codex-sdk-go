package codex

// Ptr returns a pointer to the given value.
// This is useful for constructing optional fields in structs that use pointer types.
//
// Example:
//
//	params := InitializeParams{
//		ClientInfo: ClientInfo{
//			Name:    "my-client",
//			Version: "1.0.0",
//			Title:   Ptr("My Client Title"), // optional field
//		},
//	}
func Ptr[T any](v T) *T {
	return &v
}
