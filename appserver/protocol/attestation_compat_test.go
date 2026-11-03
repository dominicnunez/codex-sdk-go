package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAttestationAdmissionRecovery(t *testing.T) {
	mock := NewMockTransport()
	c := p.NewClient(mock)
	t.Cleanup(func() { _ = c.Close() })
	calls := 0
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "preserved")
	c.SetApprovalHandlers(p.ApprovalHandlers{OnAttestationGenerate: func(got context.Context, _ p.AttestationGenerateParams) (p.AttestationGenerateResponse, error) {
		calls++
		if got.Value(contextKey{}) != "preserved" {
			t.Fatal("request context changed")
		}
		return p.AttestationGenerateResponse{Token: "synthetic-attestation"}, nil
	}})
	for _, id := range []p.RequestID{{Value: "string-id"}, {Value: int64(42)}} {
		for _, raw := range []string{`null`, `{}`, " \nnull\t", `{"extra":[null,{"future":1e10000}]}`} {
			before := calls
			response, err := mock.InjectServerRequest(ctx, p.Request{ID: id, Method: "attestation/generate", Params: json.RawMessage(raw)})
			if raw == `null` || raw == " \nnull\t" {
				if calls != before || !errors.Is(err, p.ErrInvalidParams) || len(response.Result) != 0 {
					t.Fatal("rejected request invoked callback or published result")
				}
			} else {
				if err != nil || calls != before+1 || !reflect.DeepEqual(response.ID, id) || string(response.Result) != `{"token":"synthetic-attestation"}` {
					t.Fatalf("same-client recovery or reply identity: %+v %v", response, err)
				}
			}
		}
	}
}

func TestAttestationExistingHandlerAndResponseContracts(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `[]`, `{`} {
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		id := p.RequestID{Value: "unconfigured"}
		response, err := mock.InjectServerRequest(context.Background(), p.Request{ID: id, Method: "attestation/generate", Params: json.RawMessage(raw)})
		if err != nil || !reflect.DeepEqual(response.ID, id) || response.Error == nil || response.Error.Code != p.ErrCodeMethodNotFound || len(response.Result) != 0 {
			t.Fatalf("nil handler priority changed: %+v %v", response, err)
		}
	}
	sentinel := errors.New("application failure")
	for _, appErr := range []error{nil, sentinel} {
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		c.SetApprovalHandlers(p.ApprovalHandlers{OnAttestationGenerate: func(context.Context, p.AttestationGenerateParams) (p.AttestationGenerateResponse, error) {
			return p.AttestationGenerateResponse{}, appErr
		}})
		response, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "fixture"}, Method: "attestation/generate", Params: json.RawMessage(`{}`)})
		if err == nil || len(response.Result) != 0 || errors.Is(err, p.ErrInvalidParams) {
			t.Fatal("invalid successful response or handler classification")
		}
		if appErr != nil && !errors.Is(err, sentinel) {
			t.Fatal("application error identity lost")
		}
		if appErr == nil && err.Error() != `validate attestation/generate result: missing token` {
			t.Fatalf("response validation changed: %v", err)
		}
	}
}

func TestAttestationPlainRecordAndNativeErrors(t *testing.T) {
	for _, value := range []any{p.AttestationGenerateParams{}, &p.AttestationGenerateParams{}} {
		if _, ok := value.(json.Unmarshaler); ok {
			t.Fatal("empty public record gained decoder")
		}
		if _, ok := value.(json.Marshaler); ok {
			t.Fatal("empty public record gained encoder")
		}
	}
	var anonymous struct {
		p.AttestationGenerateParams
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"Extra":"kept"}`), &anonymous); err != nil || anonymous.Extra != "kept" {
		t.Fatal("anonymous composition changed")
	}
	if err := json.Unmarshal([]byte(`null`), &anonymous); err != nil || anonymous.Extra != "kept" {
		t.Fatal("ordinary null composition changed")
	}
	var named struct {
		Params p.AttestationGenerateParams
		Extra  string
	}
	if err := json.Unmarshal([]byte(`{"Params":null,"Extra":"kept"}`), &named); err != nil || named.Extra != "kept" {
		t.Fatal("named composition changed")
	}
	for _, raw := range []string{`[]`, `7`, `false`, `"x"`, ``, `{`} {
		var native p.AttestationGenerateParams
		expected := json.Unmarshal([]byte(raw), &native)
		if expected == nil {
			t.Fatal("invalid native control")
		}
		mock := NewMockTransport()
		c := p.NewClient(mock)
		t.Cleanup(func() { _ = c.Close() })
		c.SetApprovalHandlers(p.ApprovalHandlers{OnAttestationGenerate: func(context.Context, p.AttestationGenerateParams) (p.AttestationGenerateResponse, error) {
			t.Fatal("invalid native request invoked callback")
			return p.AttestationGenerateResponse{}, nil
		}})
		_, actual := mock.InjectServerRequest(context.Background(), p.Request{Method: "attestation/generate", Params: json.RawMessage(raw)})
		if !errors.Is(actual, p.ErrInvalidParams) {
			t.Fatal("invalid-params classification lost")
		}
		var wantType, gotType *json.UnmarshalTypeError
		var wantSyntax, gotSyntax *json.SyntaxError
		if errors.As(expected, &wantType) {
			if !errors.As(actual, &gotType) || !reflect.DeepEqual(wantType, gotType) {
				t.Fatalf("native type error changed: %#v vs %#v", gotType, wantType)
			}
		} else if errors.As(expected, &wantSyntax) {
			if !errors.As(actual, &gotSyntax) || !reflect.DeepEqual(wantSyntax, gotSyntax) {
				t.Fatalf("native syntax error changed: %#v vs %#v", gotSyntax, wantSyntax)
			}
		} else {
			t.Fatalf("unexpected native control: %v", expected)
		}
	}
}
