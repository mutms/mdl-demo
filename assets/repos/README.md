# assets/repos/ — local git repos baked into the image

Git repositories dropped in this directory ship in the image at
`/srv/extra/repos/`. They serve two purposes, both resting on one fact: **`mudev`
(and the image build) clone from local `file://`/path git repos exactly like any
remote.** There is no console UI for this — it is a quiet capability for forks.

**Layout mirrors the git URL, minus the scheme.** A repo at
`https://github.com/acme/moodle-mod_foo` lives here as
`assets/repos/github.com/acme/moodle-mod_foo` and is referenced by
`file:///srv/extra/repos/github.com/acme/moodle-mod_foo`. So turning a public
source into a local one is mechanical: **strip the scheme.** Any host works, not
just GitHub.

**One private plugin.** Bake it at
`assets/repos/github.com/acme/moodle-mod_foo`, then add it through the console's
*Installed plugins → Add a plugin* using
`file:///srv/extra/repos/github.com/acme/moodle-mod_foo`. It installs like any
git plugin and is recorded in `.mudev.json`, so it travels through
backup/restore reproducibly — within images that carry the same repo.

**A fully offline demo.** The build already checks here first: drop a mirror of
`mdl-recipes` (and/or `mdl-plugins`) at
`assets/repos/github.com/mutms/mdl-recipes` and the image clones
`/srv/extra/mdl-recipes` **from your local copy** instead of GitHub. Mirror
everything else the recipes reference — Moodle core
(`assets/repos/github.com/moodle/moodle`) and each plugin repo — and rewrite the
recipe sources to their `file://` equivalents (scheme stripped). Then every `git
clone` (install) and `git pull` (Settings → Update catalogues) stays on the
local filesystem and `mudev clone` needs no internet at all. This is the model,
not a step-by-step; a `dev/` helper that mirrors the repos and builds an
`mdl-demo-offline` image is the natural way to automate it.

Notes:
- Use **absolute** `file://` paths in recipes. `mudev` does not anchor relative
  remote URLs.
- Clones run as root in the container; keep the repos root-owned (or clone with
  `-c safe.directory='*'`) to avoid git's dubious-ownership refusal.
- This README is stripped from the image at build; only the repos ship.
