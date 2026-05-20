package appserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dominicnunez/codex-sdk-go/appserver"
)

const appServerTestTimeout = 5 * time.Second

func writeAppServerScriptBinary(t *testing.T, dir, script string) string {
	t.Helper()

	binaryPath := filepath.Join(dir, "fake-codex")
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return binaryPath
}

func waitForFileContents(t *testing.T, path string) string {
	t.Helper()

	deadline := time.Now().Add(appServerTestTimeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.TrimSpace(string(data))
			if text != "" {
				return text
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for file contents: %s", path)
	return ""
}

func TestStartProcessUsesAppServerStdioArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := `#!/bin/sh
for arg in "$@"; do echo "$arg"; done > ` + argsFile + `
exit 0
`
	fakeBinary := writeAppServerScriptBinary(t, dir, script)

	proc, err := appserver.StartProcess(context.Background(), &appserver.ProcessOptions{BinaryPath: fakeBinary})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}
	_ = proc.Wait()
	_ = proc.Close()

	lines := strings.Split(strings.TrimSpace(waitForFileContents(t, argsFile)), "\n")
	want := []string{"app-server", "--listen", "stdio://"}
	if !slices.Equal(lines, want) {
		t.Fatalf("args = %v, want %v", lines, want)
	}
}

func TestStartProcessRejectsRelativeBinary(t *testing.T) {
	_, err := appserver.StartProcess(context.Background(), &appserver.ProcessOptions{BinaryPath: "codex"})
	if err == nil || !strings.Contains(err.Error(), "BinaryPath must be absolute") {
		t.Fatalf("StartProcess() error = %v, want absolute path error", err)
	}
}

func TestInitializeSendsInitializedNotification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process test requires unix shell script")
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "initialize.json")
	notifyFile := filepath.Join(dir, "notify.json")
	script := `#!/bin/sh
read initialize
printf '%s\n' "$initialize" > ` + initFile + `
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"codexHome":"/tmp/codex-home","platformFamily":"unix","platformOs":"linux","userAgent":"fake-codex/0.0.1"}}'
read initialized
printf '%s\n' "$initialized" > ` + notifyFile + `
while true; do sleep 1; done
`
	fakeBinary := writeAppServerScriptBinary(t, dir, script)

	ctx, cancel := context.WithTimeout(context.Background(), appServerTestTimeout)
	defer cancel()
	proc, err := appserver.StartProcess(ctx, &appserver.ProcessOptions{
		BinaryPath:    fakeBinary,
		ClientOptions: []appserver.ClientOption{appserver.WithRequestTimeout(time.Second)},
	})
	if err != nil {
		t.Fatalf("StartProcess: %v", err)
	}
	defer func() { _ = proc.Close() }()

	if _, err := proc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	var initMessage map[string]interface{}
	if err := json.Unmarshal([]byte(waitForFileContents(t, initFile)), &initMessage); err != nil {
		t.Fatalf("unmarshal initialize request: %v", err)
	}
	if initMessage["method"] != "initialize" {
		t.Fatalf("initialize method = %v", initMessage["method"])
	}

	var notifyMessage map[string]interface{}
	if err := json.Unmarshal([]byte(waitForFileContents(t, notifyFile)), &notifyMessage); err != nil {
		t.Fatalf("unmarshal initialized notification: %v", err)
	}
	if notifyMessage["method"] != "initialized" {
		t.Fatalf("notification method = %v", notifyMessage["method"])
	}
	if _, ok := notifyMessage["id"]; ok {
		t.Fatalf("initialized notification unexpectedly has id: %v", notifyMessage["id"])
	}
}
