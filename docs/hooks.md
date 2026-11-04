# Hook verification

The pre-commit hook checks the intended index snapshot without changing the
working tree or index. Partially staged files retain their selected contents;
unselected edits, untracked files and working copies of module or lint
configuration do not become validation inputs.

The checker copies the effective index, including alternate indices and linked
worktrees, then captures its tree. Checked NUL-delimited Git output selects added,
copied, modified, renamed, type-changed and deleted Go paths. A failed selection
stops the check. Deletions trigger package validation without formatting a missing
file. Go blobs needing canonical formatting reject the commit with a request to
format and stage the intended changes.

The complete temporary tree comes from indexed blobs and modes, including embedded
data and selected configuration. Archive exclusions, working-file filters and
unstaged symlink replacements do not supply source. Symbolic `.go` files are
unsupported; regular indexed Go files provide unambiguous source ownership.
Other symbolic links must resolve entirely to captured paths. Missing, external,
cyclic or unrepresentable paths fail before analysis.

Lint runs once on that captured module using the pinned tool and selected lint
configuration. Its complete JSON report preserves exact filenames. Issue caps
and native new-issue filters are disabled before the checker selects added Go
lines relative to `HEAD`, or the empty tree for an initial commit. NUL-delimited
raw changes identify old/new blobs; zero-context blob diffs supply line ranges
without parsing display filenames. This also handles space and Unicode names
that the pinned linter's native patch filter misreads. Type-check, tool and
report failures block the commit independently of changed-line matching.

Bootstrap and lint ignore ambient `GOFLAGS` and persisted Go environment settings
(`GOENV=off`), so overlays and alternate module files cannot replace captured
inputs. Build tags belong in the selected lint configuration. Explicit environment
settings such as `GOEXPERIMENT` remain available. Owned Git diffs also ignore
`GIT_DIFF_OPTS`, which otherwise overrides zero-context line selection.

The checker rechecks the index tree and `HEAD` before success. Changes during
validation require a retry. Temporary files and child processes are cleaned up
on ordinary failures and handled cancellation. The hook does not synchronize
concurrent Git operations after it returns. Its changed-line lint scope is
complemented by the complete repository checks in pre-push and CI.

The execution tests use disposable Git repositories and the actual hook. Cases
cover partial staging, failures, symlinks and executable modes, exact filenames,
renames, initial commits, alternate/split indices and linked worktrees. Pinned
linter cases check both inclusion of staged findings and exclusion of earlier
findings despite unrelated working edits. Generated edit histories compare the
hunk parser with independently known inserted-line positions in actual Git diffs.
