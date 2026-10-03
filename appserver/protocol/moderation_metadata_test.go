package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestModerationMetadataAcceptsEveryJSONKind(t *testing.T) {
	for _, metadata := range []string{`null`, `{}`, `{"nested":[true,null,"text"]}`, `[]`, `[1,{"nested":false}]`, `true`, `false`, `1`, `"text"`, `1e400`, `{"number":1e400}`, `9007199254740993`, `{"number":9007199254740993}`} {
		t.Run(metadata, func(t *testing.T) {
			payload := []byte(`{"threadId":"t","turnId":"u","metadata":` + metadata + `}`)
			var notification protocol.TurnModerationMetadataNotification
			if err := json.Unmarshal(payload, &notification); err != nil {
				t.Fatalf("schema-valid metadata rejected: %v", err)
			}
			if notification.ThreadID != "t" || notification.TurnID != "u" {
				t.Fatal("metadata changed notification identity")
			}
			encoded, err := json.Marshal(notification)
			if err != nil {
				t.Fatal(err)
			}
			var members map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &members); err != nil {
				t.Fatal(err)
			}
			var compact json.RawMessage
			if err := json.Unmarshal(members["metadata"], &compact); err != nil {
				t.Fatal(err)
			}
			if string(compact) != metadata {
				t.Fatalf("metadata lost: got %s want %s", compact, metadata)
			}
		})
	}
}

func TestModerationMetadataRecipientOwnership(t *testing.T) {
	mock := NewMockTransport()
	reports := 0
	client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(_ string, _ error) {
		reports++
		panic("reporter failure")
	}))
	defer client.Close()
	removeFirst := client.AddTurnModerationMetadataListener(func(notification protocol.TurnModerationMetadataNotification) {
		notification.Metadata[0] = ' '
		panic("observer failure")
	})
	defer removeFirst()
	var retained []protocol.TurnModerationMetadataNotification
	removeSecond := client.AddTurnModerationMetadataListener(func(notification protocol.TurnModerationMetadataNotification) {
		retained = append(retained, notification)
	})
	defer removeSecond()
	client.OnTurnModerationMetadata(func(notification protocol.TurnModerationMetadataNotification) {
		retained = append(retained, notification)
	})
	input := json.RawMessage(`{"threadId":"t","turnId":"u","metadata":[{"number":9007199254740993}]}`)
	before := append([]byte(nil), input...)
	mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "turn/moderationMetadata", Params: input})
	if reports != 1 || len(retained) != 2 || !bytes.Equal(input, before) {
		t.Fatalf("callback isolation failed: reports=%d retained=%d", reports, len(retained))
	}
	for i := range input {
		input[i] = ' '
	}
	const want = `[{"number":9007199254740993}]`
	for _, notification := range retained {
		if string(notification.Metadata) != want {
			t.Fatalf("input or earlier recipient changed metadata: %s", notification.Metadata)
		}
	}
	retained[0].Metadata[0] = ' '
	if string(retained[1].Metadata) != want {
		t.Fatal("retained recipients share storage")
	}
}

func FuzzModerationMetadataJSONRoundTrip(f *testing.F) {
	for _, seed := range []string{`null`, `{}`, `[]`, `true`, `"text"`, `1e400`, `9007199254740993`, `{"nested":[1e400,9007199254740993]}`, `{"key":1,"key":2}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, metadata string) {
		if !json.Valid([]byte(metadata)) {
			t.Skip()
		}
		var notification protocol.TurnModerationMetadataNotification
		if err := json.Unmarshal([]byte(`{"threadId":"t","turnId":"u","metadata":`+metadata+`}`), &notification); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(notification)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &members); err != nil {
			t.Fatal(err)
		}
		decode := func(input []byte) interface{} {
			decoder := json.NewDecoder(bytes.NewReader(input))
			decoder.UseNumber()
			var value interface{}
			if err := decoder.Decode(&value); err != nil {
				t.Fatal(err)
			}
			return value
		}
		if !reflect.DeepEqual(decode([]byte(metadata)), decode(members["metadata"])) {
			t.Fatal("round trip changed JSON value")
		}
	})
}

func TestModerationMetadataEnvelopeMarshalling(t *testing.T) {
	notification := protocol.TurnModerationMetadataNotification{Metadata: json.RawMessage(`{"key":"value"}`), ThreadID: "t", TurnID: "u"}
	for _, value := range []interface{}{
		struct {
			protocol.TurnModerationMetadataNotification
			Extra string `json:"extra"`
		}{notification, "keep"},
		struct {
			Notification protocol.TurnModerationMetadataNotification `json:"notification"`
			Extra        string                                      `json:"extra"`
		}{notification, "keep"},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &members); err != nil {
			t.Fatal(err)
		}
		if string(members["extra"]) != `"keep"` {
			t.Fatalf("notification encoding consumed outer member: %s", encoded)
		}
	}
	var envelope struct {
		Notification protocol.TurnModerationMetadataNotification `json:"notification"`
		Extra        string                                      `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"notification":{"threadId":"t","turnId":"u","metadata":1e400},"extra":"keep"}`), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Extra != "keep" || string(envelope.Notification.Metadata) != "1e400" {
		t.Fatal("named envelope lost data")
	}
}

func TestModerationMetadataConstructionAndReplacement(t *testing.T) {
	for _, metadata := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`), json.RawMessage(`[]`), json.RawMessage(`9007199254740993`)} {
		notification := protocol.TurnModerationMetadataNotification{Metadata: metadata, ThreadID: "t", TurnID: "u"}
		encoded, err := json.Marshal(notification)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &members); err != nil {
			t.Fatal(err)
		}
		want := string(metadata)
		if metadata == nil {
			want = "null"
		}
		if string(members["metadata"]) != want {
			t.Fatalf("constructed branch changed: %s", encoded)
		}
	}
	for _, invalid := range []json.RawMessage{{}, json.RawMessage(`{`), json.RawMessage(`NaN`), json.RawMessage(`{} {}`)} {
		if _, err := json.Marshal(protocol.TurnModerationMetadataNotification{Metadata: invalid}); err == nil {
			t.Fatal("malformed constructed JSON accepted")
		}
	}
	old := json.RawMessage(`{"old":"value"}`)
	notification := protocol.TurnModerationMetadataNotification{Metadata: old}
	if err := json.Unmarshal([]byte(`{"threadId":"t","turnId":"u","metadata":[]}`), &notification); err != nil {
		t.Fatal(err)
	}
	notification.Metadata[0] = ' '
	if string(old) != `{"old":"value"}` {
		t.Fatal("receiver replacement changed previously retained storage")
	}
}

func TestModerationMetadataTypedDeliveryAndRecovery(t *testing.T) {
	mock := NewMockTransport()
	reports := 0
	client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(method string, _ error) {
		if method != "turn/moderationMetadata" {
			t.Fatalf("unexpected diagnostic method %q", method)
		}
		reports++
	}))
	defer client.Close()
	var replacement, appended []string
	record := func(destination *[]string) func(protocol.TurnModerationMetadataNotification) {
		return func(notification protocol.TurnModerationMetadataNotification) {
			encoded, err := json.Marshal(notification)
			if err != nil {
				t.Fatal(err)
			}
			var members map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &members); err != nil {
				t.Fatal(err)
			}
			*destination = append(*destination, string(members["metadata"]))
		}
	}
	client.OnTurnModerationMetadata(record(&replacement))
	remove := client.AddTurnModerationMetadataListener(record(&appended))
	defer remove()
	values := []string{`{}`, `null`, `[]`, `true`, `1`, `"text"`, `1e400`}
	for _, metadata := range values {
		mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "turn/moderationMetadata", Params: json.RawMessage(`{"threadId":"t","turnId":"u","metadata":` + metadata + `}`)})
	}
	if reports != 0 || !reflect.DeepEqual(replacement, values) || !reflect.DeepEqual(appended, values) {
		t.Fatalf("valid delivery: reports=%d replacement=%v appended=%v", reports, replacement, appended)
	}
	for _, invalid := range []string{`{"threadId":"t","turnId":"u"}`, `{"threadId":null,"turnId":"u","metadata":null}`} {
		mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "turn/moderationMetadata", Params: json.RawMessage(invalid)})
	}
	if reports != 4 || len(replacement) != len(values) || len(appended) != len(values) {
		t.Fatalf("rejected input was delivered: reports=%d replacement=%v appended=%v", reports, replacement, appended)
	}
	remove()
	remove()
	client.OnTurnModerationMetadata(nil)
	client.OnTurnModerationMetadata(record(&replacement))
	mock.InjectServerNotification(context.Background(), protocol.Notification{Method: "turn/moderationMetadata", Params: json.RawMessage(`{"threadId":"t","turnId":"u","metadata":["recovered"]}`)})
	if len(appended) != len(values) || len(replacement) != len(values)+1 || replacement[len(values)] != `["recovered"]` {
		t.Fatal("recovery or unsubscribe failed")
	}
}

func TestModerationMetadataDuplicateAndReceiverContract(t *testing.T) {
	for _, tc := range []struct{ fields, want string }{
		{`"metadata":{"a":1},"metadata":{"b":2}`, `{"a":1,"b":2}`},
		{`"metadata":{"a":1},"metadata":null,"metadata":{"b":2}`, `{"b":2}`},
		{`"metadata":{"a":1},"metadata":[],"metadata":{"b":2}`, `{"b":2}`},
		{`"metadata":[1],"metadata":true`, `true`},
		{`"metadata":true,"metadata":null`, `null`},
		{`"metadata":{"a":1},"Metadata":false`, `{"a":1}`},
		{`"\u006detadata":true`, `true`},
		{`"metadata":{"a":1},"metadata":{"a":9007199254740993}`, `{"a":9007199254740993}`},
	} {
		t.Run(tc.fields, func(t *testing.T) {
			notification := protocol.TurnModerationMetadataNotification{
				Metadata: json.RawMessage(`{"old":"value"}`), ThreadID: "old", TurnID: "old",
			}
			input := []byte(`{"threadId":"t","turnId":"u",` + tc.fields + `}`)
			if err := json.Unmarshal(input, &notification); err != nil {
				t.Fatal(err)
			}
			for i := range input {
				input[i] = ' '
			}
			encoded, err := json.Marshal(notification)
			if err != nil {
				t.Fatal(err)
			}
			var members map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &members); err != nil {
				t.Fatal(err)
			}
			if string(members["metadata"]) != tc.want {
				t.Fatalf("got %s want %s", members["metadata"], tc.want)
			}
		})
	}
	for _, input := range []string{
		`{"threadId":"t","turnId":"u"}`,
		`{"threadId":null,"turnId":"u","metadata":null}`,
		`{"threadId":"t","turnId":null,"metadata":null}`,
		`{"threadId":1,"threadId":"t","turnId":"u","metadata":{}}`,
		`{"threadId":"t","turnId":"u","metadata":{`,
	} {
		t.Run(input, func(t *testing.T) {
			original := protocol.TurnModerationMetadataNotification{Metadata: json.RawMessage(`{"keep":"value"}`), ThreadID: "old", TurnID: "old"}
			notification := original
			if err := json.Unmarshal([]byte(input), &notification); err == nil {
				t.Fatal("invalid notification accepted")
			}
			if !reflect.DeepEqual(notification, original) {
				t.Fatal("failed admission changed receiver")
			}
		})
	}
}
