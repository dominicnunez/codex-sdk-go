package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNumericDiagnosticTokenClaims(t *testing.T) {
	payload := `{"ignored":1` + strings.Repeat("0", 1<<20) + `,"https://api.openai.com/auth":{"chatgpt_account_id":"synthetic-account"}}`
	var reference map[string]any
	var expected *json.UnmarshalTypeError
	if !errors.As(json.Unmarshal([]byte(payload), &reference), &expected) {
		t.Fatal("reference did not overflow")
	}
	token := "e30." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".synthetic"
	claims, err := ExtractTokenClaims(token)
	var actual *json.UnmarshalTypeError
	if !errors.As(err, &actual) || len(actual.Value) > 2048 || len(err.Error()) > 4096 || !strings.Contains(actual.Value, "bytes omitted") {
		t.Fatal("claims overflow lost bounded typed rejection")
	}
	metadata := *expected
	metadata.Value = actual.Value
	if !reflect.DeepEqual(*actual, metadata) || !reflect.DeepEqual(claims, TokenClaims{}) {
		t.Fatal("claims overflow changed metadata or published partial claims")
	}
	claims, err = ExtractTokenClaims(fakeAccessToken(t, "synthetic-account", "plus"))
	if err != nil || claims.AccountID != "synthetic-account" {
		t.Fatal("valid claims did not recover")
	}
}
