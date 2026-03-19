#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)

if command -v lefthook >/dev/null 2>&1; then
  exec lefthook install
fi

if ! command -v nix >/dev/null 2>&1; then
  printf '%s\n' 'setup-hooks: neither lefthook nor nix is installed' >&2
  printf '%s\n' 'install lefthook and rerun ./scripts/setup-hooks.sh, or install nix to use the repo dev shell' >&2
  exit 1
fi

exec nix develop "$repo_root" --command lefthook install
