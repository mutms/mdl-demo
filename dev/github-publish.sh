#!/usr/bin/env bash
# Build the multi-arch release image on macOS and publish it to ghcr.io.
#
# The version comes from git: HEAD must be exactly on a vX.Y.Z tag that is
# already pushed to origin, and the working tree must be clean — the script
# refuses to publish anything else, so no "dirty" or "-4-gabc1234" builds
# can ever reach the registry.
#
# Log in first (interactive, keeps credentials out of this script):
#   container registry login ghcr.io
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

container build --arch arm64 --arch amd64 --build-arg VERSION="$TAG" -t "$IMAGE:$TAG" -f containers/base/Containerfile .
container image push "$IMAGE:$TAG"
container image tag "$IMAGE:$TAG" "$IMAGE:latest"
container image push "$IMAGE:latest"

echo "published $IMAGE:$TAG and $IMAGE:latest"
