#!/usr/bin/env bash
# Fast local test loop on macOS: pull, rebuild natively (single arch, no
# push), run as container "mdl-demo-test". Repeat after every change —
# cached layers make code-only rebuilds take seconds.
#
# Optional: export MDL_DEMO_PASSWORD to skip the first-access password form.

set -e
cd "$(dirname "$0")"

container stop mdl-demo-test 2>/dev/null || true
container rm mdl-demo-test 2>/dev/null || true

git pull

container build -t mdl-demo-test \
    --build-arg VERSION="$(git describe --tags --always --dirty)" \
    -f containers/base/Containerfile .

container run -d --name mdl-demo-test \
    ${MDL_DEMO_PASSWORD:+-e MDL_DEMO_PASSWORD="$MDL_DEMO_PASSWORD"} \
    -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 \
    mdl-demo-test:latest

echo "web UI: http://localhost:8081"
