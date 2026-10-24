package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAccountEmailAcceptsNull(t *testing.T) {
	raw := `{"type":"chatgpt","email":null,"planType":"plus"}`
	t.Run("direct", func(t *testing.T) {
		var account p.ChatgptAccount
		if err := json.Unmarshal([]byte(raw), &account); err != nil {
			t.Fatalf("schema-valid null email rejected: %v", err)
		}
		encoded, err := json.Marshal(&account)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["email"]) != "null" {
			t.Fatalf("null email changed: %s", encoded)
		}
	})
	t.Run("public read", func(t *testing.T) {
		mock := NewMockTransport()
		mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":` + raw + `}`)})
		got, err := p.NewClient(mock).Account.Get(context.Background(), p.GetAccountParams{})
		if err != nil {
			t.Fatalf("schema-valid account read rejected: %v", err)
		}
		if got.Account == nil {
			t.Fatal("missing account")
		}
		if _, ok := got.Account.Value.(*p.ChatgptAccount); !ok {
			t.Fatalf("account = %T", got.Account.Value)
		}
	})
}

func TestAccountEmailPresenceAndRoundTrip(t *testing.T) {
	for _, tc := range []struct{ name, members, want string }{
		{"null", `"email":null`, `null`},
		{"empty", `"email":""`, `""`},
		{"text", `"email":"a@b.example"`, `"a@b.example"`},
		{"escaped key", `"em\u0061il":null`, `null`},
		{"reset", `"email":"before","email":null`, `null`},
		{"replace null", `"email":null,"email":"after"`, `"after"`},
		{"native alias reset", `"email":"before","EMAIL":null`, `null`},
		{"native alias replace", `"email":null,"EMAIL":"after"`, `"after"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"type":"chatgpt","planType":"plus",` + tc.members + `}`
			old := "old"
			direct := p.ChatgptAccount{Email: &old, PlanType: p.PlanTypeFree}
			if err := json.Unmarshal([]byte(raw), &direct); err != nil {
				t.Fatal(err)
			}
			if old != "old" {
				t.Fatal("success changed prior retained email")
			}
			checkAccountEmailJSON(t, &direct, tc.want)
			wrapper := p.AccountWrapper{Value: &p.ApiKeyAccount{}}
			if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
				t.Fatal(err)
			}
			checkAccountEmailJSON(t, &wrapper, tc.want)
			mock := NewMockTransport()
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":true,"account":` + raw + `}`)})
			got, err := p.NewClient(mock).Account.Get(context.Background(), p.GetAccountParams{})
			if err != nil {
				t.Fatal(err)
			}
			if !got.RequiresOpenaiAuth || got.Account == nil {
				t.Fatalf("read lost fields: %+v", got)
			}
			checkAccountEmailJSON(t, got.Account, tc.want)
		})
	}
}

func checkAccountEmailJSON(t *testing.T, account any, want string) {
	t.Helper()
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["email"]) != want || string(fields["type"]) != `"chatgpt"` || string(fields["planType"]) != `"plus"` {
		t.Fatalf("encoded account = %s; email want %s", encoded, want)
	}
	var roundTrip p.AccountWrapper
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	decoded, ok := roundTrip.Value.(*p.ChatgptAccount)
	if !ok {
		t.Fatalf("round trip variant = %T", roundTrip.Value)
	}
	if want == "null" {
		if decoded.Email != nil {
			t.Fatalf("null email became %q", *decoded.Email)
		}
	} else {
		var text string
		if err := json.Unmarshal([]byte(want), &text); err != nil {
			t.Fatal(err)
		}
		if decoded.Email == nil || *decoded.Email != text {
			t.Fatalf("email = %v; want %q", decoded.Email, text)
		}
	}
}

func TestAccountEmailFailuresPreserveOwners(t *testing.T) {
	for _, tc := range []struct {
		name, raw          string
		missing, nullField bool
	}{
		{"missing", `{"type":"chatgpt","planType":"plus"}`, true, false},
		{"alias only remains missing", `{"type":"chatgpt","EMAIL":null,"planType":"plus"}`, true, false},
		{"number", `{"type":"chatgpt","email":7,"planType":"plus"}`, false, false},
		{"hidden wrong kind", `{"type":"chatgpt","email":{},"email":null,"planType":"plus"}`, false, false},
		{"plan required", `{"type":"chatgpt","email":null}`, true, false},
		{"plan null", `{"type":"chatgpt","email":null,"planType":null}`, false, true},
		{"plan invalid", `{"type":"chatgpt","email":null,"planType":"future"}`, false, false},
		{"hidden plan invalid", `{"type":"chatgpt","email":null,"planType":"future","planType":"plus"}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			email := "old"
			before := p.ChatgptAccount{Type: "chatgpt", Email: &email, PlanType: p.PlanTypeFree}
			direct := before
			err := json.Unmarshal([]byte(tc.raw), &direct)
			if err == nil {
				t.Fatal("invalid account accepted")
			}
			if tc.missing && !errors.Is(err, p.ErrMissingResultField) {
				t.Fatalf("missing error = %v", err)
			}
			if tc.nullField && !errors.Is(err, p.ErrNullResultField) {
				t.Fatalf("null error = %v", err)
			}
			if !reflect.DeepEqual(direct, before) || direct.Email != before.Email || email != "old" {
				t.Fatal("failed decode changed receiver/reference")
			}
			prior := &p.ApiKeyAccount{}
			wrapper := p.AccountWrapper{Value: prior}
			if err := json.Unmarshal([]byte(tc.raw), &wrapper); err == nil || wrapper.Value != prior {
				t.Fatalf("failed wrapper changed: %+v, %v", wrapper, err)
			}
			mock := NewMockTransport()
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":true,"account":` + tc.raw + `}`)})
			client := p.NewClient(mock)
			got, err := client.Account.Get(context.Background(), p.GetAccountParams{})
			if err == nil || !reflect.DeepEqual(got, p.GetAccountResponse{}) {
				t.Fatalf("invalid public result = %+v, %v", got, err)
			}
			mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":{"type":"chatgpt","email":null,"planType":"plus"}}`)})
			recovered, err := client.Account.Get(context.Background(), p.GetAccountParams{})
			if err != nil || recovered.Account == nil {
				t.Fatalf("recovery = %+v, %v", recovered, err)
			}
			checkAccountEmailJSON(t, recovered.Account, "null")
		})
	}
}

func TestAccountEmailNativeErrorsAndOuterTraversal(t *testing.T) {
	// The public pointer representation and an independent ordinary wire record
	// must report the same native email kind failure, rather than hiding it.
	raw := []byte(`{"type":"chatgpt","email":7,"planType":"plus"}`)
	var ordinary struct {
		Type     string  `json:"type"`
		Email    *string `json:"email"`
		PlanType string  `json:"planType"`
	}
	referenceErr := json.Unmarshal(raw, &ordinary)
	var actual p.ChatgptAccount
	actualErr := json.Unmarshal(raw, &actual)
	var referenceType, actualType *json.UnmarshalTypeError
	if !errors.As(referenceErr, &referenceType) || !errors.As(actualErr, &actualType) {
		t.Fatalf("type errors = %v, %v", referenceErr, actualErr)
	}
	if actualType.Value != referenceType.Value || actualType.Type != referenceType.Type || actualType.Offset != referenceType.Offset || actualType.Field != referenceType.Field {
		t.Fatalf("native email error changed: %+v vs %+v", actualType, referenceType)
	}
	var envelope struct {
		Account p.ChatgptAccount `json:"account"`
		After   int              `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"account":{"type":"chatgpt","email":null,"planType":"future"},"after":9}`), &envelope); err == nil || envelope.After != 9 {
		t.Fatalf("outer traversal = %+v, %v", envelope, err)
	}
}
