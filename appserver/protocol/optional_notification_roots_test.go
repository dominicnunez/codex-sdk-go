package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"reflect"
	"testing"
)

func TestOptionalNotificationRootAdmission(t *testing.T) {
	for _, method := range []string{"skills/changed", "account/updated"} {
		for _, tc := range []struct {
			name, body string
			valid      bool
		}{
			{"empty-object", `{}`, true}, {"extra-object", `{"future":7}`, true},
			{"null", `null`, false}, {"array", `[]`, false}, {"bool", `false`, false},
			{"string", `"x"`, false}, {"number", `7`, false}, {"missing", ``, false},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				calls, diagnostics := 0, 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(m string, e error) {
					if m != method || e == nil {
						t.Error("bad diagnostic")
					}
					diagnostics++
				}))
				defer client.Close()
				if method == "skills/changed" {
					h := func(p.SkillsChangedNotification) { calls++ }
					client.OnSkillsChanged(h)
					client.AddSkillsChangedListener(h)
				} else {
					h := func(p.AccountUpdatedNotification) { calls++ }
					client.OnAccountUpdated(h)
					client.AddAccountUpdatedListener(h)
				}
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(tc.body)})
				if tc.valid {
					if calls != 2 || diagnostics != 0 {
						t.Fatalf("valid object: callbacks=%d diagnostics=%d", calls, diagnostics)
					}
				} else if calls != 0 || diagnostics != 2 {
					t.Fatalf("invalid root: callbacks=%d diagnostics=%d", calls, diagnostics)
				}
			})
		}
	}
}

func TestOptionalNotificationRootRecoveryAndRemoval(t *testing.T) {
	for _, method := range []string{"skills/changed", "account/updated"} {
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			on, add, raw, reports := 0, 0, 0, 0
			c := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, err error) {
				if origin != method || err == nil {
					t.Fatal("wrong diagnostic origin")
				}
				reports++
			}))
			t.Cleanup(func() { _ = c.Close() })
			var remove func()
			var clearTyped func()
			if method == "skills/changed" {
				c.OnSkillsChanged(func(p.SkillsChangedNotification) { on++ })
				remove = c.AddSkillsChangedListener(func(p.SkillsChangedNotification) { add++ })
				clearTyped = func() { c.OnSkillsChanged(nil); c.AddSkillsChangedListener(nil)() }
			} else {
				c.OnAccountUpdated(func(n p.AccountUpdatedNotification) { on++ })
				remove = c.AddAccountUpdatedListener(func(n p.AccountUpdatedNotification) { add++ })
				clearTyped = func() { c.OnAccountUpdated(nil); c.AddAccountUpdatedListener(nil)() }
			}
			removeRaw := c.AddNotificationListener(method, func(_ context.Context, n p.Notification) {
				raw++
				if len(n.Params) == 0 {
					t.Fatal("raw payload lost")
				}
			})
			defer removeRaw()
			send := func(body string) {
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(body)})
			}
			send(" \n null \t")
			if on != 0 || add != 0 || reports != 2 || raw != 1 {
				t.Fatalf("null publication on=%d add=%d reports=%d raw=%d", on, add, reports, raw)
			}
			for _, body := range []string{`{}`, `{"future":1e10000}`, `{"future":[null,{"nested":false}]}`, `{"authMode":null,"planType":null}`, `{"AUTHMODE":null,"PLANTYPE":null}`} {
				send(body)
			}
			if on != 5 || add != 5 || reports != 2 || raw != 6 {
				t.Fatal("valid object recovery or raw subscription changed")
			}
			remove()
			remove()
			send(`{}`)
			if on != 6 || add != 5 || reports != 2 || raw != 7 {
				t.Fatal("additive listener removal affected other listeners")
			}
			clearTyped()
			send(`null`)
			if on != 6 || add != 5 || reports != 2 || raw != 8 {
				t.Fatal("nil handlers retained typed admission or altered raw delivery")
			}
		})
	}
}

func TestOptionalNotificationRootRecordContracts(t *testing.T) {
	for _, value := range []any{p.SkillsChangedNotification{}, &p.SkillsChangedNotification{}} {
		if _, ok := value.(json.Unmarshaler); ok {
			t.Fatalf("empty skills record gained decoder: %T", value)
		}
	}
	var envelope struct {
		p.SkillsChangedNotification
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"Extra":"kept"}`), &envelope); err != nil || envelope.Extra != "kept" {
		t.Fatalf("anonymous skills composition changed: %v", err)
	}
	if err := json.Unmarshal([]byte(`null`), &envelope); err != nil || envelope.Extra != "kept" {
		t.Fatalf("ordinary JSON null behavior changed: %v", err)
	}
	mode, plan := p.AuthModeChatGPT, p.PlanTypePlus
	account := p.AccountUpdatedNotification{AuthMode: &mode, PlanType: &plan}
	before := account
	if err := account.UnmarshalJSON([]byte(`null`)); err == nil || account.AuthMode != before.AuthMode || account.PlanType != before.PlanType || !reflect.DeepEqual(account, before) {
		t.Fatal("null-root rejection changed prior account references")
	}
	for _, body := range []string{`{"authMode":null,"planType":null}`, `{"AUTHMODE":null,"PLANTYPE":null}`, `{}`} {
		if err := json.Unmarshal([]byte(body), &account); err != nil || account.AuthMode != nil || account.PlanType != nil {
			t.Fatalf("valid optional account fields rejected: %v", err)
		}
	}
	// Existing native error shapes for other non-object roots keep their native
	// destination context; adding a carrier-only skills check must not replace it.
	for _, body := range []string{`[]`, `false`, `"x"`, `7`, ``, `{"authMode":42}`} {
		type native p.AccountUpdatedNotification
		var expected native
		want := json.Unmarshal([]byte(body), &expected)
		if want == nil {
			t.Fatal("invalid native control")
		}
		var got p.AccountUpdatedNotification
		err := json.Unmarshal([]byte(body), &got)
		var wantType, gotType *json.UnmarshalTypeError
		if errors.As(want, &wantType) {
			if !errors.As(err, &gotType) {
				t.Fatalf("native type error lost: %v", err)
			}
			sameType := gotType.Type == wantType.Type
			if wantType.Type.Kind() == reflect.Struct {
				// The owner has always decoded through its private named wire type;
				// the independent native reference has the same layout under another name.
				sameType = gotType.Type.Name() == "wire" && gotType.Type.ConvertibleTo(wantType.Type)
			}
			if wantType.Value != gotType.Value || wantType.Offset != gotType.Offset || wantType.Field != gotType.Field || !sameType {
				t.Fatalf("native decoder context changed: %#v vs %#v", gotType, wantType)
			}
		} else if err == nil {
			t.Fatal("existing syntax failure lost")
		}
	}
}
