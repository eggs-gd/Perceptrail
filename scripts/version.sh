#!/usr/bin/env bash
# Monorepo version: the single source of truth is VERSION in the repo root.
# It is synced into the client (svebapp/package.json + lock) and the server
# (gontroller/pkg/app/version.go).
#
#   scripts/version.sh patch   bump 0.0.x (a PR into develop) and sync
#   scripts/version.sh minor   bump 0.x.0 (the develop → master PR) and sync
#   scripts/version.sh sync    write VERSION into svebapp and gontroller
#   scripts/version.sh check   fail if they are out of sync (CI)
set -euo pipefail
cd "$(dirname "$0")/.."

version=$(tr -d '[:space:]' < VERSION)
go_file=gontroller/pkg/app/version.go
pkg_file=svebapp/package.json

bump() {
  IFS=. read -r major minor patch <<< "$version"
  case "$1" in
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
  esac
  version="$major.$minor.$patch"
  echo "$version" > VERSION
}

sync() {
  printf 'package app\n\n// Version is synced from the repo-root VERSION file by scripts/version.sh.\nconst Version = "%s"\n' \
    "$version" > "$go_file"
  (cd svebapp && npm version "$version" --no-git-tag-version --allow-same-version > /dev/null)
  echo "$version"
}

check() {
  local ok=0
  grep -q "const Version = \"$version\"" "$go_file" || { echo "$go_file is not $version"; ok=1; }
  grep -q "\"version\": \"$version\"" "$pkg_file" || { echo "$pkg_file is not $version"; ok=1; }
  [ "$ok" -eq 0 ] && echo "version $version in sync"
  return "$ok"
}

case "${1:-}" in
  patch | minor) bump "$1"; sync ;;
  sync) sync ;;
  check) check ;;
  *) echo "usage: $0 patch|minor|sync|check" >&2; exit 2 ;;
esac
