#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
runner="$repo_root/scripts/hooks/run.sh"

echo "pre-push: running tests"
"$runner" go test ./...

echo "pre-push: running race tests"
"$runner" go test -race ./...

echo "pre-push: running golangci-lint"
"$runner" golangci-lint run ./...

echo "pre-push: checking go.mod/go.sum tidiness"
"$runner" go mod tidy -diff
