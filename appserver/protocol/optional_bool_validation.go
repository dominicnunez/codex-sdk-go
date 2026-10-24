package protocol

// Native wire decoding owns JSON kinds and error context. After it succeeds,
// reject null occurrences of non-null optional booleans using the same folded
// field matching, including escaped and Unicode aliases. Absence is valid.
func validateOptionalBooleans(data []byte, names ...string) error {
	return validateOptionalFields(data, func(name string, raw []byte) error {
		if isNullJSONValue(raw) {
			return responseObjectValidationErrors().null(name)
		}
		return nil
	}, names...)
}
