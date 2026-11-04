package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type outgoingTip struct{ ref, oid string }

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func pushTips(input io.Reader) ([]outgoingTip, error) {
	reader := bufio.NewReader(input)
	var tips []outgoingTip
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read ref updates: %w", err)
		}
		if line != "" {
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			fields := strings.Split(line, " ")
			if len(fields) != 4 || fields[0] == "" || fields[2] == "" || !validOID(fields[1]) || !validOID(fields[3]) || len(fields[1]) != len(fields[3]) {
				return nil, errors.New("malformed Git ref-update record")
			}
			if strings.Trim(fields[1], "0") != "" {
				tips = append(tips, outgoingTip{fields[0], fields[1]})
			}
		}
		if errors.Is(err, io.EOF) {
			return tips, nil
		}
	}
}

func toolEnvironment(g hookGit) ([]string, error) {
	data, err := g.output("rev-parse", "--local-env-vars")
	if err != nil {
		return nil, err
	}
	local := make(map[string]bool)
	for _, key := range strings.Fields(string(data)) {
		local[strings.ToUpper(key)] = true
	}
	var env []string
	for _, entry := range g.env {
		key, _, _ := strings.Cut(entry, "=")
		if !local[strings.ToUpper(key)] {
			env = append(env, entry)
		}
	}
	env = replaceEnv(env, "GO111MODULE", "on")
	env = replaceEnv(env, "GOWORK", "off")
	env = replaceEnv(env, "GOENV", "off")
	env = replaceEnv(env, "GOFLAGS", "-buildvcs=false")
	return env, nil
}

func checkPushTree(g hookGit, tree, runner string, env []string) error {
	entries, err := readTree(g, tree)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "hook-outgoing-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if err := materialize(g, entries, root); err != nil {
		return err
	}
	checks := [][]string{
		{"go", "test", "./..."},
		{"go", "test", "-race", "./..."},
		{"golangci-lint", "run", "--config=" + filepath.ToSlash(filepath.Join(root, ".golangci.yml")), "./..."},
		{"go", "mod", "tidy", "-diff"},
	}
	for _, check := range checks {
		fmt.Fprintf(os.Stdout, "pre-push: %s\n", strings.Join(check[:2], " "))
		args := append([]string{filepath.ToSlash(runner), "--repo-root", filepath.ToSlash(root)}, check...)
		// Tests can own independent process groups. Let an admitted test command
		// run its cleanup before cancellation removes source or ends the push.
		finish := check[0] == "go" && check[1] == "test"
		if err := runCommand(g.ctx, "bash", root, env, args, finish); err != nil {
			return fmt.Errorf("verify outgoing tree: %w", err)
		}
	}
	return nil
}

func checkPush(ctx context.Context, input io.Reader) error {
	tips, err := pushTips(input)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(tips) == 0 {
		fmt.Fprintln(os.Stdout, "pre-push: no outgoing commit tips; source checks skipped")
		return nil
	}
	g, err := rootGit(ctx)
	if err != nil {
		return err
	}
	env, err := toolEnvironment(g)
	if err != nil {
		return err
	}
	runner := filepath.Join(g.dir, "scripts", "hooks", "run.sh")
	seen := make(map[string]bool)
	for _, tip := range tips {
		commit, err := g.text("rev-parse", "--verify", tip.oid+"^{commit}")
		if err != nil {
			return fmt.Errorf("cannot verify outgoing %q: only commit-backed refs are supported: %w", tip.ref, err)
		}
		tree, err := g.text("rev-parse", "--verify", commit+"^{tree}")
		if err != nil {
			return err
		}
		if seen[tree] {
			continue
		}
		seen[tree] = true
		fmt.Fprintf(os.Stdout, "pre-push: verifying %q at %s\n", tip.ref, commit)
		if err := checkPushTree(g, tree, runner, env); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "pre-push: verified %d outgoing trees\n", len(seen))
	return nil
}
