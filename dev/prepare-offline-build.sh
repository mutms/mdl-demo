#!/usr/bin/env bash
#
# Prep step for an offline demo image. It does NOT build anything - it just
# fills assets/ with local git mirrors of everything a demo needs, then gets out
# of the way so I build the normal way:
#
#       bash dev/prepare-offline-build.sh
#       bash dev/macos-test-demo.sh        # or github-publish.sh - same assets/
#
# All temporary stuff is in mdl-demo/temp/offlineassets/. The mirrors it drops
# into assets/repos/ are HUGE (moodle core alone is gigabytes) - do NOT commit
# them, do NOT push the resulting image. When you're done testing, undo with:
#
#       bash dev/prepare-offline-build.sh --clean
#
# This works in stages:
#  1. clone mdl-plugins and mdl-recipes as real git repos
#  2. clone bare git mirrors of everything they reference into assets/repos/
#  3. create a local branch `offlinedemo` in both catalogues
#  4. rewire the recipes to clone from /srv/extra/repos/ when the image is live
#
# The catalogues in /srv/extra/mdl-recipes are themselves cloned (inside the
# image) from the `offlinedemo` branch baked into assets/repos/github.com/mutms/
# mdl-recipes - same for mdl-plugins - so the Settings-page "git pull" keeps
# working, just against the local fs.
#
#   --skodak
#

set -euo pipefail
# Paths are relative the mdl-demo/
cd "$(dirname "$0")/.."

TEMP="temp/offlineassets"   # under /temp/ so it's gitignored
BRANCH="offlinedemo"

# temp/ is pure scratch - fresh catalogue clones I branch, rewrite, then mirror
# into assets/repos/. Once the bare mirrors exist it's dead weight, so drop it on
# the way out (success or abort). The bare mirror is where offlinedemo lives on.
trap 'rm -rf "$TEMP"' EXIT

# Wipe assets/repos/ back to just its README. The mirrors are gitignored, so
# `git reset` won't touch them and plain `git clean` skips them too - it needs
# -x to clear ignored files. Scoped to assets/repos so it can't eat other work;
# tracked README survives. This is only for --clean: a normal run KEEPS the
# mirrors so the next build reuses them (skip the re-clone, keep podman's layer
# cache warm) - that's the whole speed trick.
reset_repos() {
    git clean -fdx assets/repos >/dev/null
}

if [ "${1:-}" = "--clean" ]; then
    reset_repos   # temp/ is removed by the EXIT trap
    echo "cleaned: assets/repos/ back to just the README, temp/ gone"
    exit 0
fi

rm -rf "$TEMP"   # in case a hard-killed prior run left scratch behind
mkdir -p "$TEMP"

# --- 1. the two catalogues, as real working repos I can branch and rewrite ---
for c in mdl-recipes mdl-plugins; do
    git clone "https://github.com/mutms/$c" "$TEMP/$c"
done

# --- 2. mirror every git remote the catalogues point at ---
# Remotes live under source.git.remotes.* (core in mdl-recipes, plugins in
# mdl-plugins) and they ALL end in .git - homepage/$schema URLs don't, so a
# ".git" at a word boundary is the whole filter. One bare mirror each, laid out
# <host>/<org>/<repo> (scheme and .git stripped) - the exact path the
# Containerfile looks for. The moodle core mirror is the monster - full history,
# gigabytes - that's the price of a truly offline demo!!! Trim it by hand if you
# only ever show a couple of versions.
grep -rhoE 'https?://[^ "#]+\.git\b' "$TEMP/mdl-recipes" "$TEMP/mdl-plugins" assets/recipes 2>/dev/null \
    | sort -u > "$TEMP/urls.txt" || true

while IFS= read -r url; do
    [ -n "$url" ] || continue
    dest="assets/repos/$(printf '%s' "$url" | sed -E 's#^https?://##; s#\.git$##')"
    [ -d "$dest" ] && continue
    mkdir -p "$(dirname "$dest")"
    git clone --mirror "$url" "$dest"
done < "$TEMP/urls.txt"

# --- 3 & 4. an `offlinedemo` branch in each catalogue, remotes repointed local ---
# https://host/org/repo.git  ->  file:///srv/extra/repos/host/org/repo
# When the image is live these resolve to the baked mirrors, so `mudev clone` and
# every "git pull" on the Settings page stay on the local fs - zero internet.
for c in mdl-recipes mdl-plugins; do
    git -C "$TEMP/$c" checkout -b "$BRANCH"
    find "$TEMP/$c" -name '*.yaml' -exec \
        perl -pi -e 's|https?://([^\s"#]+?)\.git\b|file:///srv/extra/repos/$1|g' {} +
    git -C "$TEMP/$c" commit -am "Offline demo: clone from baked local mirrors"
    # the Containerfile clones each catalogue from here and checks out its HEAD -
    # so make that HEAD the offline branch (a --mirror carries every ref along).
    # Always refreshed (cheap) so recipe edits propagate even on a repeat run.
    rm -rf "assets/repos/github.com/mutms/$c"
    git clone --mirror "$TEMP/$c" "assets/repos/github.com/mutms/$c"
    git -C "assets/repos/github.com/mutms/$c" symbolic-ref HEAD "refs/heads/$BRANCH"
done

echo
echo "assets/repos/ is loaded with local mirrors. Now build the normal way:"
echo "    bash dev/macos-test-demo.sh"
echo "When done, undo it (do NOT commit the mirrors):"
echo "    bash dev/prepare-offline-build.sh --clean"
