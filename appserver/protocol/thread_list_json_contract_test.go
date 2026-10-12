package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadListJSONAllSchemaPropertiesReachTransport(t *testing.T) {
	params := codex.ThreadListParamsJSON{
		Archived:          codex.OptionalNullable[bool]{Present: true, Value: codex.Ptr(false)},
		Cursor:            codex.OptionalNullable[string]{Present: true, Value: codex.Ptr("")},
		Cwd:               json.RawMessage(`["repo/../work",""]`),
		ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true},
		Limit:             codex.OptionalNullable[uint32]{Present: true, Value: codex.Ptr(uint32(0))},
		ModelProviders:    codex.OptionalNullable[[]string]{Present: true, Value: codex.Ptr([]string{})},
		Originators:       codex.OptionalNullable[[]string]{Present: true, Value: codex.Ptr([]string{"cli"})},
		SearchTerm:        codex.OptionalNullable[string]{Present: true},
		SectionID:         codex.OptionalNullable[string]{Present: true},
		SortDirection:     codex.OptionalNullable[codex.SortDirection]{Present: true, Value: codex.Ptr(codex.SortDirectionAsc)},
		SortKey:           codex.OptionalNullable[codex.ThreadSortKey]{Present: true, Value: codex.Ptr(codex.ThreadSortKeyUpdatedAt)},
		SourceKinds:       codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true, Value: codex.Ptr([]codex.ThreadSourceKind{})},
		UseStateDbOnly:    codex.OptionalNullable[bool]{Present: true, Value: codex.Ptr(false)},
	}
	assertListJSONParams(t, params, `{"archived":false,"cursor":"","cwd":["repo/../work",""],"excludedThreadIds":null,"limit":0,"modelProviders":[],"originators":["cli"],"searchTerm":null,"sectionId":null,"sortDirection":"asc","sortKey":"updated_at","sourceKinds":[],"useStateDbOnly":false}`)
}

func TestThreadListJSONPresenceStates(t *testing.T) {
	str := func(value string) *string { return codex.Ptr(value) }
	boolPtr := func(value bool) *bool { return codex.Ptr(value) }
	boolValue := func(value bool) codex.OptionalNullable[bool] {
		return codex.OptionalNullable[bool]{Present: true, Value: codex.Ptr(value)}
	}
	limit := func(value uint32) *uint32 { return codex.Ptr(value) }
	stringsValue := func(values ...string) *[]string {
		if values == nil {
			values = []string{}
		}
		return codex.Ptr(values)
	}
	tests := []struct {
		name   string
		params codex.ThreadListParamsJSON
		want   string
	}{
		{"all omitted", codex.ThreadListParamsJSON{}, `{}`},
		{"archived null", codex.ThreadListParamsJSON{Archived: codex.OptionalNullable[bool]{Present: true}}, `{"archived":null}`},
		{"archived false", codex.ThreadListParamsJSON{Archived: codex.OptionalNullable[bool]{Present: true, Value: boolPtr(false)}}, `{"archived":false}`},
		{"cursor null", codex.ThreadListParamsJSON{Cursor: codex.OptionalNullable[string]{Present: true}}, `{"cursor":null}`},
		{"cursor empty", codex.ThreadListParamsJSON{Cursor: codex.OptionalNullable[string]{Present: true, Value: str("")}}, `{"cursor":""}`},
		{"cwd null", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`null`)}, `{"cwd":null}`},
		{"cwd empty string", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`""`)}, `{"cwd":""}`},
		{"cwd empty array", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`[]`)}, `{"cwd":[]}`},
		{"cwd scalar unchanged", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`"repo/../work"`)}, `{"cwd":"repo/../work"}`},
		{"excluded null", codex.ThreadListParamsJSON{ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true}}, `{"excludedThreadIds":null}`},
		{"excluded empty", codex.ThreadListParamsJSON{ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true, Value: stringsValue()}}, `{"excludedThreadIds":[]}`},
		{"excluded populated", codex.ThreadListParamsJSON{ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true, Value: stringsValue("thread-1")}}, `{"excludedThreadIds":["thread-1"]}`},
		{"limit null", codex.ThreadListParamsJSON{Limit: codex.OptionalNullable[uint32]{Present: true}}, `{"limit":null}`},
		{"limit zero", codex.ThreadListParamsJSON{Limit: codex.OptionalNullable[uint32]{Present: true, Value: limit(0)}}, `{"limit":0}`},
		{"model providers null", codex.ThreadListParamsJSON{ModelProviders: codex.OptionalNullable[[]string]{Present: true}}, `{"modelProviders":null}`},
		{"model providers empty", codex.ThreadListParamsJSON{ModelProviders: codex.OptionalNullable[[]string]{Present: true, Value: stringsValue()}}, `{"modelProviders":[]}`},
		{"model providers populated", codex.ThreadListParamsJSON{ModelProviders: codex.OptionalNullable[[]string]{Present: true, Value: stringsValue("openai")}}, `{"modelProviders":["openai"]}`},
		{"originators null", codex.ThreadListParamsJSON{Originators: codex.OptionalNullable[[]string]{Present: true}}, `{"originators":null}`},
		{"originators empty", codex.ThreadListParamsJSON{Originators: codex.OptionalNullable[[]string]{Present: true, Value: stringsValue()}}, `{"originators":[]}`},
		{"search term null", codex.ThreadListParamsJSON{SearchTerm: codex.OptionalNullable[string]{Present: true}}, `{"searchTerm":null}`},
		{"search term empty", codex.ThreadListParamsJSON{SearchTerm: codex.OptionalNullable[string]{Present: true, Value: str("")}}, `{"searchTerm":""}`},
		{"section null", codex.ThreadListParamsJSON{SectionID: codex.OptionalNullable[string]{Present: true}}, `{"sectionId":null}`},
		{"section empty string", codex.ThreadListParamsJSON{SectionID: codex.OptionalNullable[string]{Present: true, Value: str("")}}, `{"sectionId":""}`},
		{"sort direction null", codex.ThreadListParamsJSON{SortDirection: codex.OptionalNullable[codex.SortDirection]{Present: true}}, `{"sortDirection":null}`},
		{"sort direction value", codex.ThreadListParamsJSON{SortDirection: codex.OptionalNullable[codex.SortDirection]{Present: true, Value: codex.Ptr(codex.SortDirectionDesc)}}, `{"sortDirection":"desc"}`},
		{"sort key null", codex.ThreadListParamsJSON{SortKey: codex.OptionalNullable[codex.ThreadSortKey]{Present: true}}, `{"sortKey":null}`},
		{"sort key value", codex.ThreadListParamsJSON{SortKey: codex.OptionalNullable[codex.ThreadSortKey]{Present: true, Value: codex.Ptr(codex.ThreadSortKeyCreatedAt)}}, `{"sortKey":"created_at"}`},
		{"source kinds null", codex.ThreadListParamsJSON{SourceKinds: codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true}}, `{"sourceKinds":null}`},
		{"source kinds empty", codex.ThreadListParamsJSON{SourceKinds: codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true, Value: codex.Ptr([]codex.ThreadSourceKind{})}}, `{"sourceKinds":[]}`},
		{"source kinds populated", codex.ThreadListParamsJSON{SourceKinds: codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true, Value: codex.Ptr([]codex.ThreadSourceKind{codex.ThreadSourceKindCLI})}}, `{"sourceKinds":["cli"]}`},
		{"state DB false", codex.ThreadListParamsJSON{UseStateDbOnly: boolValue(false)}, `{"useStateDbOnly":false}`},
		{"state DB true", codex.ThreadListParamsJSON{UseStateDbOnly: boolValue(true)}, `{"useStateDbOnly":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertListJSONParams(t, tt.params, tt.want) })
	}
}

func TestThreadListJSONRejectsInvalidCwdAndEnumsBeforeTransport(t *testing.T) {
	tests := []struct {
		name   string
		params codex.ThreadListParamsJSON
	}{
		{"empty raw JSON", codex.ThreadListParamsJSON{Cwd: json.RawMessage{}}},
		{"malformed JSON", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`{"`)}},
		{"boolean", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`true`)}},
		{"number", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`12`)}},
		{"object", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`{}`)}},
		{"null array member", codex.ThreadListParamsJSON{Cwd: json.RawMessage(`["repo",null]`)}},
		{"invalid sort direction", codex.ThreadListParamsJSON{SortDirection: codex.OptionalNullable[codex.SortDirection]{Present: true, Value: codex.Ptr(codex.SortDirection("sideways"))}}},
		{"invalid sort key", codex.ThreadListParamsJSON{SortKey: codex.OptionalNullable[codex.ThreadSortKey]{Present: true, Value: codex.Ptr(codex.ThreadSortKey("name"))}}},
		{"invalid source kind", codex.ThreadListParamsJSON{SourceKinds: codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true, Value: codex.Ptr([]codex.ThreadSourceKind{codex.ThreadSourceKind("unknown-value")})}}},
		{"null state DB value", codex.ThreadListParamsJSON{UseStateDbOnly: codex.OptionalNullable[bool]{Present: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			defer client.Close()
			_, err := client.Thread.ListJSON(context.Background(), tt.params)
			if err == nil {
				t.Fatal("ListJSON error = nil, want local validation error")
			}
			if transport.CallCount() != 0 {
				t.Fatalf("transport calls = %d, want 0", transport.CallCount())
			}
		})
	}
}

func TestThreadListJSONSharesResponseAndCachePath(t *testing.T) {
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("thread/list", map[string]interface{}{"backwardsCursor": "previous", "nextCursor": "next-page", "data": []interface{}{validThreadPayload("listed")}}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	var observed []string
	remove := client.AddThreadStateListener("listed", func(thread codex.Thread) { observed = append(observed, thread.ID) }, nil)
	defer remove()
	response, err := client.Thread.ListJSON(context.Background(), codex.ThreadListParamsJSON{Cursor: codex.OptionalNullable[string]{Present: true, Value: codex.Ptr("next")}, Limit: codex.OptionalNullable[uint32]{Present: true, Value: codex.Ptr(uint32(0))}})
	if err != nil {
		t.Fatalf("ListJSON() error = %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "listed" || !reflect.DeepEqual(observed, []string{"listed"}) {
		t.Fatalf("response=%v cache listener=%v, want listed thread and publication", response.Data, observed)
	}
	if response.NextCursor == nil || *response.NextCursor != "next-page" || response.BackwardsCursor == nil || *response.BackwardsCursor != "previous" {
		t.Fatalf("pagination response = backwards %v next %v", response.BackwardsCursor, response.NextCursor)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatal(err)
	}
	if string(params["cursor"]) != `"next"` || string(params["limit"]) != `0` {
		t.Fatalf("pagination params = %s, want cursor and explicit zero limit", transport.GetSentRequest(0).Params)
	}
	if _, ok := client.ThreadStateSnapshot("listed"); !ok {
		t.Fatal("ListJSON() did not publish returned thread to cache")
	}
}

func TestThreadListJSONRequestAndResponseFailuresDoNotPublish(t *testing.T) {
	for _, mode := range []string{"transport", "canceled", "rpc", "response"} {
		t.Run(mode, func(t *testing.T) {
			mock := NewMockTransport()
			transport := &threadListRecordingTransport{MockTransport: mock}
			switch mode {
			case "transport":
				transport.sendErr = errors.New("send failed")
			case "canceled":
				transport.sendErr = context.Canceled
			case "rpc":
				mock.SetResponse("thread/list", codex.Response{Error: &codex.Error{Code: -32600, Message: "denied"}})
			case "response":
				if err := mock.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{validThreadPayload("listed"), map[string]interface{}{"id": "broken"}}}); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unknown failure mode %q", mode)
			}
			client := codex.NewClient(transport)
			defer client.Close()
			observed := 0
			remove := client.AddThreadStateListener("listed", func(codex.Thread) { observed++ }, nil)
			defer remove()
			response, err := client.Thread.ListJSON(context.Background(), codex.ThreadListParamsJSON{})
			if err == nil {
				t.Fatal("ListJSON() error = nil, want failure")
			}
			if mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled ListJSON error = %v, want context.Canceled identity", err)
			}
			if mode == "transport" {
				var typed *codex.TransportError
				if !errors.As(err, &typed) {
					t.Fatalf("transport error type = %T, want *TransportError", err)
				}
			}
			if mode == "canceled" {
				var typed *codex.CanceledError
				if !errors.As(err, &typed) {
					t.Fatalf("cancellation error type = %T, want *CanceledError", err)
				}
			}
			if mode == "rpc" {
				var typed *codex.RPCError
				if !errors.As(err, &typed) {
					t.Fatalf("RPC error type = %T, want *RPCError", err)
				}
			}
			if len(transport.sends) != 1 || transport.sends[0].Method != "thread/list" {
				t.Fatalf("Send requests = %v, want one thread/list request", transport.sends)
			}
			if !reflect.DeepEqual(response, codex.ThreadListResponse{}) || observed != 0 {
				t.Fatalf("failed response data=%v listener calls=%d, want none", response.Data, observed)
			}
			if _, ok := client.ThreadStateSnapshot("listed"); ok {
				t.Fatal("failed ListJSON() published a thread")
			}
		})
	}
}

func TestThreadListLegacyRepresentationKeepsEnvelopeBehavior(t *testing.T) {
	type envelope struct {
		codex.ThreadListParams
		Marker string `json:"marker"`
	}
	encoded, err := json.Marshal(envelope{ThreadListParams: codex.ThreadListParams{ModelProviders: []string{}}, Marker: "outer"})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"modelProviders":[],"marker":"outer"}` {
		t.Fatalf("embedded legacy envelope = %s", encoded)
	}
	if reflect.TypeOf(codex.ThreadListParams{}).NumMethod() != 0 || reflect.TypeOf((*codex.ThreadListParams)(nil)).NumMethod() != 0 {
		t.Fatal("legacy ThreadListParams gained exported JSON methods")
	}
}

func TestThreadListJSONDecodesThenSendsAllPresenceStates(t *testing.T) {
	const input = `{"archived":false,"cursor":null,"cwd":["repo","/workspace"],"excludedThreadIds":[],"limit":0,"modelProviders":[],"originators":["cli"],"searchTerm":"","sectionId":null,"sortDirection":"desc","sortKey":"created_at","sourceKinds":[],"useStateDbOnly":false}`
	var params codex.ThreadListParamsJSON
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		t.Fatalf("decode request DTO: %v", err)
	}
	assertListJSONParams(t, params, input)
}

func TestThreadListJSONSchemaPropertyInventory(t *testing.T) {
	schema, err := os.ReadFile("schema/json/v2/ThreadListParams.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &document); err != nil {
		t.Fatal(err)
	}
	typeOfParams := reflect.TypeOf(codex.ThreadListParamsJSON{})
	fields := make(map[string]struct{}, typeOfParams.NumField())
	for i := 0; i < typeOfParams.NumField(); i++ {
		name := strings.Split(typeOfParams.Field(i).Tag.Get("json"), ",")[0]
		fields[name] = struct{}{}
	}
	if len(fields) != len(document.Properties) {
		t.Fatalf("DTO field count = %d, schema property count = %d", len(fields), len(document.Properties))
	}
	for name := range document.Properties {
		if _, ok := fields[name]; !ok {
			t.Errorf("schema property %q missing from ThreadListParamsJSON", name)
		}
	}
}

func TestThreadListSelectedNilFilterSlicesEncodeEmptyArrays(t *testing.T) {
	ids := []string(nil)
	providers := []string(nil)
	originators := []string(nil)
	sources := []codex.ThreadSourceKind(nil)
	params := codex.ThreadListParamsJSON{
		ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true, Value: &ids},
		ModelProviders:    codex.OptionalNullable[[]string]{Present: true, Value: &providers},
		Originators:       codex.OptionalNullable[[]string]{Present: true, Value: &originators},
		SourceKinds:       codex.OptionalNullable[[]codex.ThreadSourceKind]{Present: true, Value: &sources},
	}
	assertListJSONParams(t, params, `{"excludedThreadIds":[],"modelProviders":[],"originators":[],"sourceKinds":[]}`)
	if ids != nil || providers != nil || originators != nil || sources != nil {
		t.Fatal("ListJSON() changed caller-owned nil slices")
	}
}

func TestThreadListLegacyExcludedNilSliceEncodesEmptyArray(t *testing.T) {
	ids := []string(nil)
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{}}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	params := codex.ThreadListParams{ExcludedThreadIDs: codex.OptionalNullable[[]string]{Present: true, Value: &ids}}
	if _, err := client.Thread.List(context.Background(), params); err != nil {
		t.Fatalf("Thread.List() error = %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &got); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if string(got["excludedThreadIds"]) != `[]` {
		t.Fatalf("excludedThreadIds = %s, want []", got["excludedThreadIds"])
	}
	if ids != nil {
		t.Fatal("Thread.List() changed caller-owned nil slice")
	}
}

func TestThreadListJSONRejectsDecodedNullUseStateDbOnly(t *testing.T) {
	var params codex.ThreadListParamsJSON
	if err := json.Unmarshal([]byte(`{"useStateDbOnly":null}`), &params); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{}}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	_, err := client.Thread.ListJSON(context.Background(), params)
	if err == nil || !strings.Contains(err.Error(), "invalid params") {
		t.Fatalf("ListJSON() error = %v, want local non-null validation", err)
	}
	if transport.CallCount() != 0 {
		t.Fatalf("transport calls = %d, want 0", transport.CallCount())
	}
}

func TestThreadListJSONRejectsInvalidDecodedCollectionElements(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"excluded null", `{"excludedThreadIds":[null]}`},
		{"excluded number", `{"excludedThreadIds":[1]}`},
		{"providers null", `{"modelProviders":[null]}`},
		{"providers number", `{"modelProviders":[1]}`},
		{"originators null", `{"originators":[null]}`},
		{"originators number", `{"originators":[1]}`},
		{"source kinds null", `{"sourceKinds":[null]}`},
		{"source kinds number", `{"sourceKinds":[1]}`},
		{"source kinds invalid value", `{"sourceKinds":["unknown-value"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var params codex.ThreadListParamsJSON
			if err := json.Unmarshal([]byte(tt.json), &params); err == nil {
				t.Fatalf("decode %s succeeded, want invalid collection element error", tt.json)
			}
		})
	}
}

func assertListJSONParams(t *testing.T, params codex.ThreadListParamsJSON, want string) {
	t.Helper()
	transport := NewMockTransport()
	defer transport.Close()
	if err := transport.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{}}); err != nil {
		t.Fatal(err)
	}
	client := codex.NewClient(transport)
	defer client.Close()
	if _, err := client.Thread.ListJSON(context.Background(), params); err != nil {
		t.Fatalf("ListJSON() error = %v", err)
	}
	var got, expected map[string]interface{}
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &got); err != nil {
		t.Fatalf("decode actual params %s: %v", transport.GetSentRequest(0).Params, err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatalf("decode expected params: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("wire params = %s, want %s", transport.GetSentRequest(0).Params, want)
	}
}

type threadListRecordingTransport struct {
	*MockTransport
	sends   []codex.Request
	sendErr error
}

func (t *threadListRecordingTransport) Send(ctx context.Context, request codex.Request) (codex.Response, error) {
	t.sends = append(t.sends, request)
	if t.sendErr != nil {
		return codex.Response{}, t.sendErr
	}
	return t.MockTransport.Send(ctx, request)
}
