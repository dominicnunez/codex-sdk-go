package codex

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func canonicalRequestIDValue(value interface{}, allowNull bool) (interface{}, error) {
	switch v := value.(type) {
	case nil:
		if allowNull {
			return nil, nil //nolint:nilnil // nil is a valid JSON-RPC null ID when allowed.
		}
		return nil, errNullID
	case string:
		return v, nil
	}

	intID, isNumeric, err := canonicalInt64RequestID(value)
	if err != nil {
		return nil, err
	}
	if !isNumeric {
		return nil, fmt.Errorf("%w: %T", errUnexpectedIDType, value)
	}
	return intID, nil
}

func canonicalNumericRequestIDString(value interface{}) (string, bool, error) {
	intID, isNumeric, err := canonicalInt64RequestID(value)
	if err != nil {
		return "", isNumeric, err
	}
	if !isNumeric {
		return "", false, nil
	}
	return strconv.FormatInt(intID, 10), true, nil
}

func canonicalInt64RequestID(value interface{}) (int64, bool, error) {
	switch v := value.(type) {
	case json.Number:
		intID, err := parseJSONRequestID(v.String())
		if err != nil {
			return 0, true, err
		}
		return intID, true, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v || v < math.MinInt64 || v > math.MaxInt64 {
			return 0, true, fmt.Errorf("%w: %v", errUnexpectedIDType, v)
		}
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case int8:
		return int64(v), true, nil
	case int16:
		return int64(v), true, nil
	case int32:
		return int64(v), true, nil
	case int64:
		return v, true, nil
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, true, fmt.Errorf("%w: %v", errUnexpectedIDType, v)
		}
		return int64(v), true, nil
	case uint8:
		return int64(v), true, nil
	case uint16:
		return int64(v), true, nil
	case uint32:
		return int64(v), true, nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, true, fmt.Errorf("%w: %v", errUnexpectedIDType, v)
		}
		return int64(v), true, nil
	default:
		return 0, false, nil
	}
}

func parseJSONRequestID(raw string) (int64, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return 0, fmt.Errorf("%w: %q", errUnexpectedIDType, raw)
	}

	intID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errUnexpectedIDType, raw)
	}
	return intID, nil
}
