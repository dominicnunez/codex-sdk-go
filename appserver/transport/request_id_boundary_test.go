package transport

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestPendingRequestIDNumericBoundaries(t *testing.T) {
	upper := math.Ldexp(1, 63)
	for _, tc := range []struct {
		value any
		want  string
	}{
		{upper, ""}, {float64(math.MaxInt64), ""}, {math.Nextafter(upper, math.Inf(1)), ""},
		{math.Nextafter(upper, 0), "n:9223372036854774784"},
		{-upper, "n:-9223372036854775808"},
		{math.Nextafter(-upper, math.Inf(-1)), ""},
		{math.Nextafter(-upper, 0), "n:-9223372036854774784"},
		{math.Copysign(0, -1), "n:0"}, {int64(math.MinInt64), "n:-9223372036854775808"},
		{math.NaN(), ""}, {math.Inf(1), ""}, {math.Inf(-1), ""}, {1.5, ""},
		{int64(math.MaxInt64), "n:9223372036854775807"},
		{uint64(math.MaxInt64), "n:9223372036854775807"}, {uint64(math.MaxInt64) + 1, ""},
		{json.Number("9223372036854775807"), "n:9223372036854775807"},
		{json.Number("-9223372036854775808"), "n:-9223372036854775808"},
		{json.Number("9223372036854775808"), ""}, {json.Number("-9223372036854775809"), ""},
		{"9223372036854775807", "s:9223372036854775807"},
	} {
		got, err := normalizePendingRequestID(tc.value)
		if tc.want == "" {
			if !errors.Is(err, errUnexpectedIDType) {
				t.Fatalf("invalid %v key=%s err=%v", tc.value, got, err)
			}
		} else if err != nil || got != tc.want {
			t.Fatalf("value=%v key=%s err=%v want=%s", tc.value, got, err, tc.want)
		}
	}
}

func TestInvalidFloatIDCannotAcquirePendingRequest(t *testing.T) {
	tr := outcomeTransport()
	defer tr.cancelCtx()
	key := "n:-9223372036854775808"
	valid := pendingReq{id: RequestID{Value: int64(math.MinInt64)}, ch: make(chan pendingReqResult, 1)}
	tr.pendingReqs[key] = valid
	invalid := RequestID{Value: math.Ldexp(1, 63)}
	tr.handleResponse(Response{ID: invalid, Result: json.RawMessage(`false`)})
	tr.failPendingIDWithError(invalid, ErrCodeInvalidRequest, "invalid correlation")
	if len(tr.pendingReqs) != 1 || tr.pendingReqs[key].ch != valid.ch {
		t.Fatal("invalid response or recovery ID consumed the valid pending owner")
	}
	select {
	case <-valid.ch:
		t.Fatal("invalid response or recovery ID published a result")
	default:
	}
	_, err := tr.Send(context.Background(), Request{ID: invalid, Method: "test"})
	if !errors.Is(err, errUnexpectedIDType) || len(tr.pendingReqs) != 1 || tr.pendingReqs[key].ch != valid.ch || len(tr.writeQueue) != 0 {
		t.Fatalf("invalid admission touched valid request: err=%v pending=%d queued=%d", err, len(tr.pendingReqs), len(tr.writeQueue))
	}
	tr.handleResponse(Response{ID: valid.id, Result: json.RawMessage(`true`)})
	select {
	case result := <-valid.ch:
		if result.err != nil || string(result.resp.Result) != "true" {
			t.Fatalf("valid pending outcome=%+v", result)
		}
	default:
		t.Fatal("valid minimum ID did not retain its correlation")
	}
}
