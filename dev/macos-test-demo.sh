#!/usr/bin/env bash
#
# I run this on my Mac to smoke-test the real image before I tag a release.
# Native rebuild (one arch, no push), runs it as "mdl-demo-test", opens the
# console. Poke it, and if it's good, publish with dev/github-publish.sh.
#   --skodak

set -e
# Run against the repo root whatever the caller's cwd: the build context
# "." and the Containerfile path below are repo-root-relative, and this
# script now lives one level down in dev/.
cd "$(dirname "$0")/.."

container stop mdl-demo-test 2>/dev/null || true
container rm mdl-demo-test 2>/dev/null || true

git pull

container build -t mdl-demo-test \
    --build-arg VERSION="$(git describe --tags --always --dirty)" \
    -f container/Containerfile .

container run -d --name mdl-demo-test -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 mdl-demo-test:latest && open "http://127.0.0.1:8081"

echo "web UI: http://127.0.0.1:8081"
