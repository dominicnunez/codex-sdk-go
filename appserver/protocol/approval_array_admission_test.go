package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestCommandApprovalStringArrayAdmission(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name, field, base := "current", "proposedExecpolicyAmendment", `"itemId":"i","threadId":"t","turnId":"u","startedAtMs":0,"cwd":"relative/../evidence"`
		method := "item/commandExecution/requestApproval"
		if legacy {
			name = "legacy"
			field = "command"
			base = `"callId":"c","conversationId":"t","cwd":"relative/../evidence","parsedCmd":[]`
			method = "execCommandApproval"
		}
		for _, tc := range []struct {
			name, raw string
			valid     bool
		}{
			{"omitted", "", !legacy},
			{"wrong kind", `42`, false},
			{"later null", `["echo"],"` + field + `":null`, !legacy},
			{"earlier null", `null,"` + field + `":[]`, !legacy},
			{"null before reset", `[null],"` + field + `":null`, false},
			{"null among strings", `["echo",null,"arg"]`, false},
			{"valid duplicates", `["old"],"` + field + `":[]`, true},
			{"ignored alias", `["echo"],"` + strings.ToUpper(field) + `":[null]`, true},
			{"escaped null", `[],"\u` + fmt.Sprintf("%04x", field[0]) + field[1:] + `":[null]`, false},
			{"opaque strings", `["relative/../arg","","null","\u0061"]`, true},
			{"empty", `[]`, true}, {"empty-string", `[""]`, true}, {"values", `["echo","x"]`, true},
			{"null-array", `null`, !legacy}, {"null-item", `[null]`, false}, {"hidden-null-item", `[null],"` + field + `":["echo"]`, false},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				t.Cleanup(func() { _ = client.Close() })
				calls := 0
				var captured []string
				client.SetApprovalHandlers(p.ApprovalHandlers{
					OnCommandExecutionRequestApproval: func(_ context.Context, params p.CommandExecutionRequestApprovalParams) (p.CommandExecutionRequestApprovalResponse, error) {
						calls++
						if params.Cwd == nil || *params.Cwd != "relative/../evidence" {
							t.Fatal("approval cwd changed")
						}
						if params.ProposedExecpolicyAmendment != nil {
							captured = *params.ProposedExecpolicyAmendment
						}
						return p.CommandExecutionRequestApprovalResponse{Decision: p.CommandExecutionApprovalDecisionWrapper{Value: "decline"}}, nil
					},
					OnExecCommandApproval: func(_ context.Context, params p.ExecCommandApprovalParams) (p.ExecCommandApprovalResponse, error) {
						calls++
						if params.Cwd != "relative/../evidence" {
							t.Fatal("legacy cwd changed")
						}
						captured = params.Command
						return p.ExecCommandApprovalResponse{Decision: p.ReviewDecisionWrapper{Value: p.ReviewDecisionAbort}}, nil
					},
				})
				raw := `{` + base
				if tc.raw != "" {
					raw += `,"` + field + `":` + tc.raw
				}
				raw += `}`
				resp, err := mock.InjectServerRequest(context.Background(), p.Request{JSONRPC: "2.0", ID: p.RequestID{Value: 1}, Method: method, Params: json.RawMessage(raw)})
				if tc.valid {
					if calls != 1 || err != nil || resp.Error != nil {
						t.Fatalf("valid control calls=%d err=%v response=%+v", calls, err, resp)
					}
					var fields map[string]json.RawMessage
					if err = json.Unmarshal([]byte(raw), &fields); err != nil {
						t.Fatal(err)
					}
					var want []string
					if value, ok := fields[field]; ok {
						if err = json.Unmarshal(value, &want); err != nil {
							t.Fatal(err)
						}
					}
					if !reflect.DeepEqual(captured, want) {
						t.Fatalf("arguments changed: %#v vs %#v", captured, want)
					}
					if !reflect.DeepEqual(resp.ID, p.RequestID{Value: 1}) || len(resp.Result) == 0 {
						t.Fatal("invalid approval response")
					}
				} else {
					if calls != 0 {
						t.Fatalf("malformed approval reached application handler: calls=%d", calls)
					}
					if !errors.Is(err, p.ErrInvalidParams) {
						t.Fatalf("want invalid params, got %v", err)
					}
					if len(resp.Result) != 0 {
						t.Fatalf("published success response: %s", resp.Result)
					}
					// Rejection must not poison subsequent dispatch on the same client.
					req := p.Request{JSONRPC: "2.0", ID: p.RequestID{Value: 2}, Method: method, Params: json.RawMessage(`{` + base + `,"` + field + `":[]}`)}
					if _, err = mock.InjectServerRequest(context.Background(), req); err != nil || calls != 1 {
						t.Fatalf("recovery calls=%d err=%v", calls, err)
					}
				}
			})
		}
	}
}

func TestCommandApprovalArrayReceiverAndErrors(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		field, base := "proposedExecpolicyAmendment", `"itemId":"i","threadId":"t","turnId":"u","startedAtMs":0`
		if legacy {
			field = "command"
			base = `"callId":"c","conversationId":"t","cwd":"relative/../evidence","parsedCmd":[]`
		}
		prior := []string{"preserved"}
		current := p.CommandExecutionRequestApprovalParams{ItemID: "prior", ProposedExecpolicyAmendment: &prior}
		old := p.ExecCommandApprovalParams{CallID: "prior", Command: prior}
		beforeCurrent, beforeOld := current, old
		decode := current.UnmarshalJSON
		if legacy {
			decode = old.UnmarshalJSON
		}
		bad := []byte(`{` + base + `,"` + field + `":[null],"` + field + `":["valid"]}`)
		if err := decode(bad); err == nil {
			t.Fatal("null occurrence accepted")
		}
		if !reflect.DeepEqual(current, beforeCurrent) || !reflect.DeepEqual(old, beforeOld) || prior[0] != "preserved" {
			t.Fatal("failed decode changed receiver/references")
		}
		if current.ProposedExecpolicyAmendment != beforeCurrent.ProposedExecpolicyAmendment || &old.Command[0] != &prior[0] {
			t.Fatal("failed decode replaced prior references")
		}

		// Earlier owner checks must retain their priority over the new null guard.
		invalids := []string{strings.Replace(base, `"i"`, `""`, 1), base + `,"startedAtMs":"bad"`, base + `,"proposedNetworkPolicyAmendments":[{"action":"bad","host":"site"}]`}
		if legacy {
			invalids = []string{strings.Replace(base, `"c"`, `""`, 1), base + `,"parsedCmd":42`, strings.Replace(base, `"callId":"c",`, "", 1)}
		}
		for _, prefix := range invalids {
			controlErr := decode([]byte(`{` + prefix + `,"` + field + `":[]}`))
			mixedErr := decode([]byte(`{` + prefix + `,"` + field + `":[null]}`))
			if controlErr == nil || mixedErr == nil || controlErr.Error() != mixedErr.Error() {
				t.Fatalf("prior error changed: %v vs %v", controlErr, mixedErr)
			}
		}
	}
	// Standard JSON may continue after a nested decoder error. Its enclosing
	// first-error selection and later field writes remain part of the contract.
	for _, legacy := range []bool{false, true} {
		if legacy {
			value := struct {
				Before int
				Params p.ExecCommandApprovalParams
				After  int
			}{}
			err := json.Unmarshal([]byte(`{"Before":"bad","Params":{"callId":"c","conversationId":"t","cwd":"","parsedCmd":[],"command":[null]},"After":7}`), &value)
			var native *json.UnmarshalTypeError
			if !errors.As(err, &native) || native.Field != "Before" || value.After != 7 {
				t.Fatalf("legacy enclosing traversal: %+v %v", value, err)
			}
		} else {
			value := struct {
				Before int
				Params p.CommandExecutionRequestApprovalParams
				After  int
			}{}
			err := json.Unmarshal([]byte(`{"Before":"bad","Params":{"itemId":"i","threadId":"t","turnId":"u","startedAtMs":0,"proposedExecpolicyAmendment":[null]},"After":7}`), &value)
			var native *json.UnmarshalTypeError
			if !errors.As(err, &native) || native.Field != "Before" || value.After != 7 {
				t.Fatalf("current enclosing traversal: %+v %v", value, err)
			}
		}
	}
}
