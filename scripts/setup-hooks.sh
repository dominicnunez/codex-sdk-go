#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)

if command -v lefthook >/dev/null 2>&1; then
  exec lefthook install
fi

exec nix develop "$repo_root" --command lefthook install
