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

Verification after `make run`:

```sh
sudo podman exec demo systemctl is-system-running   # expect: running
sudo podman exec demo mdl-demo recipes
sudo podman exec demo mdl-demo install --recipe moodle/release/5.2.2 --adminpass 'Test1234!'
curl -s http://127.0.0.1:8080/                       # Moodle front page
```

Note the run flags differ per runtime:

- **rootful podman**: no `--cap-add` — podman's systemd mode handles cgroups, and
  `--cap-add ALL` breaks systemd 257's generator sandboxing ("Failed to fork off
  sandboxing environment … Protocol error").
- **Apple `container`**: needs `--cap-add SYS_ADMIN`. It does not pre-mount
  systemd's API filesystems the way podman does, so PID 1 mounts them itself
  and mount(2) needs SYS_ADMIN — without it systemd dies immediately
  ("Failed to mount tmpfs on /run … Failed to mount API filesystems") and
  the container goes straight to stopped. SYS_ADMIN alone suffices — do not
  use `--cap-add ALL`.
- **WSL containers (`wslc` preview)**: cannot boot systemd (tested
  2026-08-16): wslc grants Docker's default capability set (no SYS_ADMIN;
  CapEff a80425fb), mounts /sys/fs/cgroup as cgroup2 **read-only**, and
  exposes no `--cap-add`, `--privileged` or `--cgroupns`. `--tmpfs /run`
  gets systemd past the API filesystems, but it then dies on
  "Failed to create /init.scope control group: Read-only file system".
  Bind workarounds are impossible by construction: `-v` sources arrive in
  the container's utility VM as virtiofs (file sharing), which cannot carry
  a kernel control filesystem like cgroupfs. Until WSL gains Docker-parity
  cgroup handling, WSL uses the image's **systemd-free boot** instead:
  `mdl-demo init` as the entrypoint runs its own PID-1 supervisor (no
  capabilities, no cgroups needed — see "Boot modes" below).

## Boot modes

The image boots two ways from the same bits:

- **systemd (default ENTRYPOINT)** — podman, Apple `container`: a normal
  Debian boot; services are systemd units, Moodle cron is a systemd timer.
- **`mdl-demo init`** (entrypoint override, runtime must pre-mount a tmpfs
  on /run) — for runtimes that cannot boot systemd (WSL preview): mdl-demo
  runs as PID 1, starts and supervises postgresql/php-fpm/apache2 with
  restart backoff, serves the web UI in-process, replaces the cron timer
  with a per-minute ticker, reaps orphans and handles the stop signal.
  Service actions (Apache reload, cron arming, the dashboard's status card,
  the /debug diagnostics page) go through `internal/svc`, which picks the
  systemd or supervisor implementation per boot.

```sh
sudo podman run -d --name demo --tmpfs /run --entrypoint /usr/bin/mdl-demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 mdl-demo init
```

Misbehaving services surface on the web UI's `/debug` page (mode, restart
counts, last exits, log tails) as one copy-pasteable block for bug reports —
in both boot modes.

The image intentionally has no systemd-resolved: every target runtime manages
/etc/resolv.conf itself and nss stays `files dns`.

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

The image is not published anywhere yet. To try a build on another machine,
move it as a tarball:

```sh
sudo podman save --format oci-archive -o mdl-demo.tar localhost/mdl-demo
scp mdl-demo.tar <target>:
```

On the target — macOS (Apple `container`):

```sh
container image load --input mdl-demo.tar
container run -d --name demo --cap-add SYS_ADMIN \
    -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 mdl-demo
```

or Linux (podman): `sudo podman load -i mdl-demo.tar`.

A private OCI registry works too, of course — push with `podman push` and run
the image by its registry name; setting one up is outside this repo's scope.

## Multi-arch (amd64 + arm64)

The build stage runs on the build host's native arch (`--platform=$BUILDPLATFORM`) and
cross-compiles Go for `$TARGETARCH`, so only the apt layers ever run emulated.

Dev loop: plain native `make image`. Publish-time:

```sh
sudo apt-get install -y qemu-user-static binfmt-support
sudo podman build --platform linux/amd64,linux/arm64 \
    --manifest <registry>/mdl-demo:latest -f containers/base/Containerfile .
sudo podman manifest push <registry>/mdl-demo:latest
```

Escape hatch / cross-check: native arm64 `container build` on an Apple Silicon Mac.
Publishing of images to a public registry (GitHub/Docker Hub) will be set up later;
relying on Rosetta on macOS is a stopgap until it gets retired in 2027.

## Layout

- `containers/base/Containerfile` — the image: Debian trixie + systemd PID 1, Apache+PHP 8.3
  (Sury) on 8080, local PostgreSQL, mudev + mdl-demo binaries, recipe/plugin catalogues.
- `containers/base/units/` — systemd units (mdl-demo web UI, Moodle cron timer).
- `containers/base/assets/` — Apache placeholder vhost shown before a site is installed.
- `go/` — the mdl-demo Go module (`cmd/mdl-demo` + `internal/*`).

One demo site per container: paths are fixed (`/srv/projects/demo`, `/srv/data/demo`,
database `demo`). A different Moodle version = new container.
