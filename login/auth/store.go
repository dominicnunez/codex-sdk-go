package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	credentialDirMode  os.FileMode = 0o700
	credentialFileMode os.FileMode = 0o600
)

func SaveCredentials(path string, creds Credentials) error {
	if err := creds.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, credentialDirMode); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if err := ensureCredentialDirMode(dir); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".codex-auth-*.tmp")
	if err != nil {
		return fmt.Errorf("create credential temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(creds); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode credentials: %w", err)
	}
	if err := tmp.Chmod(credentialFileMode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod credential temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close credential temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace credential file: %w", err)
	}
	if err := os.Chmod(path, credentialFileMode); err != nil {
		return fmt.Errorf("chmod credential file: %w", err)
	}
	return nil
}

func ensureCredentialDirMode(dir string) error {
	if err := os.Chmod(dir, credentialDirMode); err != nil {
		return fmt.Errorf("chmod credential directory: %w", err)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat credential directory: %w", err)
	}
	if got := info.Mode().Perm(); got != credentialDirMode {
		return fmt.Errorf("credential directory mode = %v, want %v", got, credentialDirMode)
	}
	return nil
}

func LoadCredentials(path string) (Credentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("open credentials: %w", err)
	}
	defer f.Close()

	var creds Credentials
	if err := json.NewDecoder(f).Decode(&creds); err != nil {
		return Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}
