package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func denseIdentityObject(fields int) []byte {
	var b bytes.Buffer
	b.WriteString(`{"threadId":"a"`)
	for i := range fields {
		fmt.Fprintf(&b, `,"k%d":"ignored"`, i)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func BenchmarkStdioDenseIdentityDispatch(b *testing.B) {
	for _, fields := range []int{10, 50000, 500000} {
		for _, mode := range []string{"owner", "duplicate IDs"} {
			var line []byte
			if mode == "owner" {
				line = append(append([]byte(`{"method":"thread/closed","params":`), denseIdentityObject(fields)...), []byte("}\n")...)
			} else {
				line = []byte(`{"method":"audit/request",` + strings.Repeat(`"id":"",`, fields) + "\"id\":7}\n")
			}
			b.Run(fmt.Sprintf("%s/%d", mode, fields), func(b *testing.B) {
				r, w := io.Pipe()
				tr := NewStdioTransport(r, io.Discard)
				b.Cleanup(func() { _ = tr.Close(); _ = w.Close() })
				ack := make(chan struct{})
				tr.OnNotify(func(context.Context, Notification) { ack <- struct{}{} })
				tr.OnRequest(func(context.Context, Request) (Response, error) {
					ack <- struct{}{}
					return Response{Result: json.RawMessage(`null`)}, nil
				})
				b.ReportAllocs()
				b.SetBytes(int64(len(line)))
				for b.Loop() {
					if _, err := w.Write(line); err != nil {
						b.Fatal(err)
					}
					<-ack
				}
			})
		}
	}
}

func FuzzInboundFrameSelection(f *testing.F) {
	for _, seed := range []string{
		`null`, `[]`, `{"id":7,"method":"request","params":{"x":1}}`,
		`{"id":[],"ID":"a","jsonrpc":7,"JSONRPC":"2.0"}`,
		`{"method":1,"method":"request"}`, `{"method":"request","Method":null}`,
		`{"jſonrpc":"2.0","\u0049D":7,"error":null}`, `{"id":7,"id":null}`,
		`{"error":{"code":1,"message":"x"},"error":null}`, `{"params":{},"params":null}`,
		`{"id":7,"id":[]}`, `{"jsonrpc":"2.0","jsonrpc":[]}`, `{"error":{"code":1},"error":[]}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		var reference inboundFrame
		refErr := json.Unmarshal(data, &reference)
		got, err := decodeInboundFrame(data)
		// Invalid wrapper values are inaccessible to routing. The reference
		// may retain an earlier value while setting the invalid flag; selecting
		// once deliberately avoids materializing that unreachable value.
		if reference.ID.invalid {
			reference.ID.value = RequestID{}
		}
		if reference.JSONRPC.invalid {
			reference.JSONRPC.value = ""
		}
		if reference.Error.invalid {
			reference.Error.value = nil
		}
		if (err != nil) != (refErr != nil) || (err == nil && !reflect.DeepEqual(got, reference)) {
			t.Fatalf("frame differs: got=%+v/%v want=%+v/%v data=%s", got, err, reference, refErr, data)
		}
	})
}

func FuzzInboundRPCError(f *testing.F) {
	for _, seed := range []string{
		`{"code":1,"message":"x"}`, `{"code":{},"code":1}`, `{"message":1,"message":"x"}`,
		`{"code":1,"CODE":null,"message":"x","Message":null}`, `{"code":1e2}`,
		`{"code":9223372036854775808}`, `{"data":{"x":1},"Data":null}`, `{"meſſage":"x"}`,
		`null`, `[]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 65536 {
			t.Skip()
		}
		// Generate a valid object around arbitrary code/message shapes too;
		// rejected input bytes become a JSON string instead of a vacuous skip.
		if !json.Valid(input) {
			input, _ = json.Marshal(string(input))
		}
		for _, data := range [][]byte{input, append(append([]byte(`{"code":`), input...), []byte(`,"code":7,"message":"x","data":null}`)...)} {
			var reference Error
			err := json.Unmarshal(data, &reference)
			got, ok := decodeInboundRPCError(data)
			if ok != (err == nil) || (ok && !reflect.DeepEqual(got, reference)) {
				t.Fatalf("RPC error differs: got=%+v/%v want=%+v/%v data=%s", got, ok, reference, err, data)
			}
		}
	})
}

func referenceRecoverID(data []byte) (RequestID, bool, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var id RequestID
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return id, false, false
	}
	var hasID, hasMethod bool
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			break
		}
		value, err := dec.Token()
		if err != nil {
			break
		}
		if key == "id" {
			switch v := value.(type) {
			case string:
				id, hasID = RequestID{Value: v}, true
			case json.Number:
				id, hasID = RequestID{Value: v}, true
			}
		}
		if key == "method" {
			if _, ok := value.(string); ok {
				hasMethod = true
			}
		}
		if d, ok := value.(json.Delim); ok && (d == '{' || d == '[') {
			depth := 1
			for depth > 0 {
				token, tokenErr := dec.Token()
				if tokenErr != nil {
					return id, hasID, hasMethod
				}
				if delimiter, ok := token.(json.Delim); ok {
					if delimiter == '{' || delimiter == '[' {
						depth++
						// Independently model the documented bounded recovery policy.
						if depth >= 10000 {
							return id, hasID, hasMethod
						}
					} else {
						depth--
					}
				}
			}
		}
	}
	return id, hasID, hasMethod
}

func FuzzMalformedIdentityRecovery(f *testing.F) {
	for _, seed := range []string{
		`{"id":7,"result":`, `{"id":7x`, `{"id":7-8`, `{"id":7,"id":null,"broken":`,
		`{"id":7,"id":{},"method":"x"}`, `{"id":"a","extra":{"x":[1,true]},"result":`,
		`{"id":7,"method":"x","extra":[`, `{"id":7,"extra":[[[`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		want, wantID, wantMethod := referenceRecoverID(data)
		got, hasID, hasMethod := extractTopLevelIDAndMethod(data)
		if !reflect.DeepEqual(got, want) || hasID != wantID || hasMethod != wantMethod {
			t.Fatalf("recovery differs: got=%+v/%v/%v want=%+v/%v/%v data=%s", got, hasID, hasMethod, want, wantID, wantMethod, data)
		}
	})
}

func TestTransportIdentityExtractionAllocations(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("quantitative allocation checks use a standard build: race instrumentation randomly discards encoding/json's sync.Pool scanners")
	}
	dense := denseIdentityObject(50000)
	escaped := bytes.ReplaceAll(dense, []byte(`"k`), []byte(`"\u006b`))
	nested := append(append([]byte(`{"thread":{"id":"a",`), dense[1:]...), '}')
	invalidRequest := append([]byte(`{"method":true,"id":7,`), dense[1:]...)
	compositeID := append(append([]byte(`{"method":"audit/request","id":`), dense...), '}')
	malformed := append(append([]byte(`{"id":7,"extra":`), dense...), []byte(`,"broken":`)...)
	oversized := append(append([]byte(`{"extra":`), dense...), []byte(`,"id":7,"result":`)...)
	duplicates := []byte(`{"method":"audit/request",` + string(bytes.Repeat([]byte(`"id":"",`), 50000)) + `"id":7}`)
	badNested := []byte(`{"thread":{` + string(bytes.Repeat([]byte(`"ID":{},`), 50000)) + `"id":"a"}}`)
	badError := []byte(`{"id":7,"error":{` + strings.Repeat(`"code":{},`, 50000) + `"code":1,"message":"failed"}}`)
	recoveryIDs := []byte(`{` + strings.Repeat(`"id":"",`, 50000) + `"id":7,"broken":`)
	oversizedIDs := []byte(`{` + strings.Repeat(`"id":7,`, 50000) + `"result":`)
	malformedResponse := append([]byte(`{"id":7,"result":null,"error":{},`), escaped[1:]...)
	tr := &StdioTransport{}
	for _, tc := range []struct {
		name string
		run  func()
	}{
		{"owner", func() { _ = orderedNotificationKey(Notification{Method: "thread/closed", Params: dense}) }},
		{"escaped metadata", func() { _ = orderedNotificationKey(Notification{Method: "thread/closed", Params: escaped}) }},
		{"malformed response", func() { tr.processInboundLine(malformedResponse) }},
		{"nested owner", func() { _ = orderedNotificationKey(Notification{Method: "thread/started", Params: nested}) }},
		{"invalid request", func() { _, _, _ = extractInboundRequestObjectID(invalidRequest) }},
		{"composite ID", func() { _, _ = decodeInboundFrame(compositeID) }},
		{"duplicate IDs", func() { _, _ = decodeInboundFrame(duplicates) }},
		{"invalid nested duplicates", func() { _ = orderedNotificationKey(Notification{Method: "thread/started", Params: badNested}) }},
		{"invalid error duplicates", func() { _, _ = decodeInboundFrame(badError) }},
		{"duplicate recovery IDs", func() { _, _, _ = extractTopLevelIDAndMethod(recoveryIDs) }},
		{"duplicate oversized IDs", func() { _ = inspectOversizedFramePrefix(oversizedIDs) }},
		{"malformed recovery", func() { _, _, _ = extractTopLevelIDAndMethod(malformed) }},
		{"oversized recovery", func() { _ = inspectOversizedFramePrefix(oversized) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Fixed-size identities should not allocate per ignored property.
			// The envelope may still copy raw payload bytes once.
			if allocations := testing.AllocsPerRun(1, tc.run); allocations > 100 {
				t.Fatalf("identity extraction allocated %.0f times for discarded metadata", allocations)
			}
		})
	}
}

func TestTransportIdentityDepthAndLength(t *testing.T) {
	for _, depth := range []int{100, 9999, 10000, 10001} {
		value := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
		params := []byte(`{"threadId":"a","extra":` + value + `}`)
		want := "thread:a"
		if depth >= 10000 {
			want = ""
		}
		if got := orderedNotificationKey(Notification{Method: "thread/closed", Params: params}); got != want {
			t.Fatalf("depth=%d owner=%q want=%q", depth, got, want)
		}
		prefix := []byte(`{"id":7,"extra":` + value + `,"broken":`)
		id, hasID, hasMethod := extractTopLevelIDAndMethod(prefix)
		if !hasID || hasMethod || !id.Equal(RequestID{Value: int64(7)}) {
			t.Fatalf("depth=%d lost earlier correlation: %v/%v/%v", depth, id, hasID, hasMethod)
		}
		later := []byte(`{"extra":` + value + `,"id":7,"broken":`)
		_, laterID, _ := extractTopLevelIDAndMethod(later)
		if laterID != (depth < 10000) {
			t.Fatalf("depth=%d later correlation=%v", depth, laterID)
		}
		info := inspectOversizedFramePrefix([]byte(`{"id":7,"result":` + strings.Repeat("[", depth)))
		if !info.hasID || !info.hasResponseFields || info.hasMethod {
			t.Fatalf("depth=%d oversized correlation=%+v", depth, info)
		}
	}
	owner := strings.Repeat("x", 1024*1024)
	if key := orderedNotificationKey(Notification{Method: "thread/closed", Params: json.RawMessage(`{"threadId":"` + owner + `"}`)}); key != "thread:"+owner {
		t.Fatal("valid long owner changed")
	}
}

func TestOversizedIdentityScalarBoundaries(t *testing.T) {
	for _, suffix := range []string{"7x", "7-8", "7true", "7e", "7.", "nullx"} {
		info := inspectOversizedFramePrefix([]byte(`{"result":null,"id":` + suffix))
		if info.hasID || info.id.Value != nil || !info.hasResponseFields {
			t.Fatalf("invalid scalar %q correlated request: %+v", suffix, info)
		}
	}
	for _, suffix := range []string{"7}", "7,", "7 ", "7"} {
		info := inspectOversizedFramePrefix([]byte(`{"result":null,"id":` + suffix))
		if !info.hasID || !info.id.Equal(RequestID{Value: int64(7)}) {
			t.Fatalf("valid scalar %q lost correlation: %+v", suffix, info)
		}
	}
}

func BenchmarkTransportIdentityExtraction(b *testing.B) {
	for _, fields := range []int{10, 50000, 500000} {
		data := denseIdentityObject(fields)
		b.Run(fmt.Sprintf("owner/%d", fields), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				_ = orderedNotificationKey(Notification{Method: "thread/closed", Params: data})
			}
		})
	}
}

// The allocating standard decoder is an independent semantic reference.
func FuzzNotificationOwnerFields(f *testing.F) {
	for _, seed := range []string{
		`{"threadId":"a"}`, `{"threadId":null}`, `null`, `[]`,
		`{"ThreadID":"other","thread\u0049d":"a"}`,
		`{"threadId":1,"threadId":"a"}`, `{"threadId":"a","threadId":1}`,
		`{"thread":{"id":"a","ID":"b"}}`, `{"thread":{"id":1,"ID":"a"}}`,
		`{"thread":{"id":"a"},"thread":null}`, `{"threadId":""}`, `{"threadId":"a",`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		for _, method := range []string{"thread/closed", "thread/started", "warning"} {
			var fields map[string]json.RawMessage
			want := ""
			if json.Unmarshal(data, &fields) == nil {
				var owner *string
				var err error
				if method == "thread/started" {
					var carrier struct {
						ID *string `json:"id"`
					}
					err = json.Unmarshal(fields["thread"], &carrier)
					owner = carrier.ID
				} else if raw, exists := fields["threadId"]; exists {
					err = json.Unmarshal(raw, &owner)
				}
				if err == nil {
					if owner != nil {
						want = "thread:" + *owner
					} else if method == "warning" {
						want = "method:warning"
					}
				}
			}
			if got := orderedNotificationKey(Notification{Method: method, Params: data}); got != want {
				t.Fatalf("method=%s owner=%q want=%q params=%s", method, got, want, data)
			}
		}
	})
}
