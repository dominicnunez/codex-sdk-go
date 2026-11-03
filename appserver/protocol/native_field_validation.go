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

// Inspect canonical fields in every nested object occurrence, without requiring
// earlier partial objects to contain the final object's complete required set.
// Unknown fields and opaque alternatives are not interpreted as nested records.
func validateNestedNonNullFields(data []byte, object string, names ...string) error {
	return validateFields(data, func(_ string, raw []byte) error {
		return validateFields(raw, func(name string, value []byte) error {
			if isNullJSONValue(value) {
				return responseObjectValidationErrors().null(name)
			}
			return nil
		}, false, names...)
	}, false, object)
}
