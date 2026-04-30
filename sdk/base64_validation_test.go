package codex

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateBase64Syntax(t *testing.T) {
	validLargeValue := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("payload", 1024)))

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty", value: ""},
		{name: "large valid payload", value: validLargeValue},
		{name: "invalid alphabet", value: "not valid base64", wantErr: true},
		{name: "invalid padding", value: "abcd=", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBase64Syntax(tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("validateBase64Syntax() error = nil; want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateBase64Syntax() error = %v; want nil", err)
			}
		})
	}
}
