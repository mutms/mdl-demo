# mdl-demo — agent/contributor brief

A self-contained OCI image: users run one container and get a throwaway
Moodle/MuTMS demo site (Apache on container port 8082) managed by a small
web console (8081). Outside, the console port NNNN (`MDL_DEMO_PORT`, default
8081) is the demo's identity — container `mdl-demo-NNNN`, site on NNNN+1 —
so several demos run side by side. Runs identically, with no special flags,
on rootful podman, Apple `container` (macOS) and WSL containers (Windows);
`launcher/` holds the macOS and Windows user-facing launcher scripts.

## Architecture

One Go binary (`go/`, module `github.com/mutms/mdl-demo/go`, stdlib so far) with
three jobs: PID 1 of the container, management web UI, and CLI.

- `cmd/mdl-demo` — subcommand dispatch (`init`, `serve`, `recipes`,
  `install`, `status`, `reset`, `url`, `cron`, `version`).
- `internal/initd` — **the container's init**: starts and supervises
  postgresql/php-fpm/apache2 (restart with backoff), central zombie reaper,
  per-minute Moodle cron ticker, ordered shutdown on SIGTERM. The web UI
  runs in-process.
- `internal/svc` — seam for all service actions (Apache reload, cron, status
  for the dashboard and `/debug`): `Supervisor` when called inside PID 1,
  `standalone` (pidfile/`state.json`-based) when the CLI runs via `exec`.
  Never call service tooling directly from feature code — go through this.
- `internal/site` — the install/reset orchestrator (used by CLI and UI).
- `internal/webui` — stdlib HTTP + `html/template` string constants
  (`templates.go`) + vendored htmx 2 (`static/`, see `VENDOR.md`); sessions,
  CSRF and Origin checks in `auth.go`; single-flight background job in
  `job.go`; diagnostics page at `/debug`.
- `internal/moodle`, `internal/apache`, `internal/pgdb`, `internal/recipes`,
  `internal/state`, `internal/execx` — Moodle tree handling, vhost
  generation, DB provisioning, recipe catalogue scan, `/etc/mdl-demo/state.json`
  (password hash, demo identity adopted once from `MDL_DEMO_PORT`/`MDL_DEMO_NAME`,
  URL overrides from `mdl-demo url`, installed site), command runner with
  line-streamed logs.
- `php/` — the console's Moodle-side PHP: web endpoints at the top (e.g. the
  single-use login handler), CLI scripts under `php/cli/`. Baked into the
  image at `/usr/share/mdl-demo/php` and copied into the docroot as
  `mdl-demo/` on site install (root-owned, like the whole tree).
- `launcher/mdl-demo` (bash, Apple `container`) and `launcher/mdl-demo.cmd`
  (pure batch, `wslc`) — `create|start|stop|delete [NNNN]` / `list`; the only
  place the outside port mapping and env vars are spelled out for users.
- `container/Containerfile` — two-stage build; stage 1 cross-compiles
  Go for `$TARGETARCH`, stage 2 is the runtime image.
- Code assembly is delegated to the [mudev](https://github.com/mutms/mudev)
  binary (`mudev clone <recipe>`), with catalogues at
  `/srv/extra/mdl-{plugins,recipes}` (github.com/mutms/mdl-plugins and
  github.com/mutms/mdl-recipes).

## Invariants — do not break these

1. **No systemd, no required capabilities.** `mdl-demo init` is PID 1 by
   design: standard OCI runtimes give containers no CAP_SYS_ADMIN and often
   read-only cgroups, which systemd cannot boot under. Any change must keep
   the image booting with a flag-free `run` command on all three runtimes —
   and with nothing but the OS vendor's own CLI: the `launcher/` scripts are
   optional sugar, every feature must stay reachable from a plain one-line
   `run` command (env vars + port mappings), which the README always shows.
2. **One demo site per container.** Fixed paths: tree `/srv/projects/demo`,
   dataroot `/srv/data/demo`, database/user/password `demo`. No multi-site.
   Container-internal ports are fixed too (console 8081, site 8082); the
   outside console port is the demo's identity and the site is always
   console+1 — never add a second port knob. URLs behind a proxy/tunnel come
   from `mdl-demo url`, not from guessing at Host headers.
3. **The Moodle code tree stays root-owned; PHP runs as www-data with
   read-only access.** This deliberately disables Moodle's web-based plugin
   installation (plugins come only via recipes/mudev). Never chown the tree
   to www-data, even to "fix" a permission error.
4. **wwwroot = the URL in the user's browser**, chosen at install time and
   never baked into the image. Moodle breaks when visited under another URL.
5. **Moodle layout rules** (5.1+ split): `config.php` and `admin/cli/*` live
   at the tree root; the Apache docroot is `public/` when
   `public/version.php` exists, else the root; resolve every CLI script by
   trying root then `public/`; emit the `/r.php` router rewrite only when
   `r.php` exists — and only inside `<Directory>` context.
6. **Stage 2's `FROM --platform=$TARGETPLATFORM` is load-bearing** for
   multi-arch correctness. See the Containerfile comment before touching it.
7. **The `go` directive in `go/go.mod` picks the compiler** (upstream Go,
   fetched by the go command itself). The image builds on `debian:trixie`
   seeded with a pinned, checksummed Go tarball from go.dev — never
   Debian's `golang-go`, never the `golang` image; release builds pass
   `--pull --no-cache` so the base and packages are current.
   Third-party Go modules are allowed when they earn
   their place (a QR encoder, say) — keep them few and well known, and
   commit `go.sum`. The code happens to be stdlib-only today, but that is
   not a rule here: the stdlib-only discipline comes from mpd's proxy, which
   runs as root on the developer's Mac and must limit the blast radius of
   bad code; this console runs inside a throwaway container, so it must be
   secure but the bar is lower. `mudev` stays a shelled-out binary, not an
   import (it is shipped and promoted as a tool in its own right).
8. **The web UI footer is a GPL-3.0 §5(d) Appropriate Legal Notice** — keep
   it displayed; forks add their copyright beside it.
9. **Windows commands in docs are always one line** (PowerShell backtick
   continuations break on copy-paste).

## Working on it

```sh
make build test vet fmt-check   # native binary + checks
make image                      # sudo podman build (run from repo root)
make run                        # mpd-VM test container mpd-test-mdl-demo on 6381/6382,
                                # published by mpd's caddy as https://mdl-demo.<vm>.mpd.test
```

End-to-end: `sudo podman exec mpd-test-mdl-demo mdl-demo install --recipe
moodle/release/<version> --adminpass 'Test1234!'`, then browse
https://site.mdl-demo.<vm>.mpd.test (or http://127.0.0.1:6382 on the VM). `dev/README.md` has the full verification flow, multi-arch
builds (build on an Apple silicon Mac — fastest) and release steps.

Typical extension points: new UI feature → handler in
`webui/server.go` + template in `templates.go` (htmx section pattern);
new lifecycle behavior → `internal/site` + the `svc` seam; new run-mode
service → `initd.Run`'s proc list. When something fails in a container,
read the web UI's `/debug` page first — it exists for exactly that.
