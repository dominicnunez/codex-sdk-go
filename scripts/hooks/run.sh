#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
required_golangci_lint_version="v2.11.3"

is_golangci_lint_v2() {
  local binary="$1"
  local version_output

  if ! version_output=$("$binary" version 2>/dev/null); then
    return 1
  fi

  [[ "$version_output" =~ version[[:space:]]v?2\. ]]
}

resolve_golangci_lint() {
  if command -v golangci-lint >/dev/null 2>&1; then
    local path_binary
    path_binary=$(command -v golangci-lint)
    if is_golangci_lint_v2 "$path_binary"; then
      printf '%s\n' "$path_binary"
      return 0
    fi
  fi

  if ! command -v go >/dev/null 2>&1; then
    printf 'hooks: go is required to install golangci-lint %s\n' "$required_golangci_lint_version" >&2
    return 1
  fi

  local gobin
  gobin=$(go env GOBIN)
  if [[ -z "$gobin" ]]; then
    gobin="$(go env GOPATH)/bin"
  fi

  local installed_binary="$gobin/golangci-lint"
  if [[ -x "$installed_binary" ]] && is_golangci_lint_v2 "$installed_binary"; then
    printf '%s\n' "$installed_binary"
    return 0
  fi

  printf 'hooks: installing golangci-lint %s\n' "$required_golangci_lint_version" >&2
  GOFLAGS= go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${required_golangci_lint_version}"

  if [[ -x "$installed_binary" ]] && is_golangci_lint_v2 "$installed_binary"; then
    printf '%s\n' "$installed_binary"
    return 0
  fi

  printf 'hooks: failed to resolve golangci-lint %s\n' "$required_golangci_lint_version" >&2
  return 1
}

if [[ -f "$repo_root/flake.nix" && -z "${IN_NIX_SHELL:-}" ]]; then
  if command -v nix >/dev/null 2>&1; then
    exec nix develop "$repo_root" --command "$@"
  fi

  printf 'hooks: flake.nix detected but nix not found; using current environment\n' >&2
fi

if [[ "${1:-}" == "golangci-lint" ]]; then
  golangci_lint_binary=$(resolve_golangci_lint)
  shift
  exec "$golangci_lint_binary" "$@"
fi

exec "$@"
