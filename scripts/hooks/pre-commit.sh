#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
runner="$repo_root/scripts/hooks/run.sh"

mapfile -t staged < <(git diff --cached --name-only --diff-filter=ACM -- '*.go')

if [[ ${#staged[@]} -eq 0 ]]; then
  echo "pre-commit: no staged Go files"
  exit 0
fi

echo "pre-commit: formatting staged Go files"
"$runner" gofmt -w "${staged[@]}"
git add -- "${staged[@]}"

echo "pre-commit: checking formatting"
bad=$("$runner" gofmt -l "${staged[@]}")
if [[ -n "$bad" ]]; then
  echo "pre-commit: gofmt left files unformatted:"
  echo "$bad"
  exit 1
fi

echo "pre-commit: running golangci-lint"
"$runner" golangci-lint run --new
