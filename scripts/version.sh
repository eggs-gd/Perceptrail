#!/usr/bin/env bash
# Monorepo version, derived from git history — nothing to commit per merge.
#
# VERSION (repo root) holds only MAJOR.MINOR and changes only on a release.
# PATCH = number of first-parent commits since VERSION last changed. develop only
# accepts squash merges, so every merged PR is exactly one commit: +1 patch.
#
#   scripts/version.sh          print the version, e.g. 0.1.3
#   scripts/version.sh minor    bump MAJOR.MINOR in VERSION (release-prep PR)
#   scripts/version.sh stamp    write the version into svebapp/package.json of a
#                               build copy (never commit the result)
set -euo pipefail
cd "$(dirname "$0")/.."

base=$(tr -d '[:space:]' < VERSION)

current() {
  if [ "$(git rev-parse --is-shallow-repository)" = "true" ]; then
    echo "version.sh: shallow clone, patch cannot be counted (use fetch-depth: 0)" >&2
    exit 1
  fi
  local since patch
  since=$(git log -1 --format=%H -- VERSION)
  patch=$(git rev-list --count --first-parent "${since}..HEAD")
  echo "${base}.${patch}"
}

case "${1:-}" in
  "") current ;;
  minor)
    IFS=. read -r major minor <<< "$base"
    echo "${major}.$((minor + 1))" > VERSION
    cat VERSION
    ;;
  stamp)
    v=$(current)
    (cd svebapp && npm version "$v" --no-git-tag-version --allow-same-version > /dev/null)
    echo "$v"
    ;;
  *) echo "usage: $0 [minor|stamp]" >&2; exit 2 ;;
esac
