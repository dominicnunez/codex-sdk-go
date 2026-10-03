package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type numberLiteralUnionData struct {
	Type  string      `json:"type"`
	N     json.Number `json:"n"`
	Codec any         `json:"codec,omitempty"`
}

// Embedding the exported interface promotes its private marker method without
// adding a JSON codec. These values therefore reach the SDK's native encoder.
type numberLiteralAccount struct {
	protocol.Account `json:"-"`
	numberLiteralUnionData
}
type numberLiteralCommandAction struct {
	protocol.CommandAction `json:"-"`
	numberLiteralUnionData
}
type numberLiteralParsedCommand struct {
	protocol.ParsedCommand `json:"-"`
	numberLiteralUnionData
}
type numberLiteralFileChange struct {
	protocol.FileChange `json:"-"`
	numberLiteralUnionData
}
type numberLiteralConfigLayer struct {
	protocol.ConfigLayerSource `json:"-"`
	numberLiteralUnionData
}
type numberLiteralPatch struct {
	protocol.PatchChangeKind `json:"-"`
	numberLiteralUnionData
}
type numberLiteralWebSearch struct {
	protocol.WebSearchAction `json:"-"`
	numberLiteralUnionData
}
type numberLiteralDynamicOutput struct {
	protocol.DynamicToolCallOutputContentItem `json:"-"`
	numberLiteralUnionData
}
type numberLiteralFunctionOutput struct {
	protocol.FunctionCallOutputContentItem `json:"-"`
	numberLiteralUnionData
}
type numberLiteralRealtimeItem struct {
	protocol.ThreadRealtimeItem `json:"-"`
	numberLiteralUnionData
}
type numberLiteralPresentation struct {
	protocol.ThreadRealtimeBemItemPresentation `json:"-"`
	numberLiteralUnionData
}
type numberLiteralThreadItem struct {
	protocol.ThreadItem `json:"-"`
	numberLiteralUnionData
}
type numberLiteralPrediction struct {
	protocol.ThreadPredictionResult `json:"-"`
	numberLiteralUnionData
}
type numberLiteralReview struct {
	protocol.ReviewTarget `json:"-"`
	numberLiteralUnionData
}
type numberLiteralUserInput struct {
	protocol.UserInput `json:"-"`
	numberLiteralUnionData
}
type numberLiteralSchedule struct {
	protocol.ScheduledTaskSchedule `json:"-"`
	numberLiteralUnionData
}
type numberLiteralSubAgent struct {
	protocol.SubAgentSource `json:"-"`
	numberLiteralUnionData
}

func TestMalformedNumberEmbeddedUnionOwners(t *testing.T) {
	factories := map[string]func(numberLiteralUnionData) any{
		"nested schedule": func(p numberLiteralUnionData) any {
			return protocol.ScheduledTaskSummary{Schedule: numberLiteralSchedule{nil, p}}
		},
		"nested subagent": func(p numberLiteralUnionData) any {
			return protocol.SessionSourceWrapper{Value: protocol.SessionSourceSubAgent{SubAgent: numberLiteralSubAgent{nil, p}}}
		},
		"account": func(p numberLiteralUnionData) any {
			return &protocol.AccountWrapper{Value: numberLiteralAccount{nil, p}}
		},
		"command": func(p numberLiteralUnionData) any {
			return protocol.CommandActionWrapper{Value: numberLiteralCommandAction{nil, p}}
		},
		"parsed command": func(p numberLiteralUnionData) any {
			return protocol.ParsedCommandWrapper{Value: numberLiteralParsedCommand{nil, p}}
		},
		"file change": func(p numberLiteralUnionData) any {
			return protocol.FileChangeWrapper{Value: numberLiteralFileChange{nil, p}}
		},
		"config layer": func(p numberLiteralUnionData) any {
			return protocol.ConfigLayerSourceWrapper{Value: numberLiteralConfigLayer{nil, p}}
		},
		"patch": func(p numberLiteralUnionData) any {
			return protocol.PatchChangeKindWrapper{Value: numberLiteralPatch{nil, p}}
		},
		"web search": func(p numberLiteralUnionData) any {
			return protocol.WebSearchActionWrapper{Value: numberLiteralWebSearch{nil, p}}
		},
		"dynamic output": func(p numberLiteralUnionData) any {
			return protocol.DynamicToolCallOutputContentItemWrapper{Value: numberLiteralDynamicOutput{nil, p}}
		},
		"function output": func(p numberLiteralUnionData) any {
			return protocol.FunctionCallOutputContentItemWrapper{Value: numberLiteralFunctionOutput{nil, p}}
		},
		"realtime item": func(p numberLiteralUnionData) any {
			return protocol.ThreadRealtimeItemWrapper{Value: numberLiteralRealtimeItem{nil, p}}
		},
		"presentation": func(p numberLiteralUnionData) any {
			return protocol.ThreadRealtimeBemItemPresentationWrapper{Value: numberLiteralPresentation{nil, p}}
		},
		"thread item": func(p numberLiteralUnionData) any {
			return protocol.ThreadItemWrapper{Value: numberLiteralThreadItem{nil, p}}
		},
		"prediction": func(p numberLiteralUnionData) any {
			return protocol.ThreadPredictionResultWrapper{Value: numberLiteralPrediction{nil, p}}
		},
		"review": func(p numberLiteralUnionData) any {
			return protocol.ReviewTargetWrapper{Value: numberLiteralReview{nil, p}}
		},
		"user message": func(p numberLiteralUnionData) any {
			return &protocol.UserMessageThreadItem{Content: []protocol.UserInput{numberLiteralUserInput{nil, p}}}
		},
	}
	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			p := numberLiteralUnionData{Type: "custom", N: json.Number(strings.Repeat("x", 1<<20))}
			_, err := json.Marshal(factory(p))
			if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
				t.Fatal("embedded union owner retains native diagnostic")
			}
			for current := err; current != nil; current = errors.Unwrap(current) {
				if len(current.Error()) > 4096 {
					t.Fatal("native diagnostic retained in chain")
				}
			}
			p.N = "01"
			_, short := json.Marshal(factory(p))
			_, reference := json.Marshal(p)
			if short == nil || reference == nil || errors.Unwrap(short).Error() != reference.Error() {
				t.Fatal("short native error changed")
			}
			p.N = json.Number(strings.Repeat("9", 1<<16))
			data, err := json.Marshal(factory(p))
			if err != nil || !strings.Contains(string(data), string(p.N)) {
				t.Fatal("valid long union number changed")
			}
			p.N = "0"
			p.Codec = numberLiteralApplicationMarshaler{reference}
			_, err = json.Marshal(factory(p))
			if !errors.Is(err, reference) {
				t.Fatal("application error identity lost through union owner")
			}
		})
	}
}

func TestMalformedNumberReviewRequestPreparation(t *testing.T) {
	mock := NewMockTransport()
	client := protocol.NewClient(mock)
	defer client.Close()
	payload := numberLiteralUnionData{Type: "custom", N: json.Number(strings.Repeat("x", 1<<20))}
	params := protocol.ReviewStartParams{ThreadID: "t", Target: protocol.ReviewTargetWrapper{Value: numberLiteralReview{nil, payload}}}
	_, err := client.Review.Start(context.Background(), params)
	if err == nil || len(err.Error()) > 4096 || mock.GetSentRequest(0) != nil {
		t.Fatal("review preparation failure must be bounded before Send")
	}
	mock.SetResponse("review/start", protocol.Response{Result: json.RawMessage(`{"reviewThreadId":"t","turn":{"id":"u","items":[],"status":"completed","error":null}}`)})
	payload.N = json.Number(strings.Repeat("9", 1<<16))
	params.Target.Value = numberLiteralReview{nil, payload}
	if _, err := client.Review.Start(context.Background(), params); err != nil {
		t.Fatalf("review did not recover: %v", err)
	}
	request := mock.GetSentRequest(0)
	if request == nil || !strings.Contains(string(request.Params), string(payload.N)) {
		t.Fatal("review valid long numeric payload was altered")
	}
}

func TestMalformedNumberElicitationRequestOwners(t *testing.T) {
	for _, mode := range []protocol.McpServerElicitationMode{protocol.McpServerElicitationModeForm, protocol.McpServerElicitationModeURL, protocol.McpServerElicitationModeOpenAIForm, protocol.McpServerElicitationModeOpenAIFormLegacy} {
		for _, schema := range []bool{false, true} {
			t.Run(string(mode)+map[bool]string{false: "/meta", true: "/schema"}[schema], func(t *testing.T) {
				object := map[string]any{"n": json.Number(strings.Repeat("x", 1<<20))}
				p := protocol.McpServerElicitationRequestParams{Mode: mode, OpenAIRequestedSchema: json.RawMessage(`{}`)}
				if schema {
					p.RequestedSchema = &protocol.McpElicitationSchema{Properties: object}
				} else {
					p.Meta = object
				}
				// OpenAI's raw schema overrides the typed schema. An invalid Number
				// in that discarded typed value must remain irrelevant.
				ignored := schema && (mode == protocol.McpServerElicitationModeOpenAIForm || mode == protocol.McpServerElicitationModeOpenAIFormLegacy)
				_, err := json.Marshal(struct {
					Params protocol.McpServerElicitationRequestParams
				}{p})
				if ignored {
					if err != nil {
						t.Fatal("overridden schema was serialized")
					}
					return
				}
				if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
					t.Fatal("elicitation inner native diagnostic unbounded")
				}
				object["n"] = json.Number("01")
				_, err = json.Marshal(p)
				_, reference := json.Marshal(object)
				if err == nil || errors.Unwrap(err).Error() != reference.Error() {
					t.Fatal("short elicitation error changed")
				}
				object["n"] = json.Number(strings.Repeat("9", 1<<16))
				data, err := json.Marshal(p)
				if err != nil || !strings.Contains(string(data), object["n"].(json.Number).String()) {
					t.Fatal("valid elicitation number changed")
				}
				object["n"] = numberLiteralApplicationMarshaler{reference}
				_, err = json.Marshal(p)
				if !errors.Is(err, reference) {
					t.Fatal("application elicitation error identity lost")
				}
			})
		}
	}
}
