#!/usr/bin/env bash
#
# This is how I build the official images. No CI, no build farm — I do it by
# hand on my Mac. Apple silicon builds both arches natively, it's fast, and I
# like watching it. Every ghcr.io/mutms/mdl-demo image came out of this script.
# Want your own images? Change IMAGE, run it. That's the whole trick.
#
# Set up new computer:
#
#       cd ~/Developer
#       git clone git@github.com:mutms/mdl-demo.git
#       cd ~/Developer/mdl-demo
#       container registry login ghcr.io
#
# Release steps:
#
#   1. In mpd VM update CHANGELOG.md [Unreleased] to match actual release tag!!!
#   2. Tag that commit and push the tag to github
#   3. On the Mac:
#       cd ~/Developer/mdl-demo
#       git pull
#       bash dev/macos-test-demo.sh
#       bash dev/github-publish.sh
#
#   --skodak

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
