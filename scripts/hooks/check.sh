#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: check.sh pre-commit|pre-push}
case "$mode" in pre-commit|pre-push) ;; *) echo "unsupported hook: $mode" >&2; exit 1 ;; esac
repo_root=$(git rev-parse --show-toplevel)
runner="$repo_root/scripts/hooks/run.sh"

# The dedicated module ignores working module contents while allowing the
# checker to reuse the repository's process-tree controls.
temporary=$(mktemp -d)
child=
building=1
cleanup() { rm -rf -- "$temporary"; }
stop_child() {
  trap '' TERM INT HUP
  if [[ -n "$child" ]]; then
    if [[ "$building" = 1 ]]; then
      echo "$mode: waiting for bootstrap build before stopping" >&2
    else
      kill -TERM "$child" 2>/dev/null || true
    fi
    wait "$child" 2>/dev/null || true
  fi
  exit 1
}
trap cleanup EXIT
trap stop_child TERM INT HUP
run_child() {
  local interrupted=0
  trap 'interrupted=1' TERM INT HUP
  "$@" <&0 &
  child=$!
  trap stop_child TERM INT HUP
  if [[ "$interrupted" = 1 ]]; then stop_child; fi
  wait "$child"
  child=
}
if [[ "$mode" = pre-push ]]; then
  building=0
  run_child cat > "$temporary/updates"
fi
building=1
run_child env GO111MODULE=on GOWORK=off GOENV=off GOFLAGS= "$runner" \
  go -C "$repo_root" build -modfile="$repo_root/scripts/hooks/bootstrap.mod" -buildvcs=false \
  -o "$temporary/checker.exe" ./scripts/hooks </dev/null
building=0
if [[ "$mode" = pre-push ]]; then
  run_child "$temporary/checker.exe" "$mode" < "$temporary/updates"
else
  run_child "$temporary/checker.exe" "$mode"
fi
