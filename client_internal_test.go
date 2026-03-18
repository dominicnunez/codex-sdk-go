package codex

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestMarshalForWireFallback(t *testing.T) {
	type plain struct {
		Name string `json:"name"`
	}
	v := plain{Name: "test"}

	got, err := marshalForWire(v)
	if err != nil {
		t.Fatalf("marshalForWire() error = %v", err)
	}

	want, _ := json.Marshal(v)
	if string(got) != string(want) {
		t.Errorf("marshalForWire() = %s, want %s", got, want)
	}
}

func TestMarshalForWireReturnsErrorForTypedNilWireMarshaler(t *testing.T) {
	var params *ApiKeyLoginAccountParams
	_, err := marshalForWire(params)
	if !errors.Is(err, errNilWireMarshaler) {
		t.Fatalf("marshalForWire() error = %v, want errNilWireMarshaler", err)
	}
}

func TestIsEmptyResponseResult(t *testing.T) {
	tests := []struct {
		name   string
		result json.RawMessage
		want   bool
	}{
		{name: "nil", result: nil, want: true},
		{name: "empty slice", result: json.RawMessage{}, want: true},
		{name: "null", result: json.RawMessage(`null`), want: true},
		{name: "whitespace null", result: json.RawMessage(" \n\t null \r"), want: true},
		{name: "object", result: json.RawMessage(`{}`), want: false},
		{name: "array", result: json.RawMessage(`[]`), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyResponseResult(tt.result); got != tt.want {
				t.Fatalf("isEmptyResponseResult(%q) = %v, want %v", []byte(tt.result), got, tt.want)
			}
		})
	}
}

func TestValidateObjectResponseResult(t *testing.T) {
	tests := []struct {
		name   string
		result json.RawMessage
		target error
	}{
		{name: "empty", result: nil, target: ErrEmptyResult},
		{name: "null", result: json.RawMessage(`null`), target: ErrEmptyResult},
		{name: "empty object", result: json.RawMessage(`{}`)},
		{name: "object with fields", result: json.RawMessage(`{"ok":true}`)},
		{name: "array", result: json.RawMessage(`[]`), target: ErrResultNotObject},
		{name: "string", result: json.RawMessage(`"bad"`), target: ErrResultNotObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObjectResponseResult(tt.result)
			if tt.target == nil {
				if err != nil {
					t.Fatalf("validateObjectResponseResult() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.target) {
				t.Fatalf("validateObjectResponseResult() error = %v, want %v", err, tt.target)
			}
		})
	}
}

func TestSendRequestRejectsNilResultTarget(t *testing.T) {
	client := NewClient(&mockInternalTransport{})

	err := client.sendRequest(context.Background(), "test/method", nil, nil)
	if !errors.Is(err, errNilResponseTarget) {
		t.Fatalf("sendRequest() error = %v, want errNilResponseTarget", err)
	}
}
