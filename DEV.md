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

## Building multi-arch images (amd64 + arm64)

**Build on an Apple silicon Mac — it is by far the fastest path.** The arm64
leg builds natively and the amd64 leg runs under Rosetta (fast binary
translation); a full two-arch build takes ~8 minutes even on an M2. The
Containerfile's build stage runs on the build host's native arch
(`--platform=$BUILDPLATFORM`) and cross-compiles Go for `$TARGETARCH`, so Go
never compiles under emulation on any host.

```sh
container build --arch arm64 --arch amd64 --build-arg VERSION=$(git describe --tags --always --dirty) -t mdl-demo -f containers/base/Containerfile .
```

Fallback (Linux, e.g. CI): podman with qemu user emulation — correct but much
slower, since the arm64 apt layers run fully emulated:

```sh
sudo apt-get install -y qemu-user-static binfmt-support
sudo podman build --platform linux/amd64,linux/arm64 \
    --build-arg VERSION=$(git describe --tags --always --dirty) \
    --manifest mdl-demo -f containers/base/Containerfile .
```

(Rosetta is a stopgap until Apple retires it around 2027; by then the CI path
takes over the amd64 leg.)

After the first multi-arch build on a new toolchain, verify neither variant
is an arch chimera (mixed-architecture layers — it has happened): from each
variant, extract `/usr/sbin/apache2` and `/usr/bin/mdl-demo` (`podman create`
+ `podman cp`) and check `file` reports the expected architecture for both.

## Releasing to ghcr.io

Images are published to GitHub Container Registry as
`ghcr.io/mutms/mdl-demo` (the README's run commands point there). Forked the
repo? The same instructions work as-is — just replace `mutms/mdl-demo` with
your own `<owner>/<repo>` in the commands below (and in your README).
Login uses a GitHub PAT with the `write:packages` scope — entered at the
interactive prompt, never on the command line:

From the Mac (recommended, see above):

```sh
container registry login ghcr.io
container build --arch arm64 --arch amd64 --build-arg VERSION=v0.1.1 -t ghcr.io/mutms/mdl-demo:v0.1.1 -f containers/base/Containerfile .
container image push ghcr.io/mutms/mdl-demo:v0.1.1
container image tag ghcr.io/mutms/mdl-demo:v0.1.1 ghcr.io/mutms/mdl-demo:latest
container image push ghcr.io/mutms/mdl-demo:latest
```

From Linux (podman): `sudo podman login ghcr.io`, then `manifest push` the
multi-arch manifest to the same names.

Release checklist:

1. Tag the release (`git tag -a v0.1.0`) so `git describe` and the
   `VERSION` build-arg agree with the image tag.
2. Build with `--build-arg VERSION=v0.1.0` — the binary, the web UI and the
   `/debug` page all report this version.
3. Push `:v0.1.0` and move `:latest`.
4. First push only: the ghcr package is created **private** — make it public
   in the package settings on github.com. The `org.opencontainers.image.source`
   label in the Containerfile links the package to this repo automatically.

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
