package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type nativeResourceContent struct {
	Meta     json.RawMessage `json:"_meta,omitempty"`
	Blob     *string         `json:"blob,omitempty"`
	MimeType *string         `json:"mimeType,omitempty"`
	Text     *string         `json:"text,omitempty"`
	URI      string          `json:"uri"`
}

type nativeResourceReadResponse struct {
	Contents     []nativeResourceContent `json:"contents"`
	OriginCallID *string                 `json:"originCallId,omitempty"`
}

type nativePresenceResourceContent struct {
	URI  *string `json:"uri"`
	Text *string `json:"text,omitempty"`
	Blob *string `json:"blob,omitempty"`
}

type nativePresenceResourceReadResponse struct {
	Contents []nativePresenceResourceContent `json:"contents"`
}

func readResourcePayload(t *testing.T, payload string) (codex.McpResourceReadResponse, error) {
	t.Helper()
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	if err := mock.SetResponseData("mcpServer/resource/read", json.RawMessage(payload)); err != nil {
		t.Fatalf("SetResponseData: %v", err)
	}
	return client.Mcp.ResourceRead(context.Background(), codex.McpResourceReadParams{
		Server: "server-1",
		URI:    "resource://item",
	})
}

func TestMcpResourceReadAdmissionPolicies(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantErr  bool
		wantText *string
		wantBlob *string
		wantURI  string
		errField string
		errValue string
	}{
		{name: "text branch accepts incompatible blob extra", payload: `{"contents":[{"uri":"u","text":"t","blob":7}]}`, wantText: stringPointer("t"), wantURI: "u"},
		{name: "blob branch accepts incompatible text extra", payload: `{"contents":[{"uri":"u","blob":"b","text":{}}]}`, wantBlob: stringPointer("b"), wantURI: "u"},
		{name: "text branch accepts null opposite and nullable MIME", payload: `{"contents":[{"uri":"u","text":"","blob":null,"mimeType":null}]}`, wantText: stringPointer(""), wantURI: "u"},
		{name: "blob branch accepts null opposite text", payload: `{"contents":[{"uri":"u","blob":"","text":null}]}`, wantBlob: stringPointer(""), wantURI: "u"},
		{name: "both bad branches repaired in same source object", payload: `{"contents":[{"uri":"u","text":7,"blob":false,"text":"t","blob":"b"}]}`, wantText: stringPointer("t"), wantBlob: stringPointer("b"), wantURI: "u"},
		{name: "bad text can use inherited blob at source object end", payload: `{"contents":[{"uri":"u","blob":"b"}],"contents":[{"uri":"v","text":7}]}`, wantBlob: stringPointer("b"), wantURI: "v"},
		{name: "bad text followed by repair still needs same object blob", payload: `{"contents":[{"uri":"u","text":7,"text":"t"}]}`, wantErr: true, errField: "contents.text", errValue: "number"},
		{name: "later separate object cannot classify earlier bad text", payload: `{"contents":[{"uri":"u","text":7}],"contents":[{"uri":"v","blob":"b"}]}`, wantErr: true, errField: "contents.text", errValue: "number"},
		{name: "both branches absent", payload: `{"contents":[{"uri":"u"}]}`, wantErr: true},
		{name: "URI absent", payload: `{"contents":[{"text":"t"}]}`, wantErr: true},
		{name: "URI null", payload: `{"contents":[{"uri":null,"text":"t"}]}`, wantErr: true},
		{name: "null content item", payload: `{"contents":[null]}`, wantErr: true},
		{name: "invalid shared MIME type remains native error", payload: `{"contents":[{"uri":"u","text":"t","mimeType":7}]}`, wantErr: true, errField: "contents.mimeType", errValue: "number"},
		{name: "invalid MIME type with extra falls back", payload: `{"contents":[{"uri":"u","text":"t","mimeType":7,"blob":false}]}`, wantErr: true, errField: "contents.mimeType", errValue: "number"},
		{name: "folded invalid MIME type with extra falls back", payload: `{"contents":[{"uri":"u","text":"t","MIMETYPE":7,"blob":false}]}`, wantErr: true, errField: "contents.mimeType", errValue: "number"},
		{name: "escaped invalid MIME type with extra falls back", payload: `{"contents":[{"uri":"u","text":"t","m\u0069meType":7,"blob":false}]}`, wantErr: true, errField: "contents.mimeType", errValue: "number"},
		{name: "invalid URI remains native error", payload: `{"contents":[{"uri":7,"text":"t"}]}`, wantErr: true, errField: "contents.uri", errValue: "number"},
		{name: "invalid origin call ID remains native error", payload: `{"contents":[{"uri":"u","text":"t"}],"originCallId":7,"originCallId":"repaired"}`, wantErr: true, errField: "originCallId", errValue: "number"},
		{name: "invalid origin call ID with extra falls back", payload: `{"originCallId":7,"contents":[{"uri":"u","text":"t","blob":false}]}`, wantErr: true, errField: "originCallId", errValue: "number"},
		{name: "folded invalid origin call ID with extra falls back", payload: `{"ORIGINCALLID":7,"contents":[{"uri":"u","text":"t","blob":false}]}`, wantErr: true, errField: "originCallId", errValue: "number"},
		{name: "escaped invalid origin call ID with extra falls back", payload: `{"or\u0069ginCallId":7,"contents":[{"uri":"u","text":"t","blob":false}]}`, wantErr: true, errField: "originCallId", errValue: "number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var direct codex.McpResourceReadResponse
			directErr := json.Unmarshal([]byte(tt.payload), &direct)
			public, publicErr := readResourcePayload(t, tt.payload)
			if tt.wantErr {
				if directErr == nil || publicErr == nil {
					t.Fatalf("expected direct and public decode errors; direct=%v public=%v", directErr, publicErr)
				}
				if tt.errField != "" {
					var native nativeResourceReadResponse
					nativeErr := json.Unmarshal([]byte(tt.payload), &native)
					var nativeTypeErr *json.UnmarshalTypeError
					if !errors.As(nativeErr, &nativeTypeErr) {
						t.Fatalf("native plain reference error = %T %v", nativeErr, nativeErr)
					}
					for name, err := range map[string]error{"direct": directErr, "public": publicErr} {
						var typeErr *json.UnmarshalTypeError
						if !errors.As(err, &typeErr) {
							t.Errorf("%s error %T is not an UnmarshalTypeError: %v", name, err, err)
							continue
						}
						if typeErr.Field != tt.errField || typeErr.Value != tt.errValue {
							t.Errorf("%s type error = {Field:%q Value:%q}; want {Field:%q Value:%q}", name, typeErr.Field, typeErr.Value, tt.errField, tt.errValue)
						}
						if typeErr.Offset != nativeTypeErr.Offset || typeErr.Type != nativeTypeErr.Type || typeErr.Struct != "ResourceContent" && typeErr.Struct != "wire" {
							t.Errorf("%s type error context = {Struct:%q Type:%v Offset:%d}; native offset/type={%v %d}", name, typeErr.Struct, typeErr.Type, typeErr.Offset, nativeTypeErr.Type, nativeTypeErr.Offset)
						}
					}
				}
				return
			}
			if directErr != nil || publicErr != nil {
				t.Fatalf("decode failed: direct=%v public=%v", directErr, publicErr)
			}
			for name, response := range map[string]codex.McpResourceReadResponse{"direct": direct, "public": public} {
				if len(response.Contents) != 1 {
					t.Errorf("%s content count = %d, want 1", name, len(response.Contents))
					continue
				}
				content := response.Contents[0]
				if content.URI != tt.wantURI || !sameOptionalString(content.Text, tt.wantText) || !sameOptionalString(content.Blob, tt.wantBlob) {
					t.Errorf("%s content = {uri:%q text:%v blob:%v}; want {uri:%q text:%v blob:%v}", name, content.URI, content.Text, content.Blob, tt.wantURI, tt.wantText, tt.wantBlob)
				}
			}
		})
	}
}

func TestMcpResourceReadDuplicateArrayMergeMatchesNativeReference(t *testing.T) {
	payloads := []string{
		`{"contents":[{"uri":"u"}],"contents":[{"text":"t"}]}`,
		`{"contents":[{"uri":"a","text":"ta"},{"uri":"b","blob":"bb"}],"contents":[{"uri":"c"}],"contents":[{"uri":"d"},{"uri":"e"}]}`,
		`{"contents":[{"uri":"a","text":"ta"},{"uri":"b","blob":"bb"}],"contents":[],"contents":[{"uri":"c","text":"tc"},{"uri":"d","blob":"db"}]}`,
		`{"contents":[{"uri":"","text":""},{"uri":"b","blob":"bb"}],"contents":[{"URI":"c","text":null,"blob":"cb"}]}`,
		`{"contents":[{"uri":"a","text":"ta"}],"contents":[{"uri":"b","text":null,"blob":"bb"}]}`,
	}
	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			var reference nativeResourceReadResponse
			if err := json.Unmarshal([]byte(payload), &reference); err != nil {
				t.Fatalf("native reference decode: %v", err)
			}
			var direct codex.McpResourceReadResponse
			if err := json.Unmarshal([]byte(payload), &direct); err != nil {
				t.Fatalf("direct response decode: %v", err)
			}
			public, err := readResourcePayload(t, payload)
			if err != nil {
				t.Fatalf("public response decode: %v", err)
			}
			for name, response := range map[string]codex.McpResourceReadResponse{"direct": direct, "public": public} {
				if len(response.Contents) != len(reference.Contents) {
					t.Errorf("%s contents length = %d, native reference = %d", name, len(response.Contents), len(reference.Contents))
					continue
				}
				if !reflect.DeepEqual(response.OriginCallID, reference.OriginCallID) {
					t.Errorf("%s originCallId = %v, native reference = %v", name, response.OriginCallID, reference.OriginCallID)
				}
				for i, got := range response.Contents {
					want := reference.Contents[i]
					if got.URI != want.URI || !reflect.DeepEqual(got.Text, want.Text) || !reflect.DeepEqual(got.Blob, want.Blob) || !reflect.DeepEqual(got.MimeType, want.MimeType) || !bytes.Equal(got.Meta, want.Meta) {
						t.Errorf("%s contents[%d] differs from native duplicate-merge reference: got %#v, want %#v", name, i, got, want)
					}
				}
			}
		})
	}
}

func TestMcpResourceReadGeneratedDuplicateArraysMatchNativeReference(t *testing.T) {
	for initialLength := 1; initialLength <= 4; initialLength++ {
		for middleLength := 0; middleLength <= 5; middleLength++ {
			for finalLength := 0; finalLength <= 6; finalLength++ {
				for _, reset := range []bool{false, true} {
					if reset && middleLength != 0 {
						continue
					}
					name := fmt.Sprintf("initial-%d-middle-%d-final-%d-reset-%t", initialLength, middleLength, finalLength, reset)
					t.Run(name, func(t *testing.T) {
						initial := make([]string, initialLength)
						for i := range initial {
							if i%2 == 0 {
								initial[i] = fmt.Sprintf(`{"uri":"i%d","text":"t%d"}`, i, i)
							} else {
								initial[i] = fmt.Sprintf(`{"uri":"i%d","blob":"b%d"}`, i, i)
							}
						}
						middle := make([]string, middleLength)
						for i := range middle {
							middle[i] = fmt.Sprintf(`{"uri":"m%d"}`, i)
						}
						final := make([]string, finalLength)
						for i := range final {
							final[i] = fmt.Sprintf(`{"uri":"f%d"}`, i)
						}
						payload := fmt.Sprintf(`{"contents":[%s],"contents":[%s],"contents":[%s]}`, strings.Join(initial, ","), strings.Join(middle, ","), strings.Join(final, ","))
						if reset {
							payload = fmt.Sprintf(`{"contents":[%s],"contents":[],"contents":[%s]}`, strings.Join(initial, ","), strings.Join(final, ","))
						}

						var reference nativeResourceReadResponse
						if err := json.Unmarshal([]byte(payload), &reference); err != nil {
							t.Fatalf("native reference decode: %v", err)
						}
						wantAccept := true
						// This length-only generator supplies a nonempty URI in every item;
						// URI presence and empty-string behavior are covered by the pointer oracle.
						for _, item := range reference.Contents {
							if item.Text == nil && item.Blob == nil {
								wantAccept = false
								break
							}
						}
						var direct codex.McpResourceReadResponse
						directErr := json.Unmarshal([]byte(payload), &direct)
						public, publicErr := readResourcePayload(t, payload)
						if (directErr == nil) != wantAccept || (publicErr == nil) != wantAccept {
							t.Fatalf("acceptance direct=%v public=%v want=%t; payload=%s", directErr, publicErr, wantAccept, payload)
						}
						if !wantAccept {
							return
						}
						for label, got := range map[string]codex.McpResourceReadResponse{"direct": direct, "public": public} {
							if len(got.Contents) != len(reference.Contents) {
								t.Errorf("%s length=%d, reference=%d", label, len(got.Contents), len(reference.Contents))
								continue
							}
							for i, item := range got.Contents {
								want := reference.Contents[i]
								if item.URI != want.URI || !reflect.DeepEqual(item.Text, want.Text) || !reflect.DeepEqual(item.Blob, want.Blob) {
									t.Errorf("%s item %d=%#v; reference=%#v", label, i, item, want)
								}
							}
						}
					})
				}
			}
		}
	}
}

type generatedResourceContent struct {
	uri      *string
	uriNull  bool
	text     *string
	textNull bool
	blob     *string
	blobNull bool
	nullItem bool
}

func TestMcpResourceReadGeneratedPresenceMatchesIndependentModel(t *testing.T) {
	quote := func(value string) string {
		encoded, _ := json.Marshal(value)
		return string(encoded)
	}
	itemJSON := func(item generatedResourceContent, variant int) string {
		if item.nullItem {
			return "null"
		}
		uriKey := []string{"uri", "URI", `u\u0072i`}[variant%3]
		textKey := []string{"text", "TEXT", `t\u0065xt`}[variant%3]
		blobKey := []string{"blob", "BLOB", `b\u006cob`}[variant%3]
		var fields []string
		if item.uriNull {
			fields = append(fields, `"`+uriKey+`":null`)
		} else if item.uri != nil {
			fields = append(fields, `"`+uriKey+`":`+quote(*item.uri))
		}
		if item.textNull {
			fields = append(fields, `"`+textKey+`":null`)
		} else if item.text != nil {
			fields = append(fields, `"`+textKey+`":`+quote(*item.text))
		}
		if item.blobNull {
			fields = append(fields, `"`+blobKey+`":null`)
		} else if item.blob != nil {
			fields = append(fields, `"`+blobKey+`":`+quote(*item.blob))
		}
		return `{` + strings.Join(fields, ",") + `}`
	}

	accepted, rejected, populatedAccepted, emptyURIAccepted := 0, 0, 0, 0
	for initialLength := 1; initialLength <= 3; initialLength++ {
		for duplicateLength := 0; duplicateLength <= 4; duplicateLength++ {
			for _, reset := range []bool{false, true} {
				for event := 0; event < 4; event++ {
					initial := make([]generatedResourceContent, initialLength)
					for i := range initial {
						u := fmt.Sprintf("initial-%d", i)
						if i%2 == 0 {
							u = ""
						}
						initial[i].uri = &u
						if i%2 == 0 {
							initial[i].text = stringPointer(fmt.Sprintf("text-%d", i))
						} else {
							initial[i].blob = stringPointer(fmt.Sprintf("blob-%d", i))
						}
					}
					if event == 1 {
						initial[0].uriNull = true
					}
					if event == 2 {
						initial[0].nullItem = true
					}

					duplicate := make([]generatedResourceContent, duplicateLength)
					for i := range duplicate {
						if i%3 == 1 {
							u := fmt.Sprintf("duplicate-%d", i)
							duplicate[i].uri = &u
						}
						switch i % 3 {
						case 0:
							duplicate[i].textNull = true
						case 1:
							duplicate[i].blob = stringPointer(fmt.Sprintf("new-blob-%d", i))
						case 2:
							duplicate[i].blobNull = true
						}
					}
					preserveBranch := duplicateLength > 0 && (initialLength+duplicateLength)%2 == 0
					if event == 0 && preserveBranch {
						duplicate[0].textNull = false
						duplicate[0].blob = stringPointer("replacement-blob")
						if reset {
							u := ""
							duplicate[0].uri = &u
						}
					}
					if duplicateLength > 0 && (event == 1 || event == 2) {
						u := "repaired-uri"
						duplicate[0].uri = &u
						duplicate[0].textNull = false
						duplicate[0].blobNull = false
						duplicate[0].text = stringPointer("repaired-text")
					}

					arrays := [][]generatedResourceContent{initial}
					if event == 3 {
						arrays = append(arrays, []generatedResourceContent{})
					}
					if reset {
						arrays = append(arrays, []generatedResourceContent{})
					}
					arrays = append(arrays, duplicate)

					var payloadParts []string
					for arrayIndex, array := range arrays {
						rootKey := "contents"
						if arrayIndex > 0 && arrayIndex%2 == 1 {
							rootKey = "CONTENTS"
						}
						if arrayIndex > 0 && arrayIndex%2 == 0 {
							rootKey = `cont\u0065nts`
						}
						var items []string
						for itemIndex, item := range array {
							items = append(items, itemJSON(item, arrayIndex+itemIndex))
						}
						payloadParts = append(payloadParts, `"`+rootKey+`":[`+strings.Join(items, ",")+"]")
						if arrayIndex == 0 && event == 3 {
							payloadParts = append(payloadParts, `"CONTENTS":null`)
						}
					}
					payload := `{` + strings.Join(payloadParts, ",") + `}`

					var required nativePresenceResourceReadResponse
					if err := json.Unmarshal([]byte(payload), &required); err != nil {
						t.Fatalf("native required-presence reference decode: %v; payload=%s", err, payload)
					}
					var values nativeResourceReadResponse
					if err := json.Unmarshal([]byte(payload), &values); err != nil {
						t.Fatalf("native value reference decode: %v", err)
					}
					wantAccept := event == 0
					for _, item := range required.Contents {
						if item.URI == nil || item.Text == nil && item.Blob == nil {
							wantAccept = false
							break
						}
					}
					if wantAccept {
						accepted++
						if len(required.Contents) > 0 {
							populatedAccepted++
							if required.Contents[0].URI != nil && *required.Contents[0].URI == "" {
								emptyURIAccepted++
							}
						}
					} else {
						rejected++
					}

					name := fmt.Sprintf("initial-%d-duplicate-%d-reset-%t-event-%d", initialLength, duplicateLength, reset, event)
					t.Run(name, func(t *testing.T) {
						var direct codex.McpResourceReadResponse
						directErr := json.Unmarshal([]byte(payload), &direct)
						public, publicErr := readResourcePayload(t, payload)
						if (directErr == nil) != wantAccept || (publicErr == nil) != wantAccept {
							t.Fatalf("acceptance direct=%v public=%v want=%t payload=%s", directErr, publicErr, wantAccept, payload)
						}
						if !wantAccept {
							if len(public.Contents) != 0 {
								t.Fatalf("rejected public result published %d contents", len(public.Contents))
							}
							return
						}
						for label, got := range map[string]codex.McpResourceReadResponse{"direct": direct, "public": public} {
							if len(got.Contents) != len(values.Contents) {
								t.Errorf("%s length=%d, reference=%d", label, len(got.Contents), len(values.Contents))
								continue
							}
							for i, item := range got.Contents {
								want := values.Contents[i]
								if item.URI != want.URI || !reflect.DeepEqual(item.Text, want.Text) || !reflect.DeepEqual(item.Blob, want.Blob) {
									t.Errorf("%s item %d=%#v; reference=%#v", label, i, item, want)
								}
							}
						}
					})
				}
			}
		}
	}
	if accepted == 0 || rejected == 0 || populatedAccepted == 0 || emptyURIAccepted == 0 {
		t.Fatalf("generated reference covered accepted=%d rejected=%d populated=%d populated-empty-URI=%d cases", accepted, rejected, populatedAccepted, emptyURIAccepted)
	}
}
func TestMcpResourceReadAdmissionPreservesPlainResourceContentContract(t *testing.T) {
	if reflect.TypeOf(codex.ResourceContent{}).NumMethod() != 0 || reflect.TypeOf((*codex.ResourceContent)(nil)).NumMethod() != 0 {
		t.Fatal("ResourceContent acquired a JSON method")
	}
	type envelope struct {
		codex.ResourceContent
		Kind string `json:"kind"`
	}
	var wrapped envelope
	if err := json.Unmarshal([]byte(`{"uri":"u","kind":"application"}`), &wrapped); err != nil {
		t.Fatalf("plain application envelope decode: %v", err)
	}
	if wrapped.URI != "u" || wrapped.Text != nil || wrapped.Blob != nil || wrapped.Kind != "application" {
		t.Fatalf("plain envelope = %#v", wrapped)
	}
	encoded, err := json.Marshal(codex.ResourceContent{URI: "u"})
	if err != nil {
		t.Fatalf("plain ResourceContent marshal: %v", err)
	}
	if string(encoded) != `{"uri":"u"}` {
		t.Fatalf("plain ResourceContent JSON = %s", encoded)
	}
}

func TestMcpResourceReadAdmissionKeepsMetaRawMessageOwned(t *testing.T) {
	metas := []string{"null", "true", "9007199254740993", `"raw"`, `[1,null]`, `{"n":9007199254740993,"nil":null}`}
	for _, meta := range metas {
		for _, recoverExtra := range []bool{false, true} {
			name := fmt.Sprintf("%s/recover-extra-%t", meta, recoverExtra)
			t.Run(name, func(t *testing.T) {
				member := `"_meta":` + meta
				if recoverExtra {
					member += `,"blob":7`
				}
				payload := []byte(`{"contents":[{"uri":"u","text":"t",` + member + `}]}`)
				var direct codex.McpResourceReadResponse
				if err := json.Unmarshal(payload, &direct); err != nil {
					t.Fatalf("direct decode response: %v", err)
				}
				public, err := readResourcePayload(t, string(payload))
				if err != nil {
					t.Fatalf("public decode response: %v", err)
				}
				wantMeta := []byte(meta)
				for label, response := range map[string]codex.McpResourceReadResponse{"direct": direct, "public": public} {
					if len(response.Contents) != 1 || !bytes.Equal(response.Contents[0].Meta, wantMeta) {
						t.Fatalf("%s _meta bytes = %#v, want %s", label, response.Contents, wantMeta)
					}
				}
				for i := range payload {
					payload[i] = 'x'
				}
				if !bytes.Equal(direct.Contents[0].Meta, wantMeta) {
					t.Fatalf("direct _meta aliases input: got %s, want %s", direct.Contents[0].Meta, wantMeta)
				}
			})
		}
	}
}

func TestMcpResourceReadAdmissionErrorsPreserveSeededReceiver(t *testing.T) {
	text := stringPointer("seed-text")
	blob := stringPointer("seed-blob")
	mime := stringPointer("seed-mime")
	origin := stringPointer("seed-origin")
	meta := json.RawMessage(`{"seed":true}`)
	response := codex.McpResourceReadResponse{
		Contents:     []codex.ResourceContent{{URI: "seed-uri", Text: text, Blob: blob, MimeType: mime, Meta: meta}},
		OriginCallID: origin,
	}
	contentsAddress := &response.Contents[0]
	textAddress := response.Contents[0].Text
	blobAddress := response.Contents[0].Blob
	mimeAddress := response.Contents[0].MimeType
	metaAddress := &response.Contents[0].Meta[0]
	originAddress := response.OriginCallID
	for _, payload := range []string{
		`{"contents":[{"uri":"new"}]}`,
		`{"contents":[{"uri":"new","text":7}]}`,
	} {
		if err := json.Unmarshal([]byte(payload), &response); err == nil {
			t.Fatalf("expected admission error for %s", payload)
		}
		if &response.Contents[0] != contentsAddress || response.Contents[0].Text != textAddress || response.Contents[0].Blob != blobAddress || response.Contents[0].MimeType != mimeAddress || &response.Contents[0].Meta[0] != metaAddress || response.OriginCallID != originAddress {
			t.Fatal("failed admission changed seeded receiver references")
		}
		content := response.Contents[0]
		if content.URI != "seed-uri" || *content.Text != "seed-text" || *content.Blob != "seed-blob" || *content.MimeType != "seed-mime" || *response.OriginCallID != "seed-origin" || string(content.Meta) != `{"seed":true}` {
			t.Fatalf("failed admission changed seeded receiver values: %#v origin=%v meta=%s", content, *response.OriginCallID, content.Meta)
		}
	}
}

func TestMcpResourceReadAdmissionFoldedContentsNullIsRejected(t *testing.T) {
	payload := `{"contents":[{"uri":"u","text":"t"}],"CONTENTS":null}`
	var direct codex.McpResourceReadResponse
	if err := json.Unmarshal([]byte(payload), &direct); err == nil || err.Error() != `server returned null for required result field "contents"` {
		t.Fatalf("direct folded-null error = %v", err)
	}
	if _, err := readResourcePayload(t, payload); err == nil || !bytes.Contains([]byte(err.Error()), []byte(`server returned null for required result field "contents"`)) {
		t.Fatalf("public folded-null error = %v", err)
	}
}
