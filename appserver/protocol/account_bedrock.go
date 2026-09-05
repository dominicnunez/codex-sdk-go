package protocol

import (
	"encoding/json"
	"fmt"
)

// AmazonBedrockAccessKeysLoginAccountParams authenticates with AWS access keys.
// Credentials are redacted from standard JSON and formatting; Login sends them on the wire.
type AmazonBedrockAccessKeysLoginAccountParams struct {
	Type            string  `json:"type"`
	AccessKeyID     string  `json:"accessKeyId"`
	SecretAccessKey string  `json:"secretAccessKey"`
	SessionToken    *string `json:"sessionToken,omitempty"`
	Region          string  `json:"region"`
}

func (*AmazonBedrockAccessKeysLoginAccountParams) isLoginAccountParams() {}

func (p *AmazonBedrockAccessKeysLoginAccountParams) marshalWire() ([]byte, error) {
	if p == nil {
		return nil, errNilLoginAccountParams
	}
	for _, f := range []struct{ name, value string }{{"accessKeyId", p.AccessKeyID}, {"secretAccessKey", p.SecretAccessKey}, {"region", p.Region}} {
		if err := validateRequiredNonEmptyStringField(f.name, f.value); err != nil {
			return nil, err
		}
	}
	type wire AmazonBedrockAccessKeysLoginAccountParams
	v := wire(*p)
	v.Type = "amazonBedrockAccessKeys"
	return json.Marshal(v)
}

func (p AmazonBedrockAccessKeysLoginAccountParams) MarshalJSON() ([]byte, error) {
	type wire AmazonBedrockAccessKeysLoginAccountParams
	v := wire(p)
	v.Type = "amazonBedrockAccessKeys"
	v.AccessKeyID = "[REDACTED]"
	v.SecretAccessKey = "[REDACTED]"
	if p.SessionToken != nil {
		v.SessionToken = Ptr("[REDACTED]")
	}
	return json.Marshal(v)
}

func (p AmazonBedrockAccessKeysLoginAccountParams) String() string {
	return "AmazonBedrockAccessKeysLoginAccountParams{credentials:[REDACTED]}"
}
func (p AmazonBedrockAccessKeysLoginAccountParams) GoString() string { return p.String() }
func (p AmazonBedrockAccessKeysLoginAccountParams) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprint(f, p.String())
}
