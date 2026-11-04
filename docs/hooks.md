# Hook verification

The pre-commit hook checks the intended index snapshot without changing the
working tree or index. Partially staged files retain their selected contents;
unselected edits, untracked files and working copies of module or lint
configuration do not become validation inputs.

Both hooks build their checker with a dedicated, dependency-free bootstrap module
and reuses the existing process-tree controls. Bootstrap source files and the
root `go.mod` must remain present; Go requires the latter to locate the module
even though its working contents are ignored. A missing prerequisite blocks the
check without changing staging. The shell owns the temporary executable and
forwards cancellation to the checker before removing it.
Cancellation during bootstrap waits for the active build to finish before
cleanup and prevents checker launch; it does not abandon the compiler frontend.

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
validation require a retry. Temporary files and supervised group/job members are
cleaned up on ordinary failures and handled cancellation. Runner execution begins only
after process-tree attachment; version checks, installation and analysis share
that ownership. Remaining group/job members are stopped on ordinary completion
as well. Temporary roots are resolved before analysis and report validation.
The hook does not synchronize
concurrent Git operations after it returns. Its changed-line lint scope is
complemented by the complete repository checks in pre-push and CI.

The execution tests use disposable Git repositories and the actual hook. Cases
cover partial staging, failures, symlinks and executable modes, exact filenames,
renames, initial commits, alternate/split indices and linked worktrees. Pinned
linter cases check both inclusion of staged findings and exclusion of earlier
findings despite unrelated working edits. Generated edit histories compare the
hunk parser with independently known inserted-line positions in actual Git diffs.

## Outgoing revisions

Pre-push accepts nondeleted updates only when each resolves to the exact
checked-out `HEAD` commit. All updates are preflighted before checks begin, so an
unsupported ref prevents execution even when another update is supported. Tags
pointing to `HEAD` and multiple refs pointing to it are supported. Other commits,
including unattached contributor refs, are rejected with a request to inspect and
check out the intended revision first. This retains the existing trust assumption
that the developer has chosen the checked-out code to run; snapshot isolation
alone does not authorize executing other repository code on the host.

Tracked working edits, staged changes and index flags that hide tracked source
(assume-unchanged or sparse-checkout entries) reject the push. Working content
is compared through a fresh private `HEAD` index without caller stat-cache data;
Git checkout conversions still apply, while stale size/mtime metadata cannot hide
changed bytes. Untracked and ignored
files remain excluded from verification inputs. The checker captures immutable
`HEAD` source and rechecks both `HEAD` and tracked cleanliness before success.
Concurrent edits are preserved and require a retry. It verifies the selected
commit, rather than certifying every historical commit reachable from it.

The shell captures the complete ref-update input before building the checker.
The compiler receives EOF instead of Git's input. Capture failures, malformed
records and noncommit targets stop the push. A successful empty or deletion-only
selection skips source checks. Deletions mixed with source updates still require
the complete selection to be supported and verified.

The private tree comes from Git blobs and modes using pre-commit's path and
symlink boundaries. Ordinary tests, race tests, pinned lint and module tidiness
run against its module and lint configuration. Working files and the caller's
index are never rewritten. Replacement objects cannot substitute another tree,
and a failed check prevents Git from publishing the updates.

The checked-out bootstrap and runner remain trusted tooling prerequisites. The
runner consumes an explicit source-root argument without exporting a sticky
override. Git-local environment variables are removed before checks, including
for nested Git fixtures; the private tree contains no Git metadata.
`GOFLAGS=-buildvcs=false` disables metadata stamping, and workspace, overlay and
persisted Go defaults cannot replace selected inputs. Caller-supplied experiments
remain available. Nix delegation uses the same trusted runner and a URL-escaped
`path:` flake reference to the captured source, including when its temporary
directory is inside another Git worktree.

Owned checks share group/job supervision and cleanup with pre-commit. Input
capture is separately cancellable. Cancellation during compilation waits for the
active build. Admitted ordinary/race tests also finish before cancellation
returns: their own cleanup can release independently managed process groups.
The cancelled push then fails without starting later checks. This can delay
cancellation until test completion. Other checks stop their supervised group/job
before source removal. Crashing or timing-out tools, uncatchable termination and
independently detached children are outside this containment guarantee; hooks
execute trusted tooling and checked-out tests rather than isolating arbitrary
code.

Execution regressions use actual Git pushes to local bare repositories and check
remote refs and source contents. Harmless marker-writing initializers demonstrate
that unattached refs, mixed unsupported updates and hidden or patched tracked
source are rejected before execution. Tests also cover all four failing gates,
multiple supported refs, tags, deletions, exact Unicode names and capture failures.
Actual hook-PID cancellation covers capture, compilation and each check phase.
Finite ordinary/race fixtures retain source through independent child-group
cleanup despite repeated cancellation, and checkout drift cannot produce a
successful verification.
