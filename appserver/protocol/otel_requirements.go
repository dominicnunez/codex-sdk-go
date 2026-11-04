package protocol

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonobject"
)

// OtelRequirements describes managed telemetry settings; it does not configure an exporter.
type OtelRequirements struct {
	LogSseDeltas           *bool                         `json:"log_sse_deltas,omitempty"`
	Environment            *string                       `json:"environment,omitempty"`
	Exporter               *OtelExporter                 `json:"exporter,omitempty"`
	MetricsExporter        *OtelExporter                 `json:"metrics_exporter,omitempty"`
	TraceExporter          *OtelExporter                 `json:"trace_exporter,omitempty"`
	LogAgentResponses      *bool                         `json:"log_agent_responses,omitempty"`
	LogGuardianAssessments *bool                         `json:"log_guardian_assessments,omitempty"`
	LogUserPrompt          *bool                         `json:"log_user_prompt,omitempty"`
	SpanAttributes         *map[string]string            `json:"span_attributes,omitempty"`
	Tracestate             *map[string]map[string]string `json:"tracestate,omitempty"`
	ToolResult             *OtelToolResultLogConfig      `json:"tool_result,omitempty"`
}

func (o *OtelRequirements) UnmarshalJSON(data []byte) error {
	type wire OtelRequirements
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateOptionalFields(data, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		return validateOtelStringMap(raw, false)
	}, "span_attributes"); err != nil {
		return err
	}
	if err := validateOptionalFields(data, func(_ string, raw []byte) error {
		if isNullJSONValue(raw) {
			return nil
		}
		return validateOtelStringMap(raw, true)
	}, "tracestate"); err != nil {
		return err
	}
	*o = OtelRequirements(decoded)
	return nil
}

// OtelExporter selects exactly one of Kind (none/statsig), HTTP or GRPC.
type OtelExporter struct {
	Kind string            `json:"-"`
	HTTP *OtelHTTPExporter `json:"-"`
	GRPC *OtelGRPCExporter `json:"-"`
}

type OtelHttpProtocol string

const (
	OtelHttpProtocolBinary OtelHttpProtocol = "binary"
	OtelHttpProtocolJSON   OtelHttpProtocol = "json"
)

var validOtelHttpProtocols = map[OtelHttpProtocol]struct{}{OtelHttpProtocolBinary: {}, OtelHttpProtocolJSON: {}}

func (p *OtelHttpProtocol) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "otel.protocol", validOtelHttpProtocols, p)
}
func (p OtelHttpProtocol) MarshalJSON() ([]byte, error) {
	return marshalEnumString("otel.protocol", p, validOtelHttpProtocols)
}

type OtelHTTPExporter struct {
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	Protocol OtelHttpProtocol  `json:"protocol"`
	TLS      *OtelTlsConfig    `json:"tls,omitempty"`
}
type OtelGRPCExporter struct {
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	TLS      *OtelTlsConfig    `json:"tls,omitempty"`
}

// OtelTlsConfig carries server-reported TLS metadata as opaque strings.
type OtelTlsConfig struct {
	CACertificate     *string `json:"ca-certificate,omitempty"`
	ClientCertificate *string `json:"client-certificate,omitempty"`
	ClientPrivateKey  *string `json:"client-private-key,omitempty"`
}
type OtelToolResultLogConfig struct {
	MaxBytes uint `json:"max_bytes"`
}

func (c *OtelToolResultLogConfig) UnmarshalJSON(data []byte) error {
	if err := validateRequiredObjectFields(data, "max_bytes"); err != nil {
		return err
	}
	type wire OtelToolResultLogConfig
	var decoded wire
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateNativeNonNullFields(data, "max_bytes"); err != nil {
		return err
	}
	*c = OtelToolResultLogConfig(decoded)
	return nil
}

func (o *OtelExporter) UnmarshalJSON(data []byte) error {
	decoded, err := decodeOtelExporter(data)
	if err != nil {
		return err
	}
	*o = decoded
	return nil
}

func decodeOtelExporter(data []byte) (OtelExporter, error) {
	var decoded OtelExporter
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := jsondecode.Unmarshal(data, &decoded.Kind); err != nil {
			return decoded, err
		}
		if decoded.Kind != "none" && decoded.Kind != "statsig" {
			return decoded, errors.New("invalid otel exporter")
		}
		return decoded, nil
	}
	if err := validateObjectResponseResult(data); err != nil {
		return decoded, err
	}
	var fields map[string]json.RawMessage
	if err := jsondecode.Unmarshal(data, &fields); err != nil {
		return decoded, err
	}
	if len(fields) != 1 {
		return decoded, errors.New("otel exporter requires exactly one branch")
	}
	// Each object occurrence must validate, including an earlier branch hidden by
	// a later duplicate. Publish only the final valid value after the entire walk.
	err := validateFields(data, func(name string, raw []byte) error {
		switch name {
		case "otlp-http":
			value := new(OtelHTTPExporter)
			if err := decodeOtelExporterObject(raw, value, "endpoint", "headers", "protocol"); err != nil {
				return err
			}
			decoded.HTTP = value
		case "otlp-grpc":
			value := new(OtelGRPCExporter)
			if err := decodeOtelExporterObject(raw, value, "endpoint", "headers"); err != nil {
				return err
			}
			decoded.GRPC = value
		}
		return nil
	}, false, "otlp-http", "otlp-grpc")
	if err != nil {
		return decoded, err
	}
	if decoded.HTTP == nil && decoded.GRPC == nil {
		return decoded, errors.New("unknown otel exporter branch")
	}
	return decoded, nil
}

func decodeOtelExporterObject(raw []byte, dest any, required ...string) error {
	if err := validateRequiredObjectFields(raw, required...); err != nil {
		return err
	}
	if err := jsondecode.Unmarshal(raw, dest); err != nil {
		return err
	}
	if err := validateNativeNonNullFields(raw, required...); err != nil {
		return err
	}
	return validateOptionalFields(raw, func(_ string, headers []byte) error { return validateOtelStringMap(headers, false) }, "headers")
}

func (o OtelExporter) MarshalJSON() ([]byte, error) {
	switch {
	case o.HTTP == nil && o.GRPC == nil && (o.Kind == "none" || o.Kind == "statsig"):
		return jsonencode.Marshal(o.Kind)
	case o.Kind == "" && o.HTTP != nil && o.GRPC == nil:
		if o.HTTP.Headers == nil {
			return nil, errors.New("otel headers must not be null")
		}
		return jsonencode.Marshal(struct {
			HTTP *OtelHTTPExporter `json:"otlp-http"`
		}{o.HTTP})
	case o.Kind == "" && o.GRPC != nil && o.HTTP == nil:
		if o.GRPC.Headers == nil {
			return nil, errors.New("otel headers must not be null")
		}
		return jsonencode.Marshal(struct {
			GRPC *OtelGRPCExporter `json:"otlp-grpc"`
		}{o.GRPC})
	default:
		return nil, errors.New("otel exporter requires exactly one valid branch")
	}
}

func validateOtelStringMap(raw []byte, nested bool) error {
	if isNullJSONValue(raw) {
		return errors.New("otel map must not be null")
	}
	// Check shape before borrowing fields. Do not materialize a map that would
	// discard an earlier invalid duplicate value.
	if err := validateObjectResponseResult(raw); err != nil {
		return err
	}
	var validationErr error
	jsonobject.WalkFields(raw, true, func(_ []byte, value []byte) {
		if validationErr != nil {
			return
		}
		if nested {
			validationErr = validateOtelStringMap(value, false)
			return
		}
		if isNullJSONValue(value) {
			validationErr = errors.New("otel map value must be a string")
			return
		}
		var text string
		validationErr = jsondecode.Unmarshal(value, &text)
	})
	return validationErr
}
