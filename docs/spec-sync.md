# Spec-sync maintenance

The `spec-sync` workflow checks the upstream Codex app-server schemas on its schedule and through manual dispatch. When it detects changes, it commits the schema snapshot to a bot-owned `spec-sync/*` branch and opens a pull request.

## Superseded pull requests

Spec-sync workflow runs are serialized with `cancel-in-progress: false`. A run creates or finds its current pull request before looking for older sync pull requests. It then closes an older pull request only when all of these conditions hold:

- the REST API author login is `github-actions[bot]`;
- the head branch belongs to this repository;
- the head branch matches `spec-sync/*`;
- the pull request is not the current run's pull request.

Creating the current pull request first is intentional. It prevents two near-simultaneous runs from both observing an empty pull-request list and leaving duplicate sync pull requests open.

Do not weaken the author, repository, branch-prefix, or current-PR exclusions in the cleanup step. They prevent the workflow from closing unrelated contributor pull requests.

Cleanup uses the paginated REST pulls endpoint in `scripts/spec-sync-prune.sh`.
Do not compare `gh pr list --json author` against the REST login: that GraphQL
command returns `app/github-actions` for the same bot. This mismatch previously
caused every bot PR to be skipped while the workflow still reported success.
Branch deletion runs as part of closing each superseded PR.
