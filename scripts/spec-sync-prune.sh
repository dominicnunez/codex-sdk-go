#!/usr/bin/env bash
set -euo pipefail

current_num="${1:?current PR number is required}"
short_upstream="${2:?upstream SHA is required}"

# Use the paginated REST endpoint: its bot login is github-actions[bot].
# gh pr list uses GraphQL and represents that same author as app/github-actions.
gh api --paginate "repos/${GITHUB_REPOSITORY:?}/pulls?state=open&per_page=100" \
  --jq '.[] | [.number, .user.login, .head.ref, (.head.repo.full_name == .base.repo.full_name)] | @tsv' |
while IFS=$'\t' read -r num author head_ref same_repo; do
  [ -z "${num}" ] && continue
  [ "${author}" = "github-actions[bot]" ] || continue
  [ "${same_repo}" = "true" ] || continue
  case "${head_ref}" in
    spec-sync/*) ;;
    *) continue ;;
  esac
  [ "${num}" != "${current_num}" ] || continue

  gh pr close "${num}" --delete-branch --comment "Superseded by newer spec-sync PR #${current_num} for ${short_upstream}."
done
