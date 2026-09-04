#!/usr/bin/env bash
# Build the multi-arch release image on macOS (Apple silicon — both arches
# build natively, no emulation) and publish it to ghcr.io.
#
# The version comes from git: HEAD must be exactly on a vX.Y.Z tag that is
# already pushed to origin, and the working tree must be clean — the script
# refuses to publish anything else, so no "dirty" or "-4-gabc1234" builds
# can ever reach the registry.
#
# Release steps:
#   1. Move the [Unreleased] entries in CHANGELOG.md under a new
#      [X.Y.Z] - YYYY-MM-DD heading; commit and push.
#   2. Tag that commit and push the tag:
#        git tag -a vX.Y.Z -m 'Release vX.Y.Z' && git push origin vX.Y.Z
#   3. On the Mac, check out the tag, log in, and run this script:
#        git fetch --tags && git checkout vX.Y.Z
#        container registry login ghcr.io   # interactive; creds stay out of scripts
#        dev/github-publish.sh
#
# First release only: the new package is private by default — make it public
# in the GitHub package settings, or the README's run commands fail with an
# authentication error.
#
# Forks: IMAGE=ghcr.io/<owner>/<repo> dev/github-publish.sh

set -euo pipefail
# Run against the repo root whatever the caller's cwd: the build context
# "." and the Containerfile path below are repo-root-relative.
cd "$(dirname "$0")/.."

IMAGE="${IMAGE:-ghcr.io/mutms/mdl-demo}"

if [ -n "$(git status --porcelain)" ]; then
    echo "error: working tree is not clean — commit or stash first" >&2
    exit 1
fi

if ! TAG="$(git describe --tags --exact-match HEAD 2>/dev/null)"; then
    echo "error: HEAD is not tagged (got '$(git describe --tags --always)')" >&2
    echo "       tag the release commit first: git tag -a vX.Y.Z -m 'Release vX.Y.Z'" >&2
    exit 1
fi

if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: tag '$TAG' is not a release tag (expected vX.Y.Z)" >&2
    exit 1
fi

if ! remote_tag="$(git ls-remote --tags origin "refs/tags/$TAG")"; then
    echo "error: cannot query origin to check that $TAG is pushed" >&2
    exit 1
fi
if [ -z "$remote_tag" ]; then
    echo "error: tag $TAG is not on origin — push it first: git push origin $TAG" >&2
    exit 1
fi

echo "publishing $IMAGE:$TAG (and :latest)"

# --pull --no-cache: a release is always built on the freshly pulled base
# with current packages, never from stale local layers.
container build --pull --no-cache --arch arm64 --arch amd64 --build-arg VERSION="$TAG" -t "$IMAGE:$TAG" -f container/Containerfile .
container image push "$IMAGE:$TAG"
container image tag "$IMAGE:$TAG" "$IMAGE:latest"
container image push "$IMAGE:latest"

echo "published $IMAGE:$TAG and $IMAGE:latest"
