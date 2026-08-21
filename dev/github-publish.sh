#!/usr/bin/env bash
# Login first.
#container registry login ghcr.io

# TODO: fetch current tag and validate it
# TODO: use the tag for release

# Run against the repo root whatever the caller's cwd: the build context
# "." and the Containerfile path below are repo-root-relative, and this
# script now lives one level down in dev/.
cd "$(dirname "$0")/.." || exit 1

container build --arch arm64 --arch amd64 --build-arg VERSION=v0.1.2 -t ghcr.io/mutms/mdl-demo:v0.1.2 -f containers/base/Containerfile .
container image push ghcr.io/mutms/mdl-demo:v0.1.2
container image tag ghcr.io/mutms/mdl-demo:v0.1.2 ghcr.io/mutms/mdl-demo:latest
container image push ghcr.io/mutms/mdl-demo:latest
