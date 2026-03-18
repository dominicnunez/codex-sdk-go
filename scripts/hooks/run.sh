#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)

if [[ -f "$repo_root/flake.nix" && -z "${IN_NIX_SHELL:-}" ]]; then
  exec nix develop "$repo_root" --command "$@"
fi

exec "$@"
