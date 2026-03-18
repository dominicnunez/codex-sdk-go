#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
runner="$repo_root/scripts/hooks/run.sh"

echo "pre-push: running tests"
"$runner" go test ./...

echo "pre-push: running go vet"
"$runner" go vet ./...

echo "pre-push: checking go.mod/go.sum tidiness"
"$runner" go mod tidy -diff
