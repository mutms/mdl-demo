# mdl-demo - OCI image creation and development

Development happens in an mpd VM ([mpd-virt](https://github.com/mutms/mpd-virt)) or any
Linux box with rootful podman and Go 1.24+ (Debian trixie: `apt-get install golang-go make`).

1. install [mpd-virt](https://github.com/mutms/mpd-virt) and create an mpd VM
2. ssh into mpd-NNN-vm and clone mdl-demo into /srv/projects/mdl-demo
3. `make image` — builds the OCI image with rootful podman
4. `make run` — starts a throwaway instance on localhost:8080/8081

## Build & test

```sh
make build        # native bin/mdl-demo (for quick iteration; the image builds its own)
make test vet fmt-check
make image        # sudo podman build -t mdl-demo -f containers/base/Containerfile .
make run          # sudo podman run -d --name demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 mdl-demo
```

On macOS, `./rebuild-mdl-demo-test.sh` is the whole loop: pull, native
single-arch build (no push), fresh `mdl-demo-test` container on the same
ports. Code-only rebuilds finish in seconds thanks to layer caching.

Verification after `make run`:

```sh
sudo podman logs demo                                # "mdl-demo init: supervising …"
sudo podman exec demo mdl-demo recipes
sudo podman exec demo mdl-demo install --recipe moodle/release/5.2.2 --adminpass 'Test1234!'
curl -s http://127.0.0.1:8080/                       # Moodle front page
```

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

## Browsing a VM-hosted demo from the host

When the demo runs under podman inside an mpd dev VM, bind the ports on the
VM's interfaces instead of loopback and browse from the host over the vmnet
host-only network (only the host and sibling VMs can reach it; 8081 is
password-protected):

```sh
sudo podman run -d --name demo -p 8080:8080 -p 8081:8081 mdl-demo
# host browser: http://10.163.NNN.1:8081  (or the VM's mpd.test name)
```

wwwroot must match the URL the browser uses — Moodle bakes it in at install:

- Web UI: handled automatically — the install form prefills the hostname from
  the Host header of your 8081 request, so open the UI at the address you will
  browse the site from and accept the prefill.
- CLI: pass it explicitly, e.g. `--wwwroot http://10.163.NNN.1:8080`.

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
container run -d --name demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 mdl-demo
```

or Linux (podman): `sudo podman load -i mdl-demo.tar`.

A private OCI registry works too, of course — push with `podman push` and run
the image by its registry name; setting one up is outside this repo's scope.

## Layout

- `containers/base/Containerfile` — the image: Debian trixie with `mdl-demo init`
  as PID 1, Apache+PHP 8.3 (Sury) on 8080, local PostgreSQL, the
  [mudev](https://github.com/mutms/mudev) + mdl-demo binaries, and the
  [mdl-recipes](https://github.com/mutms/mdl-recipes) /
  [mdl-plugins](https://github.com/mutms/mdl-plugins) catalogues.
- `containers/base/assets/` — Apache placeholder vhost shown before a site is installed.
- `go/` — the mdl-demo Go module (`cmd/mdl-demo` + `internal/*`).

One demo site per container: paths are fixed (`/srv/projects/demo`, `/srv/data/demo`,
database `demo`). A different Moodle version = new container.

## Releasing OCI packages to ghcr.io

Images are published to GitHub Container Registry as
`ghcr.io/mutms/mdl-demo` (the README's run commands point there). Forked the
repo? The same instructions work as-is — just replace `mutms/mdl-demo` with
your own `<owner>/<repo>` in the commands below (and in your README).

Release checklist:

1. Replace the versions in the following commands to match the release tag
2. Update CHANGELOG.md and commit/push to GitHub repo
3. Tag the release commit and push git tag to GitHub
4. Run the following commands from macOS to build and publish OCI images:

```sh
container registry login ghcr.io

container build --arch arm64 --arch amd64 --build-arg VERSION=v0.1.2 -t ghcr.io/mutms/mdl-demo:v0.1.2 -f containers/base/Containerfile .
container image push ghcr.io/mutms/mdl-demo:v0.1.2
container image tag ghcr.io/mutms/mdl-demo:v0.1.2 ghcr.io/mutms/mdl-demo:latest
container image push ghcr.io/mutms/mdl-demo:latest
```
