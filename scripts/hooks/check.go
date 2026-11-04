// Command check validates the source snapshots selected by local Git hooks.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/dominicnunez/codex-sdk-go/internal/process"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "hook-child" {
		if err := runChild(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "hook child:", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) != 2 || os.Args[1] != "pre-commit" {
		fmt.Fprintln(os.Stderr, "usage: check pre-commit")
		os.Exit(1)
	}
	if err := checkCommit(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "pre-commit:", err)
		os.Exit(1)
	}
}

func runChild(args []string) error {
	var gate [1]byte
	if _, err := io.ReadFull(os.Stdin, gate[:]); err != nil {
		return err
	}
	if gate[0] != 1 {
		return errors.New("invalid hook child admission")
	}
	cmd := exec.Command(args[0], args[1:]...) //nolint:noctx // The admitted process tree owns runner cancellation.
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// runOwned admits the runner only after its process group/job owns descendants.
func runOwned(ctx context.Context, command, dir string, env, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, append([]string{"hook-child", command}, args...)...) //nolint:noctx // Attach the tree before arming cancellation or admitting the runner.
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	gate, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer gate.Close()
	process.ConfigureCommand(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	tree, err := process.AttachTree(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	defer tree.Close()
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = tree.ForceKill(cmd.Process)
		close(cancelled)
	})
	if ctx.Err() == nil {
		_, err = gate.Write([]byte{1})
	}
	_ = gate.Close()
	waitErr := cmd.Wait()
	if !stop() {
		<-cancelled
	}
	// A runner must not leave background work after its own exit either.
	killErr := tree.ForceKill(cmd.Process)
	if process.IsExpectedStopError(killErr) {
		killErr = nil
	}
	return errors.Join(err, waitErr, killErr, ctx.Err())
}

type hookGit struct {
	ctx context.Context
	dir string
	env []string
}

func replaceEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		match := name == key
		if runtime.GOOS == "windows" {
			match = strings.EqualFold(name, key)
		}
		if !match {
			out = append(out, entry)
		}
	}
	return append(out, key+"="+value)
}

func (g hookGit) output(args ...string) ([]byte, error) {
	cmd := exec.CommandContext(g.ctx, "git", args...)
	cmd.Dir, cmd.Env = g.dir, g.env
	var out, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(diagnostic.String()))
	}
	return out.Bytes(), nil
}

func (g hookGit) text(args ...string) (string, error) {
	out, err := g.output(args...)
	return strings.TrimSuffix(strings.TrimSuffix(string(out), "\n"), "\r"), err
}

func exitCode(err error, code int) bool {
	var failure *exec.ExitError
	return errors.As(err, &failure) && failure.ExitCode() == code
}

func (g hookGit) head() (string, error) {
	head, err := g.text("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err == nil {
		return head, nil
	}
	if exitCode(err, 1) {
		ref, refErr := g.text("symbolic-ref", "--quiet", "HEAD")
		if refErr == nil {
			_, existsErr := g.output("show-ref", "--verify", "--quiet", ref)
			if exitCode(existsErr, 1) {
				return "", nil
			}
		}
	}
	return "", err
}

func captureIndex(g hookGit, index string) (hookGit, string, func(), error) {
	file, err := os.CreateTemp(filepath.Dir(index), "hook-index-*")
	if err != nil {
		return g, "", nil, err
	}
	private := file.Name()
	cleanup := func() {
		_ = os.Remove(private)
		_ = os.Remove(private + ".lock")
	}
	data, readErr := os.ReadFile(index)
	if readErr == nil {
		_, err = file.Write(data)
	} else if !os.IsNotExist(readErr) {
		err = readErr
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return g, "", nil, err
	}
	g.env = replaceEnv(g.env, "GIT_INDEX_FILE", filepath.ToSlash(private))
	if os.IsNotExist(readErr) {
		if err := os.Remove(private); err != nil {
			cleanup()
			return g, "", nil, err
		}
		if _, err := g.output("read-tree", "--empty"); err != nil {
			cleanup()
			return g, "", nil, err
		}
	}
	tree, err := g.text("write-tree")
	if err != nil {
		cleanup()
		return g, "", nil, err
	}
	return g, tree, cleanup, nil
}

func nulFields(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if data[len(data)-1] != 0 {
		return nil, errors.New("incomplete NUL-delimited Git output")
	}
	return strings.Split(string(data[:len(data)-1]), "\x00"), nil
}

type treeEntry struct{ mode, oid, name string }

func indexedPath(name string) error {
	if !filepath.IsLocal(filepath.FromSlash(name)) || runtime.GOOS == "windows" && strings.Contains(name, "\\") {
		return fmt.Errorf("cannot represent indexed path %q", name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." || strings.EqualFold(component, ".git") {
			return fmt.Errorf("unsafe indexed path %q", name)
		}
	}
	return nil
}

func readTree(g hookGit, tree string) ([]treeEntry, error) {
	data, err := g.output("ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	rows, err := nulFields(data)
	if err != nil {
		return nil, err
	}
	entries := make([]treeEntry, 0, len(rows))
	for _, row := range rows {
		meta, name, ok := strings.Cut(row, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("cannot capture indexed entry %q", name)
		}
		if err := indexedPath(name); err != nil {
			return nil, err
		}
		if fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000" {
			return nil, fmt.Errorf("unsupported indexed mode %s for %q", fields[0], name)
		}
		entries = append(entries, treeEntry{fields[0], fields[2], name})
	}
	return entries, nil
}

func makeParents(root, name string, made map[string]bool) error {
	parent := path.Dir(name)
	if parent == "." || made[parent] {
		return nil
	}
	if err := makeParents(root, parent, made); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(parent)), 0o700); err != nil {
		return fmt.Errorf("cannot represent directory %q: %w", parent, err)
	}
	made[parent] = true
	return nil
}

type blobReader struct {
	cmd        *exec.Cmd
	in         io.WriteCloser
	out        *bufio.Reader
	diagnostic bytes.Buffer
}

func openBlobs(g hookGit) (*blobReader, error) {
	b := &blobReader{cmd: exec.CommandContext(g.ctx, "git", "cat-file", "--batch")}
	b.cmd.Dir, b.cmd.Env = g.dir, g.env
	in, err := b.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	b.in = in
	out, err := b.cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	b.out = bufio.NewReader(out)
	b.cmd.Stderr = &b.diagnostic
	if err := b.cmd.Start(); err != nil {
		_ = in.Close()
		_ = out.Close()
		return nil, err
	}
	return b, nil
}

func (b *blobReader) close(failed bool) error {
	_ = b.in.Close()
	if failed {
		_ = b.cmd.Process.Kill()
	}
	if err := b.cmd.Wait(); err != nil {
		return fmt.Errorf("read indexed blobs: %w: %s", err, b.diagnostic.String())
	}
	return nil
}

func (b *blobReader) size(oid string) (int64, error) {
	if _, err := fmt.Fprintln(b.in, oid); err != nil {
		return 0, err
	}
	header, err := b.out.ReadString('\n')
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[0] != oid || fields[1] != "blob" {
		return 0, fmt.Errorf("unexpected indexed blob header %q", header)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("invalid indexed blob size %q", fields[2])
	}
	return size, nil
}

func (b *blobReader) copyEntry(entry treeEntry, root string, links map[string]string) error {
	size, err := b.size(entry.oid)
	if err != nil {
		return err
	}
	switch entry.mode {
	case "120000":
		if strings.HasSuffix(entry.name, ".go") || size > 65536 {
			return fmt.Errorf("cannot verify symbolic Go source or oversized link %q; use indexed regular Go files", entry.name)
		}
		data := make([]byte, int(size))
		if _, err := io.ReadFull(b.out, data); err != nil {
			return err
		}
		links[entry.name] = string(data)
	default:
		mode := os.FileMode(0o600)
		if entry.mode == "100755" {
			mode = 0o700
		}
		file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(entry.name)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return fmt.Errorf("cannot capture %q: %w", entry.name, err)
		}
		_, copyErr := io.CopyN(file, b.out, size)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	separator, err := b.out.ReadByte()
	if err != nil || separator != '\n' {
		return errors.New("incomplete indexed blob")
	}
	return nil
}

func capturedLink(name string, links map[string]string, known, dirs map[string]bool) error {
	queue := strings.Split(name, "/")
	var parts []string
	hops := 0
	for len(queue) != 0 {
		component := queue[0]
		queue = queue[1:]
		switch component {
		case "", ".":
			continue
		case "..":
			if len(parts) == 0 {
				return fmt.Errorf("link %q leaves the indexed snapshot", name)
			}
			parts = parts[:len(parts)-1]
			continue
		}
		parts = append(parts, component)
		current := strings.Join(parts, "/")
		if target, ok := links[current]; ok {
			hops++
			if hops > 255 || filepath.IsAbs(target) || path.IsAbs(filepath.ToSlash(target)) || filepath.VolumeName(target) != "" || strings.ContainsRune(target, 0) {
				return fmt.Errorf("unresolvable indexed link %q", name)
			}
			parts = parts[:len(parts)-1]
			queue = append(strings.Split(filepath.ToSlash(target), "/"), queue...)
			continue
		}
		if !known[current] && !dirs[current] || len(queue) != 0 && !dirs[current] {
			return fmt.Errorf("link %q refers to uncaptured source %q", name, current)
		}
	}
	return nil
}

func createLinks(root string, links map[string]string, entries []treeEntry, dirs map[string]bool) error {
	known := make(map[string]bool, len(entries))
	for _, entry := range entries {
		known[entry.name] = true
	}
	for name := range links {
		if err := capturedLink(name, links, known, dirs); err != nil {
			return err
		}
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(name))); err != nil {
			return err
		}
	}
	return nil
}

func materialize(g hookGit, entries []treeEntry, root string) (err error) {
	blobs, err := openBlobs(g)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := blobs.close(err != nil); err == nil {
			err = closeErr
		}
	}()
	dirs := map[string]bool{".": true}
	links := make(map[string]string)
	for _, entry := range entries {
		if err := g.ctx.Err(); err != nil {
			return err
		}
		if err := makeParents(root, entry.name, dirs); err != nil {
			return err
		}
		if err := blobs.copyEntry(entry, root, links); err != nil {
			return err
		}
	}
	return createLinks(root, links, entries, dirs)
}

type lineRange struct{ first, last int }

var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

func addedLines(diff []byte) ([]lineRange, error) {
	var ranges []lineRange
	for _, line := range bytes.Split(diff, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("@@")) {
			continue
		}
		match := hunkHeader.FindSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("invalid Git hunk header %q", line)
		}
		first, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return nil, err
		}
		count := 1
		if len(match[2]) != 0 {
			count, err = strconv.Atoi(string(match[2]))
			if err != nil {
				return nil, err
			}
		}
		if count == 0 {
			continue
		}
		if first < 1 || count > int(^uint(0)>>1)-first+1 {
			return nil, errors.New("git hunk line range overflows")
		}
		ranges = append(ranges, lineRange{first, first + count - 1})
	}
	return ranges, nil
}

func stagedLines(g hookGit, base, tree string) (map[string][]lineRange, error) {
	data, err := g.output("diff", "--no-color", "--raw", "-z", "--no-abbrev", "--find-renames", "--no-ext-diff", "--no-textconv", base, tree, "--", "*.go")
	if err != nil {
		return nil, err
	}
	rows, err := nulFields(data)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]lineRange)
	for n := 0; n < len(rows); {
		meta := strings.Fields(rows[n])
		if len(meta) != 5 || !strings.HasPrefix(meta[0], ":") || n+1 >= len(rows) {
			return nil, errors.New("invalid raw Git change record")
		}
		name := rows[n+1]
		n += 2
		if strings.HasPrefix(meta[4], "R") || strings.HasPrefix(meta[4], "C") {
			if n >= len(rows) {
				return nil, errors.New("incomplete Git rename record")
			}
			name = rows[n]
			n++
		}
		if meta[1] == "000000" || !strings.HasSuffix(name, ".go") {
			continue
		}
		if meta[0] == ":000000" {
			result[name] = []lineRange{{1, int(^uint(0) >> 1)}}
			continue
		}
		diff, err := g.output("diff", "--no-color", "--text", "--no-ext-diff", "--no-textconv", "--unified=0", "--inter-hunk-context=0", meta[2], meta[3])
		if err != nil {
			return nil, err
		}
		ranges, err := addedLines(diff)
		if err != nil {
			return nil, err
		}
		result[name] = ranges
	}
	return result, nil
}

type lintIssue struct {
	FromLinter string
	Text       string
	Pos        struct {
		Filename string
		Line     int
		Column   int
	}
}

func lintReport(g hookGit, root, gitdir, index string) ([]lintIssue, error) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "hook-lint-*.json")
	if err != nil {
		return nil, err
	}
	report := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(report)
		return nil, err
	}
	defer os.Remove(report)
	args := []string{filepath.ToSlash(filepath.Join(root, "scripts", "hooks", "run.sh")), "golangci-lint", "run",
		"--config=" + filepath.ToSlash(filepath.Join(root, ".golangci.yml")), "--issues-exit-code=0", "--show-stats=false", "--path-mode=abs",
		"--output.json.path=" + filepath.ToSlash(report), "--output.text.path=", "--max-issues-per-linter=0", "--max-same-issues=0", "--uniq-by-line=false",
		"--new=false", "--new-from-rev=", "--new-from-patch=", "--new-from-merge-base=", "--whole-files=false", "./..."}
	env := replaceEnv(g.env, "GIT_DIR", filepath.ToSlash(gitdir))
	env = replaceEnv(env, "GIT_WORK_TREE", filepath.ToSlash(root))
	env = replaceEnv(env, "GIT_INDEX_FILE", filepath.ToSlash(index))
	env = replaceEnv(env, "GO111MODULE", "on")
	env = replaceEnv(env, "GOWORK", "off")
	env = replaceEnv(env, "GOENV", "off")
	env = replaceEnv(env, "GOFLAGS", "")
	if err := runOwned(g.ctx, bash, root, env, args); err != nil {
		return nil, fmt.Errorf("lint staged snapshot: %w", err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, err
	}
	var envelope struct{ Issues json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("read lint report: %w", err)
	}
	if len(envelope.Issues) == 0 {
		return nil, errors.New("lint report omitted Issues")
	}
	var issues []lintIssue
	if err := json.Unmarshal(envelope.Issues, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func checkIssues(issues []lintIssue, root string, lines map[string][]lineRange, entries []treeEntry) error {
	known := make(map[string]bool, len(entries))
	for _, entry := range entries {
		known[entry.name] = true
	}
	failures := 0
	for _, issue := range issues {
		if issue.FromLinter == "typecheck" {
			fmt.Fprintf(os.Stderr, "%s (typecheck)\n", issue.Text)
			failures++
			continue
		}
		name := issue.Pos.Filename
		if !filepath.IsAbs(name) {
			return fmt.Errorf("lint reported a non-absolute source path %q", name)
		}
		rel, err := filepath.Rel(root, name)
		if err != nil || !filepath.IsLocal(rel) || !known[filepath.ToSlash(rel)] {
			return fmt.Errorf("lint reported source outside the indexed snapshot: %q", name)
		}
		rel = filepath.ToSlash(rel)
		for _, interval := range lines[rel] {
			if issue.Pos.Line == 0 || issue.Pos.Line >= interval.first && issue.Pos.Line <= interval.last {
				fmt.Fprintf(os.Stderr, "%q:%d:%d: %s (%s)\n", rel, issue.Pos.Line, issue.Pos.Column, issue.Text, issue.FromLinter)
				failures++
				break
			}
		}
	}
	if failures != 0 {
		return fmt.Errorf("%d staged lint findings", failures)
	}
	return nil
}

func checkFormatting(root string, selected []string, entries []treeEntry) error {
	modes := make(map[string]string, len(entries))
	for _, entry := range entries {
		modes[entry.name] = entry.mode
	}
	for _, name := range selected {
		if !strings.HasSuffix(name, ".go") || modes[name] == "" {
			continue
		}
		if modes[name] != "100644" && modes[name] != "100755" {
			return fmt.Errorf("cannot verify symbolic Go source %q", name)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if _, err := parser.ParseFile(token.NewFileSet(), name, data, parser.ParseComments|parser.SkipObjectResolution); err != nil {
			return fmt.Errorf("staged Go syntax in %q: %w", name, err)
		}
		canonical, err := format.Source(data)
		if err != nil {
			return fmt.Errorf("staged Go syntax in %q: %w", name, err)
		}
		if !bytes.Equal(data, canonical) {
			return fmt.Errorf("staged Go file %q needs gofmt; format the working file and stage the intended changes", name)
		}
	}
	return nil
}

func validateSnapshot(g hookGit, tree, head, gitdir string, selected []string) error {
	base := head
	if base == "" {
		var err error
		base, err = g.text("mktree")
		if err != nil {
			return err
		}
	}
	entries, err := readTree(g, tree)
	if err != nil {
		return err
	}
	snapshot, err := os.MkdirTemp("", "hook-snapshot-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(snapshot)
	snapshot, err = filepath.EvalSymlinks(snapshot)
	if err != nil {
		return err
	}
	if err := materialize(g, entries, snapshot); err != nil {
		return err
	}
	if err := checkFormatting(snapshot, selected, entries); err != nil {
		return err
	}
	lines, err := stagedLines(g, base, tree)
	if err != nil {
		return err
	}
	index := ""
	for _, entry := range g.env {
		if value, ok := strings.CutPrefix(entry, "GIT_INDEX_FILE="); ok {
			index = value
		}
	}
	issues, err := lintReport(g, snapshot, gitdir, index)
	if err != nil {
		return err
	}
	return checkIssues(issues, snapshot, lines, entries)
}

func checkCommit(ctx context.Context) error {
	g := hookGit{ctx: ctx, env: replaceEnv(os.Environ(), "GIT_NO_REPLACE_OBJECTS", "1")}
	g.env = replaceEnv(g.env, "GIT_OPTIONAL_LOCKS", "0")
	g.env = replaceEnv(g.env, "GIT_DIFF_OPTS", "")
	root, err := g.text("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	g.dir = root
	gitdir, err := g.text("rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	index, err := g.text("rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(index) {
		index = filepath.Join(root, index)
	}
	head, err := g.head()
	if err != nil {
		return err
	}
	private, tree, cleanup, err := captureIndex(g, index)
	if err != nil {
		return fmt.Errorf("capture staged index: %w", err)
	}
	defer cleanup()
	data, err := private.output("diff", "--no-color", "--cached", "--name-only", "-z", "--diff-filter=ACDMRT", "--no-ext-diff", "--no-textconv", "--", "*.go")
	if err != nil {
		return fmt.Errorf("select staged Go files: %w", err)
	}
	selected, err := nulFields(data)
	if err != nil {
		return err
	}
	if len(selected) != 0 {
		if err := validateSnapshot(private, tree, head, gitdir, selected); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, afterTree, afterCleanup, err := captureIndex(g, index)
	if err != nil {
		return err
	}
	defer afterCleanup()
	afterHead, err := g.head()
	if err != nil {
		return err
	}
	if tree != afterTree || head != afterHead {
		return errors.New("staged index or HEAD changed during validation; retry the commit")
	}
	fmt.Fprintln(os.Stdout, "pre-commit: staged snapshot checks passed")
	return nil
}
