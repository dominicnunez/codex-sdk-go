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

func inputFlagQuestion(members string) string {
	return `{"header":"Header","id":"q","question":"Question"` + members + `}`
}
func inputFlagParams(question string) string {
	return `{"isBlocking":true,"itemId":"item","threadId":"thread","turnId":"turn","questions":[` + question + `]}`
}

func TestUserInputFlagsRejectNull(t *testing.T) {
	for _, field := range []string{"isOther", "isSecret"} {
		t.Run(field, func(t *testing.T) {
			question := inputFlagQuestion(fmt.Sprintf(`,%q:null`, field))
			var direct p.ToolRequestUserInputQuestion
			if err := json.Unmarshal([]byte(question), &direct); err == nil {
				t.Error("null flag accepted")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			calls := 0
			client.SetApprovalHandlers(p.ApprovalHandlers{OnToolRequestUserInput: func(context.Context, p.ToolRequestUserInputParams) (p.ToolRequestUserInputResponse, error) {
				calls++
				return p.ToolRequestUserInputResponse{Answers: map[string]p.ToolRequestUserInputAnswer{}}, nil
			}})
			response, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "input"}, Method: "item/tool/requestUserInput", Params: json.RawMessage(inputFlagParams(question))})
			if err == nil || calls != 0 || !reflect.DeepEqual(response, p.Response{}) {
				t.Errorf("malformed question reached handler: error=%v calls=%d response=%+v", err, calls, response)
			}
		})
	}
}

func TestUserInputFlagsValues(t *testing.T) {
	for _, tc := range []struct {
		members       string
		other, secret bool
	}{
		{``, false, false},
		{`,"isOther":false,"isSecret":false`, false, false},
		{`,"isOther":true,"isSecret":true`, true, true},
		{`,"isOther":true,"isOther":false,"isSecret":false,"isSecret":true`, false, true},
		{`,"isOther":false,"isOther":true,"isSecret":true,"isSecret":false`, true, false},
		{`,"\u0069sOther":true,"\u0069sSecret":true`, true, true},
		{`,"ISOTHER":true,"ISSECRET":true`, false, false},
		{`,"isOther":true,"isSecret":true,"ISOTHER":null,"ISSECRET":[]`, true, true},
		{`,"isOther":true,"options":null`, true, false},
		{`,"isSecret":true,"options":[]`, false, true},
	} {
		t.Run(tc.members, func(t *testing.T) {
			raw := inputFlagQuestion(tc.members)
			check := func(question p.ToolRequestUserInputQuestion) {
				t.Helper()
				if question.IsOther != tc.other || question.IsSecret != tc.secret {
					t.Fatalf("flag/default selection changed: %+v", question)
				}
			}
			var direct p.ToolRequestUserInputQuestion
			if err := json.Unmarshal([]byte(raw), &direct); err != nil {
				t.Fatal(err)
			}
			check(direct)
			encoded, err := json.Marshal(direct)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &direct); err != nil {
				t.Fatal(err)
			}
			check(direct)
			mock := NewMockTransport()
			client := p.NewClient(mock)
			calls := 0
			client.SetApprovalHandlers(p.ApprovalHandlers{OnToolRequestUserInput: func(_ context.Context, params p.ToolRequestUserInputParams) (p.ToolRequestUserInputResponse, error) {
				calls++
				if len(params.Questions) != 1 {
					t.Fatal("missing question")
				}
				check(params.Questions[0])
				return p.ToolRequestUserInputResponse{Answers: map[string]p.ToolRequestUserInputAnswer{}}, nil
			}})
			response, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "input"}, Method: "item/tool/requestUserInput", Params: json.RawMessage(inputFlagParams(raw))})
			if err != nil || calls != 1 || response.Error != nil || response.ID.Value != "input" {
				t.Fatalf("valid question failed approval dispatch: %v calls=%d response=%+v", err, calls, response)
			}
		})
	}
}

func TestUserInputFlagsAdmissionFailures(t *testing.T) {
	for _, field := range []string{"isOther", "isSecret"} {
		for _, members := range []string{
			fmt.Sprintf(`,%q:null`, field),
			fmt.Sprintf(`,%q:null,%q:false`, field, field),
			fmt.Sprintf(`,%q:true,%q:null`, field, field),
			fmt.Sprintf(`,"\u0069%s":null,%q:true`, field[1:], field),
			fmt.Sprintf(`,%q:1`, field),
			fmt.Sprintf(`,%q:"false"`, field),
			fmt.Sprintf(`,%q:{}`, field),
			fmt.Sprintf(`,%q:[]`, field),
		} {
			t.Run(field+members, func(t *testing.T) {
				question := inputFlagQuestion(members)
				for _, prefix := range []string{"", inputFlagQuestion("") + ","} {
					raw := inputFlagParams(prefix + question)
					var params p.ToolRequestUserInputParams
					if err := json.Unmarshal([]byte(raw), &params); err == nil {
						t.Fatal("invalid question accepted into params")
					}
					mock := NewMockTransport()
					client := p.NewClient(mock)
					calls := 0
					client.SetApprovalHandlers(p.ApprovalHandlers{OnToolRequestUserInput: func(context.Context, p.ToolRequestUserInputParams) (p.ToolRequestUserInputResponse, error) {
						calls++
						return p.ToolRequestUserInputResponse{Answers: map[string]p.ToolRequestUserInputAnswer{}}, nil
					}})
					request := p.Request{ID: p.RequestID{Value: "input"}, Method: "item/tool/requestUserInput", Params: json.RawMessage(raw)}
					response, err := mock.InjectServerRequest(context.Background(), request)
					if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(response, p.Response{}) {
						t.Fatalf("invalid question published: error=%v calls=%d response=%+v", err, calls, response)
					}
					request.Params = json.RawMessage(inputFlagParams(inputFlagQuestion(`,"isOther":true,"isSecret":true`)))
					response, err = mock.InjectServerRequest(context.Background(), request)
					if err != nil || calls != 1 || response.Error != nil {
						t.Fatalf("same-client recovery failed: %v calls=%d", err, calls)
					}
				}
			})
		}
	}
}

func TestUserInputFlagsReceiverAndPrecedence(t *testing.T) {
	var params p.ToolRequestUserInputParams
	seedQuestion := inputFlagQuestion(`,"isSecret":true,"isOther":true,"options":[{"label":"old","description":"old-description"}]`)
	if err := json.Unmarshal([]byte(inputFlagParams(seedQuestion)), &params); err != nil {
		t.Fatal(err)
	}
	before := params
	beforeQuestion := params.Questions[0]
	for _, field := range []string{"isSecret", "isOther"} {
		bad := inputFlagQuestion(fmt.Sprintf(`,%q:null`, field))
		question := beforeQuestion
		if err := json.Unmarshal([]byte(bad), &question); err == nil {
			t.Fatal("null question flag accepted")
		}
		if !reflect.DeepEqual(question, beforeQuestion) || question.Options != beforeQuestion.Options || &(*question.Options)[0] != &(*beforeQuestion.Options)[0] {
			t.Fatal("failure replaced direct question references")
		}
		if err := json.Unmarshal([]byte(inputFlagParams(bad)), &params); err == nil {
			t.Fatal("null flag accepted into params")
		}
		if !reflect.DeepEqual(params, before) || &params.Questions[0] != &before.Questions[0] || params.Questions[0].Options != beforeQuestion.Options {
			t.Fatal("failure replaced params references")
		}
		var envelope struct {
			Question p.ToolRequestUserInputQuestion `json:"question"`
			After    string                         `json:"after"`
		}
		envelope.Question = beforeQuestion
		if err := json.Unmarshal([]byte(`{"question":`+bad+`,"after":"visited"}`), &envelope); err == nil || envelope.After != "visited" || envelope.Question.Options != beforeQuestion.Options {
			t.Fatalf("enclosing traversal/reference changed: %v", err)
		}
	}
	for _, tc := range []struct {
		question, want string
		native         bool
	}{
		{`{"id":"q","question":"Q","isSecret":null}`, "header", false},
		{`{"header":1,"id":"q","question":"Q","isSecret":null}`, "cannot unmarshal number", true},
		{`{"header":"H","id":"","question":"Q","isSecret":null}`, "id", false},
		{inputFlagQuestion(`,"isSecret":null,"options":[{"label":1,"description":"D"}]`), "cannot unmarshal number", true},
	} {
		var question p.ToolRequestUserInputQuestion
		err := json.Unmarshal([]byte(tc.question), &question)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("older error lost precedence: %v", err)
		}
		if tc.native {
			var mismatch *json.UnmarshalTypeError
			if !errors.As(err, &mismatch) {
				t.Fatalf("native error replaced: %v", err)
			}
		}
	}
	if err := json.Unmarshal([]byte(inputFlagParams(inputFlagQuestion(""))), &params); err != nil || params.Questions[0].IsSecret || params.Questions[0].IsOther || params.Questions[0].Options != nil {
		t.Fatalf("reuse retained old values or lost defaults: %v", err)
	}
	if !beforeQuestion.IsOther || !beforeQuestion.IsSecret || (*beforeQuestion.Options)[0].Label != "old" {
		t.Fatal("success mutated earlier values/references")
	}
}
