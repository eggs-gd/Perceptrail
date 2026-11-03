#!/usr/bin/env bash
# The landing's site from both branches' checkouts (.github/workflows/pages.yml):
# develop's under /develop, master's at the root — or develop's there too while master
# has no landing/ yet (before the first release with it). Any build that fails fails
# the site: a broken master is never replaced by develop's page.
#
#   scripts/landing-site.sh <develop checkout> <master checkout> <out>
set -euo pipefail
develop=$1 master=$2 out=$3

build() { # checkout, channel, base path, out
  (cd "$1/landing" && npm ci && LANDING_CHANNEL=$2 BASE_PATH=$3 npm run build)
  mkdir -p "$4" && cp -R "$1/landing/build/." "$4"
}

build "$develop" develop /develop "$out/develop"
if [ -d "$master/landing" ]; then
  build "$master" stable "" "$out"
else
  echo "landing-site: master has no landing/ yet, the root is develop's"
  build "$develop" develop "" "$out"
fi
echo perceptrail.eggs.gd > "$out/CNAME"
