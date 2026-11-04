package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestModelProviderListAdmission(t *testing.T) {
	for _, settings := range []string{`{"type":"openai"}`, `{"type":"customGateway","auth":{"mode":"environmentVariable","envKey":"KEY"}}`, `{"type":"bedrock","region":null}`, `{"type":"gcp","credentialFile":"relative/provider.json"}`} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/list", p.Response{Result: json.RawMessage(`{"canChangeDefault":false,"defaultProviderId":"p","data":[{"id":"p","name":"provider","canEdit":false,"settings":` + settings + `}]}`)})
		resp, err := p.NewClient(mock).ModelProvider.List(context.Background(), p.ModelProviderListParams{})
		if err != nil || len(resp.Data) != 1 || resp.DefaultProviderID != "p" {
			t.Fatalf("settings %s: response=%+v error=%v", settings, resp, err)
		}
		if req := mock.GetSentRequest(0); req.Method != "modelProvider/list" || string(req.Params) != "{}" {
			t.Fatalf("request=%+v", req)
		}
	}
}

func TestModelProviderListRejectsMalformed(t *testing.T) {
	for _, body := range []string{`{}`, `{"canChangeDefault":false,"defaultProviderId":"p","data":null}`, `{"canChangeDefault":false,"defaultProviderId":"p","data":[null]}`, `{"canChangeDefault":false,"defaultProviderId":"p","data":[{"id":"p","name":"P","canEdit":false,"settings":{"type":"customGateway","auth":{"mode":"environmentVariable"}}}]}`} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/list", p.Response{Result: json.RawMessage(body)})
		resp, err := p.NewClient(mock).ModelProvider.List(context.Background(), p.ModelProviderListParams{})
		if err == nil || resp.Data != nil || resp.DefaultProviderID != "" {
			t.Fatalf("published malformed %s: %+v %v", body, resp, err)
		}
	}
}

func TestModelProviderConfigWrite(t *testing.T) {
	for _, patch := range []string{
		`{"type":"customGateway"}`, `{"type":"customGateway","baseUrl":null,"envKey":"","name":"","providerId":null}`,
		`{"type":"bedrock","providerId":"amazon-bedrock","profile":null,"region":""}`,
		`{"type":"bedrock","providerId":"amazon-bedrock-runtime"}`,
		`{"type":"gcp","providerId":"gcp","credentialFile":"relative/credentials.json","location":null,"projectId":""}`,
	} {
		t.Run(patch, func(t *testing.T) {
			var params p.ModelProviderConfigWriteParams
			if err := json.Unmarshal([]byte(`{"settings":`+patch+`}`), &params); err != nil {
				t.Fatal(err)
			}
			mock := NewMockTransport()
			mock.SetResponse("modelProvider/config/write", p.Response{Result: json.RawMessage(`{"provider":{"id":"new","name":"P","canEdit":true,"settings":{"type":"openai"}}}`)})
			resp, err := p.NewClient(mock).ModelProvider.ConfigWrite(context.Background(), params)
			if err != nil || resp.Provider.ID != "new" {
				t.Fatalf("response=%+v err=%v", resp, err)
			}
			var got, want any
			if err := json.Unmarshal(mock.GetSentRequest(0).Params, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"settings":`+patch+`}`), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("request=%v want %v", got, want)
			}
		})
	}
}

func TestModelProviderPatchRejectsMalformed(t *testing.T) {
	for _, input := range []string{
		`null`, `{}`, `{"settings":null}`, `{"settings":{"type":"openai"}}`,
		`{"settings":{"type":"customGateway","name":null}}`,
		`{"settings":{"type":"customGateway","name":null,"name":"ok"}}`,
		`{"settings":{"type":"customGateway","region":"x"}}`,
		`{"settings":{"type":"bedrock","providerId":"gcp"}}`,
		`{"settings":{"type":"bedrock"}}`,
		`{"settings":{"type":"gcp","providerId":null}}`,
		`{"settings":{"type":"gcp","providerId":"gcp","credentialFile":12}}`,
		`{"settings":{"type":"gcp","providerId":"gcp","credentialFile":12,"credentialFile":"ok"}}`,
		`{"settings":{"type":"gcp","providerId":"gcp"},"extra":null}`,
		`{"settings":{"type":"customGateway","NaMe":"x"}}`,
		`{"settings":{"type":"customGateway","\u0065xtra":true}}`,
	} {
		var params p.ModelProviderConfigWriteParams
		if err := json.Unmarshal([]byte(input), &params); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	mock := NewMockTransport()
	_, err := p.NewClient(mock).ModelProvider.ConfigWrite(context.Background(), p.ModelProviderConfigWriteParams{})
	if err == nil || mock.GetSentRequest(0) != nil {
		t.Fatalf("invalid request sent: err=%v request=%+v", err, mock.GetSentRequest(0))
	}
}

func TestModelProviderUnionAdmission(t *testing.T) {
	for _, input := range []string{
		`null`, `{}`, `{"type":null}`, `{"type":"future"}`, `{"type":"future","type":"openai"}`,
		`{"type":"customGateway"}`, `{"type":"customGateway","auth":null}`,
		`{"type":"customGateway","auth":{"mode":"environmentVariable","envKey":null}}`,
		`{"type":"customGateway","auth":{"mode":"none","mode":null}}`,
		`{"type":"customGateway","auth":{"mode":"future","mode":"none"}}`,
		`{"type":"bedrock","region":5,"region":"ok"}`,
	} {
		var value p.ProviderSettingsWrapper
		if err := json.Unmarshal([]byte(input), &value); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	// Inactive fields are permitted additional properties; they cannot narrow another branch.
	for _, input := range []string{`{"type":"openai","auth":4}`, `{"type":"bedrock","auth":4}`, `{"type":"customGateway","auth":{"mode":"none","envKey":4}}`, `{"type":"customGateway","auth":{"mode":"externallyConfigured","envKey":null}}`} {
		var value p.ProviderSettingsWrapper
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			t.Errorf("rejected %s: %v", input, err)
		}
	}
}

func TestModelProviderResponseRequiredFields(t *testing.T) {
	valid := `{"id":"p","name":"P","canEdit":false,"settings":{"type":"openai"}}`
	for _, field := range []string{"id", "name", "canEdit", "settings"} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(valid), &fields); err != nil {
			t.Fatal(err)
		}
		for _, null := range []bool{false, true} {
			clone := make(map[string]json.RawMessage)
			for k, v := range fields {
				clone[k] = v
			}
			if null {
				clone[field] = json.RawMessage(`null`)
			} else {
				delete(clone, field)
			}
			raw, err := json.Marshal(clone)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"modelProvider/list", "modelProvider/config/write"} {
				mock := NewMockTransport()
				body := `{"provider":` + string(raw) + `}`
				if method == "modelProvider/list" {
					body = `{"data":[` + string(raw) + `],"canChangeDefault":true,"defaultProviderId":"p"}`
				}
				mock.SetResponse(method, p.Response{Result: json.RawMessage(body)})
				c := p.NewClient(mock)
				if method == "modelProvider/list" {
					resp, err := c.ModelProvider.List(context.Background(), p.ModelProviderListParams{})
					if err == nil || resp.Data != nil {
						t.Fatalf("published %s: %+v %v", body, resp, err)
					}
				} else {
					resp, err := c.ModelProvider.ConfigWrite(context.Background(), p.ModelProviderConfigWriteParams{Settings: p.ProviderSettingsPatchWrapper{Value: p.CustomGatewayProviderSettingsPatch{}}})
					if err == nil || resp.Provider.ID != "" {
						t.Fatalf("published %s: %+v %v", body, resp, err)
					}
				}
			}
		}
	}
}

func TestModelProviderErrorRawFallback(t *testing.T) {
	for _, data := range []string{`{"type":"modelProviderMismatch","modelProvider":"a","currentModelProvider":"b"}`, `{"type":"modelProviderNotConfigured","modelProvider":"a"}`, `{"type":"modelProviderPolicyConflict","modelProvider":"a","requiredModelProvider":"b"}`, `{"type":"future","private":"opaque"}`} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/list", p.Response{Error: &p.Error{Code: -32000, Message: "server error", Data: json.RawMessage(data)}})
		_, err := p.NewClient(mock).ModelProvider.List(context.Background(), p.ModelProviderListParams{})
		var rpc *p.RPCError
		if !errors.As(err, &rpc) || string(rpc.Data()) != data {
			t.Fatalf("lost raw fallback: %v", err)
		}
		var typed p.ModelProviderErrorWrapper
		decodeErr := json.Unmarshal(rpc.Data(), &typed)
		if strings.Contains(data, `"future"`) {
			if decodeErr == nil {
				t.Fatal("accepted unknown structured error")
			}
		} else if decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	for _, raw := range []string{`null`, `{}`, `{"type":"modelProviderMismatch","modelProvider":"a"}`, `{"type":"modelProviderPolicyConflict","modelProvider":"a","requiredModelProvider":null}`} {
		var value p.ModelProviderErrorWrapper
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestModelProviderUnionConstructedPointers(t *testing.T) {
	values := []p.ProviderSettings{p.OpenaiProviderSettings{}, &p.OpenaiProviderSettings{}, p.CustomGatewayProviderSettings{Auth: p.GatewayAuthWrapper{Value: p.NoneGatewayAuth{}}}, &p.BedrockProviderSettings{}, &p.GcpProviderSettings{}}
	for _, value := range values {
		raw, err := json.Marshal(p.ProviderSettingsWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var decoded p.ProviderSettingsWrapper
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	var nilValue *p.GcpProviderSettings
	if _, err := json.Marshal(p.ProviderSettingsWrapper{Value: nilValue}); err == nil {
		t.Fatal("accepted typed nil")
	}
	var envelope struct {
		p.ModelProviderSummary
		Extra string `json:"extra"`
	}
	envelope.Extra = "keep"
	envelope.Settings = p.ProviderSettingsWrapper{Value: p.OpenaiProviderSettings{}}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"extra":"keep"`) {
		t.Fatal("summary consumed application envelope")
	}
}

func FuzzModelProviderOpaquePatch(f *testing.F) {
	for _, seed := range []string{"", "relative/credentials.json", "https://user@example.invalid/path", "\n\t\"\\", "../../opaque"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) {
			t.Skip()
		}
		params := p.ModelProviderConfigWriteParams{Settings: p.ProviderSettingsPatchWrapper{Value: p.GcpProviderSettingsPatch{ProviderID: p.GcpProviderIDGcp, CredentialFile: p.OptionalNullable[string]{Present: true, Value: &value}}}}
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		var decoded p.ModelProviderConfigWriteParams
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		got, ok := decoded.Settings.Value.(p.GcpProviderSettingsPatch)
		if !ok || !got.CredentialFile.Present || got.CredentialFile.Value == nil || *got.CredentialFile.Value != value {
			t.Fatalf("opaque credential path changed: %+v", decoded)
		}
	})
}

func TestModelProviderDiagnosticsBounded(t *testing.T) {
	raw := []byte(`{"type":` + strconv.Quote(strings.Repeat("\x00", 1<<18)) + `}`)
	var value p.ProviderSettingsWrapper
	err := json.Unmarshal(raw, &value)
	if err == nil || len(err.Error()) > 4096 {
		t.Fatalf("unbounded invalid discriminator diagnostic length: %v", err)
	}
	// Failed admission leaves previously owned response and union values untouched.
	value.Value = p.OpenaiProviderSettings{}
	if err := json.Unmarshal([]byte(`{"type":"customGateway","auth":null}`), &value); err == nil {
		t.Fatal("accepted malformed auth")
	}
	if _, ok := value.Value.(p.OpenaiProviderSettings); !ok {
		t.Fatalf("failed decode changed receiver: %+v", value)
	}
}

type providerExtension struct {
	p.OpenaiProviderSettings
	Type      string      `json:"type"`
	Extension json.Number `json:"extension"`
}
type providerCodecError struct {
	p.OpenaiProviderSettings
	failure error
}

func (v providerCodecError) MarshalJSON() ([]byte, error) { return nil, v.failure }

func TestModelProviderExtensionEncoding(t *testing.T) {
	value := p.ProviderSettingsWrapper{Value: providerExtension{Type: "openai", Extension: json.Number("42")}}
	raw, err := json.Marshal(value)
	if err != nil || !strings.Contains(string(raw), `"extension":42`) {
		t.Fatalf("extension lost: %s %v", raw, err)
	}
	value.Value = providerExtension{Type: "openai", Extension: json.Number(strings.Repeat("x", 1<<18))}
	if _, err := json.Marshal(value); err == nil || len(err.Error()) > 4096 {
		t.Fatal("native Number diagnostic was not bounded")
	}
	sentinel := errors.New("application codec error")
	value.Value = providerCodecError{failure: sentinel}
	if _, err := json.Marshal(value); !errors.Is(err, sentinel) {
		t.Fatalf("application codec error identity lost: %v", err)
	}
}

func TestModelProviderOtherConstructedUnions(t *testing.T) {
	for _, value := range []p.GatewayAuth{p.NoneGatewayAuth{}, &p.NoneGatewayAuth{}, p.EnvironmentVariableGatewayAuth{EnvKey: ""}, &p.EnvironmentVariableGatewayAuth{EnvKey: "KEY"}, p.ModeGatewayAuth{}, &p.ModeGatewayAuth{}} {
		raw, err := json.Marshal(p.GatewayAuthWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var decoded p.GatewayAuthWrapper
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []p.ProviderSettingsPatch{p.CustomGatewayProviderSettingsPatch{}, &p.CustomGatewayProviderSettingsPatch{}, p.BedrockProviderSettingsPatch{ProviderID: p.BedrockProviderIDAmazonBedrock}, &p.BedrockProviderSettingsPatch{ProviderID: p.BedrockProviderIDAmazonBedrockRuntime}, p.GcpProviderSettingsPatch{ProviderID: p.GcpProviderIDGcp}, &p.GcpProviderSettingsPatch{ProviderID: p.GcpProviderIDGcp}} {
		raw, err := json.Marshal(p.ProviderSettingsPatchWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var decoded p.ProviderSettingsPatchWrapper
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []p.ModelProviderError{p.ModelProviderMismatchError{}, &p.ModelProviderMismatchError{}, p.ModelProviderNotConfiguredError{}, &p.ModelProviderNotConfiguredError{}, p.ModelProviderPolicyConflictError{}, &p.ModelProviderPolicyConflictError{}} {
		raw, err := json.Marshal(p.ModelProviderErrorWrapper{Value: value})
		if err != nil {
			t.Fatal(err)
		}
		var decoded p.ModelProviderErrorWrapper
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
	}
	var auth *p.NoneGatewayAuth
	var patch *p.CustomGatewayProviderSettingsPatch
	var rpc *p.ModelProviderMismatchError
	for _, value := range []any{p.GatewayAuthWrapper{Value: auth}, p.ProviderSettingsPatchWrapper{Value: patch}, p.ModelProviderErrorWrapper{Value: rpc}} {
		if _, err := json.Marshal(value); err == nil {
			t.Errorf("accepted typed nil %T", value)
		}
	}
}

func TestModelProviderExactNamesAndMerges(t *testing.T) {
	// Differently cased response extras are unconstrained and cannot select a branch.
	for _, settings := range []string{`{"TYPE":null,"type":"openai"}`, `{"type":"openai","TYPE":"future"}`, `{"type":"customGateway","auth":{"MODE":null,"mode":"none"}}`} {
		var value p.ProviderSettingsWrapper
		if err := json.Unmarshal([]byte(settings), &value); err != nil {
			t.Errorf("rejected uppercase extra %s: %v", settings, err)
		}
	}
	for _, settings := range []string{`{"TYPE":"openai"}`, `{"type":"future","type":"openai"}`, `{"\u0074ype":null,"type":"openai"}`} {
		var value p.ProviderSettingsWrapper
		if err := json.Unmarshal([]byte(settings), &value); err == nil {
			t.Errorf("accepted %s", settings)
		}
	}
	summaries := `{"id":"p","ID":"untrusted-extra","name":"P","NAME":null,"canEdit":false,"settings":{"type":"openai"}}`
	for _, body := range []string{
		`{"data":[` + summaries + `],"DATA":[null],"canChangeDefault":false,"defaultProviderId":"p"}`,
		`{"data":[{"id":"p"}],"data":[{"name":"P","canEdit":false,"settings":{"type":"openai"}}],"canChangeDefault":false,"defaultProviderId":"p"}`,
	} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/list", p.Response{Result: json.RawMessage(body)})
		response, err := p.NewClient(mock).ModelProvider.List(context.Background(), p.ModelProviderListParams{})
		if err != nil || len(response.Data) != 1 || response.Data[0].ID != "p" {
			t.Fatalf("response=%+v err=%v body=%s", response, err, body)
		}
	}
	for _, body := range []string{
		`{"provider":` + summaries + `,"PROVIDER":null}`,
		`{"provider":{"id":"p"},"provider":{"name":"P","canEdit":false,"settings":{"type":"openai"}}}`,
	} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/config/write", p.Response{Result: json.RawMessage(body)})
		response, err := p.NewClient(mock).ModelProvider.ConfigWrite(context.Background(), p.ModelProviderConfigWriteParams{Settings: p.ProviderSettingsPatchWrapper{Value: p.CustomGatewayProviderSettingsPatch{}}})
		if err != nil || response.Provider.ID != "p" {
			t.Fatalf("response=%+v err=%v body=%s", response, err, body)
		}
	}
	for _, body := range []string{
		`{"data":[` + summaries + `],"data":[],"data":[{}],"canChangeDefault":false,"defaultProviderId":"p"}`,
		`{"data":[{"id":null}],"data":[` + summaries + `],"canChangeDefault":false,"defaultProviderId":"p"}`,
	} {
		mock := NewMockTransport()
		mock.SetResponse("modelProvider/list", p.Response{Result: json.RawMessage(body)})
		response, err := p.NewClient(mock).ModelProvider.List(context.Background(), p.ModelProviderListParams{})
		if err == nil || response.Data != nil {
			t.Fatalf("published invalid merge: %+v %v", response, err)
		}
	}
}
