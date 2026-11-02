package protocol

// validateNativeNonNullFields follows native folded field matching after wire
// decoding has established kinds and error context. Presence is checked by the
// owning record separately; no recognized null occurrence can hide in duplicates.
func validateNativeNonNullFields(data []byte, names ...string) error {
	return validateOptionalFields(data, func(name string, raw []byte) error {
		if isNullJSONValue(raw) {
			return responseObjectValidationErrors().null(name)
		}
		return nil
	}, names...)
}
