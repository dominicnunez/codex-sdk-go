package protocol_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestOptionalArrayPlainReceiverCompatibility(t *testing.T) {
	// Independent, method-free stdlib models retain the original public type
	// names, field tags and error context. Forbidden null arrays are deliberately
	// excluded from this valid/established-type-error compatibility comparison.
	type ReasoningThreadItem codex.ReasoningThreadItem
	type TextUserInput codex.TextUserInput
	type SandboxWorkspaceWrite codex.SandboxWorkspaceWrite
	type SandboxPolicyWorkspaceWrite codex.SandboxPolicyWorkspaceWrite
	for _, tc := range []struct {
		name              string
		actual, reference any
		bodies            []string
	}{
		{"reasoning", &codex.ReasoningThreadItem{ID: "old", Summary: []string{"prior"}}, &ReasoningThreadItem{ID: "old", Summary: []string{"prior"}}, []string{`{}`, `null`, `{"ID":"new","SUMMARY":["a"],"content":[""]}`, `{"summary":[],"summary":["last"]}`, `{"id":42,"content":["last"]}`, `{"id":"new","summary":[42],"content":["last"]}`, `{"id":"new"`, `[]`}},
		{"text", &codex.TextUserInput{Text: "old", TextElements: []codex.TextElement{{ByteRange: codex.ByteRange{Start: 0, End: 1}}}}, &TextUserInput{Text: "old", TextElements: []codex.TextElement{{ByteRange: codex.ByteRange{Start: 0, End: 1}}}}, []string{`{}`, `null`, `{"TEXT":"new","text_elements":[]}`, `{"text":42,"text_elements":[]}`, `{"text":"new","text_elements":true}`, `[]`}},
		{"config", &codex.SandboxWorkspaceWrite{WritableRoots: []string{"prior"}}, &SandboxWorkspaceWrite{WritableRoots: []string{"prior"}}, []string{`{}`, `null`, `{"WRITABLE_ROOTS":["relative",""]}`, `{"writable_roots":[],"writable_roots":["last"]}`, `{"writable_roots":42,"network_access":true}`, `{"network_access":42,"writable_roots":["last"]}`, `[]`}},
		{"policy", &codex.SandboxPolicyWorkspaceWrite{WritableRoots: []string{"/tmp/prior"}}, &SandboxPolicyWorkspaceWrite{WritableRoots: []string{"/tmp/prior"}}, []string{`{}`, `null`, `{"WRITABLEROOTS":["/tmp/new"]}`, `{"writableRoots":42,"networkAccess":true}`, `{"networkAccess":42,"writableRoots":["/tmp/new"]}`, `[]`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, body := range tc.bodies {
				gotErr := json.Unmarshal([]byte(body), tc.actual)
				wantErr := json.Unmarshal([]byte(body), tc.reference)
				var wantType *json.UnmarshalTypeError
				if errors.As(wantErr, &wantType) && wantType.Type == reflect.ValueOf(tc.reference).Elem().Type() {
					// The external reference's package differs. Its root model type
					// represents the original public type in the direct contract.
					wantType.Type = reflect.ValueOf(tc.actual).Elem().Type()
				}
				if !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("%s error=%+v, reference=%+v", body, gotErr, wantErr)
				}
				got := reflect.ValueOf(tc.actual).Elem().Convert(reflect.ValueOf(tc.reference).Elem().Type()).Interface()
				if !reflect.DeepEqual(got, reflect.ValueOf(tc.reference).Elem().Interface()) {
					t.Fatalf("%s receiver differs from stdlib: %+v vs %+v", body, got, tc.reference)
				}
			}
		})
	}
}

func TestOptionalArrayNamedEnvelope(t *testing.T) {
	var envelope struct {
		Before string                    `json:"before"`
		Item   codex.ReasoningThreadItem `json:"item"`
		After  string                    `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"before":"b","item":{"id":"r","summary":["s"]},"after":"a"}`), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Before != "b" || envelope.After != "a" || !reflect.DeepEqual(envelope.Item.Summary, []string{"s"}) {
		t.Fatal("named envelope lost valid sibling data")
	}
	prior := envelope.Item.Summary
	if err := json.Unmarshal([]byte(`{"before":"changed","item":{"summary":null},"after":"unreached"}`), &envelope); err == nil {
		t.Fatal("enclosing decoder accepted forbidden array")
	}
	if envelope.Before != "changed" || envelope.After != "a" || !reflect.DeepEqual(prior, envelope.Item.Summary) {
		t.Fatal("failed enclosing decode or retained item state differs from documented boundary")
	}
}

func TestOptionalArrayFailedEnvelopeBoundary(t *testing.T) {
	type ReasoningThreadItem codex.ReasoningThreadItem
	type actualEnvelope struct {
		Before int                       `json:"before"`
		Item   codex.ReasoningThreadItem `json:"item"`
		After  string                    `json:"after"`
	}
	type referenceEnvelope struct {
		Before int                 `json:"before"`
		Item   ReasoningThreadItem `json:"item"`
		After  string              `json:"after"`
	}
	for _, item := range []string{`{"id":42,"content":["valid"]}`, `{"id":"r","summary":[42]}`} {
		for _, before := range []string{`1`, `"bad"`} {
			body := `{"before":` + before + `,"item":` + item + `,"after":"a"}`
			var actual actualEnvelope
			var reference referenceEnvelope
			gotErr := json.Unmarshal([]byte(body), &actual)
			wantErr := json.Unmarshal([]byte(body), &reference)
			var got, want *json.UnmarshalTypeError
			if !errors.As(gotErr, &got) || !errors.As(wantErr, &want) {
				t.Fatalf("actual=%v reference=%v", gotErr, wantErr)
			}
			var inner ReasoningThreadItem
			var innerType *json.UnmarshalTypeError
			if !errors.As(json.Unmarshal([]byte(item), &inner), &innerType) {
				t.Fatal("invalid reference item did not fail")
			}
			if got.Field != "item."+innerType.Field || got.Struct != "actualEnvelope" || got.Offset != innerType.Offset {
				t.Fatalf("concrete error composition = %+v, inner=%+v", got, innerType)
			}
			if actual.After != "" || reference.After != "a" {
				t.Fatal("failed custom item no longer stops outer traversal as documented")
			}
			if before == `"bad"` && want.Field != "before" {
				t.Fatal("stdlib reference no longer retains first outer type error")
			}
		}
	}
	var item codex.ReasoningThreadItem
	err := json.Unmarshal([]byte(`{"summary":[42,null]}`), &item)
	if err == nil || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("mixed invalid/null list guard = %v", err)
	}
	// The new null-admission failure takes precedence in a formerly plain
	// concrete owner; this is not an established valid-array type-error claim.
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		t.Fatal("mixed null list unexpectedly reported only the saved scalar mismatch")
	}
}
