# mdl-demo - OCI image creation and development

Development happens in an mpd VM ([mpd-virt](https://github.com/mutms/mpd-virt)) or any
Linux box with rootful podman, Go (any version — go.mod picks the compiler) and make.

1. install [mpd-virt](https://github.com/mutms/mpd-virt) and create an mpd VM
2. ssh into mpd-NNN-vm and clone mdl-demo into /srv/projects/mdl-demo
3. `make image` — builds the OCI image with rootful podman
4. `make run` — starts the test container; the site is published by mpd as
   https://site.mdl-demo.NNN.mpd.test, the console is at http://VM-IP:6381

## Build & test

```sh
make build        # native bin/mdl-demo (for quick iteration; the image builds its own)
make test vet fmt-check
make image        # sudo podman build -t mdl-demo -f container/Containerfile .
make run          # replaces the mpd-test-mdl-demo container (VM ports 6381/6382, see below)
```

On macOS, `dev/macos-test-demo.sh` is the whole loop: pull, native
single-arch build (no push), fresh `mdl-demo-test` container on the default
ports 8081/8082. Code-only rebuilds finish in seconds thanks to layer caching.

Verification after `make run`:

```sh
sudo podman logs mpd-test-mdl-demo                   # "demo identity: …", "mdl-demo init: supervising …"
sudo podman exec mpd-test-mdl-demo mdl-demo status   # identity + URLs
sudo podman exec mpd-test-mdl-demo mdl-demo recipes
sudo podman exec mpd-test-mdl-demo mdl-demo install --recipe moodle/release/5.2.2 --adminpass 'Test1234!'
curl -s http://127.0.0.1:6382/                       # Moodle front page
```

## Ports and identity

Inside the container the ports never change: console 8081, Moodle 8082.
Outside, the console port is the demo's identity — `MDL_DEMO_PORT` (default
8081), container `mdl-demo-<port>`, site on port+1 — and is recorded in
`/etc/mdl-demo/state.json` at first start together with `MDL_DEMO_NAME`.
The launcher scripts in `launcher/` are the user-facing way to set them; the
README documents the raw `-e`/`-p` form. The macOS launcher can be exercised
on an mpd VM unchanged: mpd ships `/opt/mpd/bin/container`, a podman-backed
stand-in for Apple `container` (same verbs, `ls` output normalised):

```sh
MDL_DEMO_IMAGE=localhost/mdl-demo launcher/mdl-demo create 7777 --name="Shim test"
launcher/mdl-demo list
launcher/mdl-demo delete 7777
```

When something sits in front of the container (mpd's caddy, a
trycloudflare tunnel), the port-derived site URL is wrong; tell the container
the site's public address instead:

```sh
sudo podman exec mpd-test-mdl-demo mdl-demo url --site https://site.x.example
sudo podman exec mpd-test-mdl-demo mdl-demo url --clear
```

The override lives in state.json (temporary like the container) and drives
the install form's suggested site URL and the `/debug` report. An https
wwwroot makes the generated config.php set `$CFG->sslproxy`. Moodle bakes
wwwroot in at install, so set the URL before installing — an installed site
needs a reset to move.

Only the site can be moved this way. The console answers to IP addresses and
to `localhost` and nothing else (`go/internal/webui/auth.go`), so it is
always reached at `http://<host-or-ip>:<console port>` — put it behind a
proxy with a hostname and it returns 403. That is deliberate: what decides
who can reach the console is where its port is published, and a name it
would answer to is an invitation to publish it somewhere it should not be.

## Running inside an mpd VM

use following:

```shell
make image
make run
```

## Layout

- `launcher/` — `mdl-demo` (bash, Apple `container`) and `mdl-demo.cmd`
  (batch, `wslc`): the user-facing `create|start|stop|delete|list` wrappers.
- `container/Containerfile` — the image: Debian trixie with `mdl-demo init`
  as PID 1, Apache+PHP 8.3 (Sury) on 8082 and the console on 8081, local PostgreSQL, the
  [mudev](https://github.com/mutms/mudev) + mdl-demo binaries, and the
  [mdl-recipes](https://github.com/mutms/mdl-recipes) /
  [mdl-plugins](https://github.com/mutms/mdl-plugins) catalogues.
- `container/assets/` — Apache placeholder vhost shown before a site is installed.
- `go/` — the mdl-demo Go module (`cmd/mdl-demo` + `internal/*`).

One demo site per container: paths are fixed (`/srv/projects/demo`, `/srv/data/demo`,
database `demo`). A different Moodle version = new container.

## Releasing OCI packages to ghcr.io

Images are published to GitHub Container Registry as
`ghcr.io/mutms/mdl-demo` (the README's run commands point there). Releases
are built on an Apple silicon Mac with `dev/github-publish.sh`, which builds
both architectures natively — no emulation — and pushes `:vX.Y.Z` and
`:latest`.

The script takes the version from git and refuses to run unless HEAD is
exactly on a `vX.Y.Z` tag that is already on origin and the working tree is
clean, so an unreleased or modified tree can never reach the registry.

Release checklist:

1. Move the `[Unreleased]` entries in `CHANGELOG.md` under a new
   `[X.Y.Z] - YYYY-MM-DD` heading, commit and push.
2. Tag that commit and push the tag:
   `git tag -a vX.Y.Z -m 'Release vX.Y.Z' && git push origin vX.Y.Z`
3. On the Mac, check out the tag, log in (interactive — credentials never
   go into scripts) and publish:

```sh
git fetch --tags && git checkout vX.Y.Z
container registry login ghcr.io
dev/github-publish.sh
```

First release only: the new package is private by default — make it public
in the package settings on GitHub, otherwise the README's run commands fail
with an authentication error.

Forked the repo? Set `IMAGE` to your own registry name
(`IMAGE=ghcr.io/<owner>/<repo> dev/github-publish.sh`) and replace
`mutms/mdl-demo` in your README's run commands.
