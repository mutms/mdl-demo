#!/usr/bin/env bash
#
# I run this on my Mac to smoke-test the real image before I tag a release.
# Native rebuild (one arch, no push), runs it as "mdl-demo-test", opens the
# console. Poke it, and if it's good, publish with dev/github-publish.sh.
#   --skodak

set -e
# Paths are relative the mdl-demo/
cd "$(dirname "$0")/.."

container stop mdl-demo-test 2>/dev/null || true
container rm mdl-demo-test 2>/dev/null || true

git pull

# Another demo on this port would answer instead of the new build, and every demo
# started without MDL_DEMO_PORT calls itself mdl-demo-8081 too.
if curl -fs -o /dev/null --max-time 2 http://127.0.0.1:8081/; then
    echo "error: something else already answers on 127.0.0.1:8081, stop it first:" >&2
    container list >&2
    exit 1
fi

VERSION="$(git describe --tags --always --dirty)"

# Same as dev/github-publish.sh: a run here once ended up with an old binary, so
# nothing old may survive, neither the image nor its cached layers.
container image delete mdl-demo-test:latest 2>/dev/null || true
container build --pull --no-cache -t mdl-demo-test \
    --build-arg VERSION="$VERSION" \
    -f container/Containerfile .

container run -d --memory 4g --name mdl-demo-test -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 mdl-demo-test:latest

# A smoke test of the wrong image proves nothing, so make sure it is this one.
RUNNING="$(container exec mdl-demo-test mdl-demo version)"
if [ "$RUNNING" != "mdl-demo $VERSION" ]; then
    echo "error: built $VERSION, but the container runs '$RUNNING'" >&2
    exit 1
fi

open "http://127.0.0.1:8081"

echo "web UI: http://127.0.0.1:8081 ($VERSION)"
