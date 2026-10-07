package protocol

import (
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

func validateRateLimitBuckets(data []byte) error {
	return validateOptionalFields(data, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		var bucketErr error
		jsonobject.WalkFields(raw, true, func(key, value []byte) {
			if bucketErr == nil && isNullJSONValue(value) {
				bucketErr = fmt.Errorf("bucket %s must not be null", rateLimitBucketKeyDiagnostic(key))
			}
		})
		return bucketErr
	}, "rateLimitsByLimitId")
}

func rateLimitBucketKeyDiagnostic(key []byte) string {
	const previewLimit = 128
	if len(key) <= previewLimit {
		return quotedValueDiagnostic(string(key))
	}
	return fmt.Sprintf("%s... (%d bytes omitted)", quotedValueDiagnostic(string(key[:previewLimit])), len(key)-previewLimit)
}
