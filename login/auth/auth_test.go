package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const permissiveCredentialDirMode os.FileMode = 0o755

func TestExtractTokenClaims(t *testing.T) {
	token := fakeAccessToken(t, "acct_123", "plus")
	claims, err := ExtractTokenClaims(token)
	if err != nil {
		t.Fatalf("ExtractTokenClaims() error = %v", err)
	}
	if claims.AccountID != "acct_123" {
		t.Fatalf("AccountID = %q, want acct_123", claims.AccountID)
	}
	if claims.PlanType == nil || *claims.PlanType != "plus" {
		t.Fatalf("PlanType = %v, want plus", claims.PlanType)
	}
}

func TestCredentialsStoragePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	creds := testCredentials(t)
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if runtime.GOOS != "windows" {
		// Windows only maps os.Chmod to the read-only attribute; Unix bits are synthetic.
		if got := info.Mode().Perm(); got != credentialFileMode {
			t.Fatalf("credential mode = %v, want %v", got, credentialFileMode)
		}
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatalf("stat credential directory: %v", err)
		}
		if got := dirInfo.Mode().Perm(); got != credentialDirMode {
			t.Fatalf("credential directory mode = %v, want %v", got, credentialDirMode)
		}
	}
	loaded, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if loaded.AccessToken != creds.AccessToken || loaded.RefreshToken != creds.RefreshToken || loaded.AccountID != creds.AccountID {
		t.Fatalf("loaded credentials = %+v", loaded.Redacted())
	}
}

func TestSaveCredentialsTightensExistingDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permission bits are synthetic")
	}

	dir := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(dir, permissiveCredentialDirMode); err != nil {
		t.Fatalf("mkdir credential directory: %v", err)
	}
	if err := os.Chmod(dir, permissiveCredentialDirMode); err != nil {
		t.Fatalf("chmod credential directory: %v", err)
	}

	path := filepath.Join(dir, "credentials.json")
	if err := SaveCredentials(path, testCredentials(t)); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat credential directory: %v", err)
	}
	if got := info.Mode().Perm(); got != credentialDirMode {
		t.Fatalf("credential directory mode = %v, want %v", got, credentialDirMode)
	}
}

func testCredentials(t *testing.T) Credentials {
	t.Helper()
	return Credentials{
		AccessToken:  fakeAccessToken(t, "acct_123", "plus"),
		RefreshToken: "refresh-secret",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct_123",
	}
}

func TestStringRedaction(t *testing.T) {
	creds := Credentials{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct_123",
	}
	text := fmt.Sprintf("%v %#v", creds, creds)
	if strings.Contains(text, "access-secret") || strings.Contains(text, "refresh-secret") {
		t.Fatalf("credentials string leaked secret: %s", text)
	}

	params, err := NewAuthTokensLoginParams(creds)
	if err != nil {
		t.Fatalf("NewAuthTokensLoginParams() error = %v", err)
	}
	text = fmt.Sprintf("%v %#v", params, params)
	if strings.Contains(text, "access-secret") {
		t.Fatalf("params string leaked secret: %s", text)
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal(params): %v", err)
	}
	if !strings.Contains(string(encoded), "access-secret") {
		t.Fatalf("wire JSON should include access token: %s", encoded)
	}

	refresh, err := NewAuthTokensRefreshResponse(creds)
	if err != nil {
		t.Fatalf("NewAuthTokensRefreshResponse() error = %v", err)
	}
	text = fmt.Sprintf("%v %#v", refresh, refresh)
	if strings.Contains(text, "access-secret") {
		t.Fatalf("refresh response string leaked secret: %s", text)
	}
}

func fakeAccessToken(t *testing.T, accountID string, planType string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := map[string]any{
		openAIAuthClaim: map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_plan_type":  planType,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fake JWT payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + ".signature"
}
