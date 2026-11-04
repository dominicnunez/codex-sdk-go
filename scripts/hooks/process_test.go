package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "hook-child" {
		main()
		return
	}
	os.Exit(m.Run())
}

func TestHookRunnerProbe(t *testing.T) {
	marker := os.Getenv("HOOK_TEST_MARKER")
	if marker == "" {
		return
	}
	switch os.Getenv("HOOK_TEST_MODE") {
	case "sleep":
		time.Sleep(time.Minute)
	case "spawn":
		cmd := exec.Command(os.Args[0], "-test.run=^TestHookRunnerProbe$")
		cmd.Env = replaceEnv(os.Environ(), "HOOK_TEST_MODE", "sleep")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		if err := os.WriteFile(marker, []byte("ran"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHookChildAdmission(t *testing.T) {
	for name, gate := range map[string][]byte{"missing": nil, "invalid": {0}, "valid": {1}} {
		t.Run(name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			t.Setenv("HOOK_TEST_MARKER", marker)
			input, err := os.CreateTemp(t.TempDir(), "gate")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if _, err := input.Write(gate); err != nil {
				t.Fatal(err)
			}
			if _, err := input.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			original := os.Stdin
			os.Stdin = input
			err = runChild([]string{os.Args[0], "-test.run=^TestHookRunnerProbe$"})
			os.Stdin = original
			valid := len(gate) == 1 && gate[0] == 1
			if (err == nil) != valid {
				t.Fatalf("gate acceptance: %v", err)
			}
			_, statErr := os.Stat(marker)
			if valid && statErr != nil || !valid && !os.IsNotExist(statErr) {
				t.Fatalf("runner admission differs from gate: %v", statErr)
			}
		})
	}
}

func TestOwnedRunnerRejectsCancelledAdmission(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	env := replaceEnv(os.Environ(), "HOOK_TEST_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runOwned(ctx, os.Args[0], "", env, []string{"-test.run=^TestHookRunnerProbe$"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission returned %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("cancelled runner executed: %v", err)
	}
}

func TestOwnedRunnerStopsBackgroundChildren(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "child")
	env := replaceEnv(os.Environ(), "HOOK_TEST_MARKER", marker)
	env = replaceEnv(env, "HOOK_TEST_MODE", "spawn")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, reader); close(drained) }()
	defer func() { _ = reader.Close(); <-drained }()
	original := os.Stdout
	os.Stdout = writer
	err = runOwned(context.Background(), os.Args[0], "", env, []string{"-test.run=^TestHookRunnerProbe$"})
	os.Stdout = original
	_ = writer.Close()
	data, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, parseErr := strconv.Atoi(string(data))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	defer func() {
		if child, err := os.FindProcess(pid); err == nil {
			_ = child.Kill()
			_ = child.Release()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("completed runner left a child retaining output")
	}
}
