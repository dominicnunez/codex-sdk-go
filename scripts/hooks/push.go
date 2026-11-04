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

func cleanTracked(g hookGit, head string) error {
	data, err := g.output("ls-files", "-v", "-z")
	if err != nil {
		return err
	}
	entries, err := nulFields(data)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if len(entry) < 3 || entry[:2] != "H " {
			return errors.New("unsupported index flags; clear assume-unchanged or sparse-checkout flags before pushing so tracked source is visible")
		}
	}
	dirty := errors.New("tracked source differs from checked-out HEAD; inspect and commit or set aside tracked edits before pushing")
	if _, err := g.output("diff", "--quiet", "--cached", "--no-ext-diff", "--no-textconv", head, "--"); err != nil {
		if exitOne(err) {
			return dirty
		}
		return err
	}
	// A copied index could reuse stale size/mtime data and miss changed bytes.
	// Populate a new index from HEAD without any caller stat cache or hidden
	// entry flags, so Git reads working content with its checkout conversions.
	root, err := os.MkdirTemp("", "hook-clean-index-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	fresh := g
	fresh.env = replaceEnv(g.env, "GIT_INDEX_FILE", filepath.ToSlash(filepath.Join(root, "index")))
	config := []string{"-c", "core.splitIndex=false", "-c", "core.sparseCheckout=false", "-c", "core.fsmonitor=false", "-c", "core.ignoreStat=false", "-c", "core.checkStat=default", "-c", "core.trustctime=true"}
	if _, err := fresh.output(append(append([]string{}, config...), "read-tree", head)...); err != nil {
		return err
	}
	args := append(append([]string{}, config...), "diff", "--quiet", "--no-ext-diff", "--no-textconv", "--")
	if _, err := fresh.output(args...); err != nil {
		if exitOne(err) {
			return dirty
		}
		return err
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
	head, err := g.head()
	if err != nil {
		return err
	}
	if head == "" {
		return errors.New("check out the intended commit before pushing; only checked-out HEAD is supported")
	}
	// Preflight every update before executing even the first supported tree.
	for _, tip := range tips {
		commit, err := g.text("rev-parse", "--verify", tip.oid+"^{commit}")
		if err != nil {
			return fmt.Errorf("cannot verify outgoing %q: only commit-backed refs are supported: %w", tip.ref, err)
		}
		if commit != head {
			return fmt.Errorf("outgoing %q is not checked-out HEAD; inspect and check out that commit before pushing it, so uninspected ref code is not executed", tip.ref)
		}
	}
	if err := cleanTracked(g, head); err != nil {
		return err
	}
	tree, err := g.text("rev-parse", "--verify", head+"^{tree}")
	if err != nil {
		return err
	}
	env, err := toolEnvironment(g)
	if err != nil {
		return err
	}
	runner := filepath.Join(g.dir, "scripts", "hooks", "run.sh")
	fmt.Fprintf(os.Stdout, "pre-push: verifying checked-out HEAD at %s\n", head)
	if err := checkPushTree(g, tree, runner, env); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	after, err := g.head()
	if err != nil {
		return err
	}
	if after != head {
		return errors.New("HEAD changed during verification; retry the push after inspecting the checked-out revision")
	}
	if err := cleanTracked(g, head); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "pre-push: verified checked-out HEAD for all outgoing updates")
	return nil
}
