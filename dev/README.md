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

Only the site can be moved this way. The console answers to `localhost` and
to IP addresses and nothing else (`go/internal/webui/auth.go`), so it is
always reached at `http://<host-or-ip>:<console port>` — put it behind a
proxy with a hostname and it returns 403. That is deliberate: what decides
who can reach the console is where its port is published, and a name it
would answer to is an invitation to publish it somewhere it should not be.

## Why mdl-demo's own init (no systemd)

The image boots `mdl-demo init` as PID 1: it starts and supervises
postgresql/php-fpm/apache2 with restart backoff, serves the web UI
in-process, runs Moodle cron on a per-minute ticker, reaps orphans and turns
SIGTERM into an ordered shutdown. Service actions (Apache reload, cron
arming, the dashboard status card, the `/debug` diagnostics page) go through
`internal/svc`, which distinguishes the PID-1 supervisor from exec'd CLI
processes.

This is deliberate and was proven the hard way (2026-08-16): standard OCI
runtimes give PID 1 no CAP_SYS_ADMIN, and systemd cannot boot without it —
it cannot even mount tmpfs on /run. Each runtime failed differently:

- **Apple `container`** needed `--cap-add SYS_ADMIN` for systemd (no
  pre-mounted API filesystems).
- **rootful podman** booted systemd only via its special systemd mode — and
  `--cap-add ALL` actively broke systemd 257's generator sandboxing.
- **WSL containers preview** cannot boot systemd at all: default runtime caps
  (CapEff a80425fb), /sys/fs/cgroup mounted cgroup2 read-only, no
  `--cap-add`/`--privileged`/`--cgroupns`, and `-v` arrives as virtiofs,
  which cannot carry a cgroupfs.

The Go init needs none of that: no capabilities, no cgroups, no tmpfs — the
identical flag-free run command works on all three runtimes.

Misbehaving services surface on the web UI's `/debug` page (mode, restart
counts, last exits, log tails) as one copy-pasteable block for bug reports.

The image intentionally has no systemd-resolved (or systemd at all): every
target runtime manages /etc/resolv.conf itself and nss stays `files dns`.

## Running inside an mpd VM

mpd has an `mdl-demo` project type: `mpd start mdl-demo` publishes
`https://mdl-demo.NNN.mpd.test` (console) and `https://site.mdl-demo.NNN.mpd.test`
(site) through the runtime's caddy, with certificate and DNS, pointing at
fixed VM ports 6381 and 6382. `make run` is the other half of that contract:
it removes any previous `mpd-test-mdl-demo` container, starts a new one with
`MDL_DEMO_PORT=6381` bound on the VM's interfaces (the runtime reaches the VM
at its bridge address; the vmnet is host-only), waits for the console, and
records the site's mpd address with `mdl-demo url`. Accept the prefilled
`site.` URL in the install form and the site comes up over https.

The caddy *console* address is the one thing that no longer works: the
console does not answer to a hostname, so `https://mdl-demo.NNN.mpd.test`
returns 403. Open it at `http://<vm bridge ip>:6381` from the Mac instead
(`jq -r .gateway /srv/meta/vm.json` prints the address).

One test demo per VM. Stop it with `sudo podman stop mpd-test-mdl-demo`.

A site installed under one address will not work when visited under another
(redirects/logins break) — `mdl-demo reset` and reinstall when switching.

## Distributing dev builds

To try an unpublished build on another machine, move it as a tarball:

```sh
sudo podman save --format oci-archive -o mdl-demo.tar localhost/mdl-demo
scp mdl-demo.tar <target>:
```

On the target — macOS (Apple `container`):

```sh
container image load --input mdl-demo.tar
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 mdl-demo
```

or Linux (podman): `sudo podman load -i mdl-demo.tar`.

A private OCI registry works too, of course — push with `podman push` and run
the image by its registry name; setting one up is outside this repo's scope.

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
