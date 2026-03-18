#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)

if [[ -f "$repo_root/flake.nix" && -z "${IN_NIX_SHELL:-}" ]]; then
  if command -v nix >/dev/null 2>&1; then
    exec nix develop "$repo_root" --command "$@"
  fi

  printf 'hooks: flake.nix detected but nix not found; using current environment\n' >&2
fi

exec "$@"
