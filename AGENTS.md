# mdl-demo — agent/contributor brief

A self-contained OCI image: users run one container and get a throwaway
Moodle/MuTMS demo site (Apache on 8080) managed by a small web UI (8081).
Runs identically, with no special flags, on rootful podman, Apple
`container` (macOS) and WSL containers (Windows).

## Architecture

One Go binary (`go/`, module `github.com/mutms/mdl-demo`, stdlib only) with
three jobs: PID 1 of the container, management web UI, and CLI.

- `cmd/mdl-demo` — subcommand dispatch (`init`, `serve`, `recipes`,
  `install`, `status`, `reset`, `cron`, `version`).
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
  generation, DB provisioning, recipe catalogue scan, `/etc/mdl-demo/state.json`,
  command runner with line-streamed logs.
- `containers/base/Containerfile` — two-stage build; stage 1 cross-compiles
  Go for `$TARGETARCH`, stage 2 is the runtime image.
- Code assembly is delegated to the [mudev](https://github.com/mutms/mudev)
  binary (`mudev clone <recipe>`), with catalogues at
  `/srv/extra/mdl-{plugins,recipes}` (github.com/mutms/mdl-plugins and
  github.com/mutms/mdl-recipes).

## Invariants — do not break these

1. **No systemd, no required capabilities.** `mdl-demo init` is PID 1 by
   design: standard OCI runtimes give containers no CAP_SYS_ADMIN and often
   read-only cgroups, which systemd cannot boot under. Any change must keep
   the image booting with a flag-free `run` command on all three runtimes.
2. **One demo site per container.** Fixed paths: tree `/srv/projects/demo`,
   dataroot `/srv/data/demo`, database/user/password `demo`. No multi-site.
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
7. **Stdlib only; `GOTOOLCHAIN=local` (Debian trixie Go 1.24).** No new Go
   dependencies, no toolchain downloads; shell out to `mudev`, do not import it.
8. **The web UI footer is a GPL-3.0 §5(d) Appropriate Legal Notice** — keep
   it displayed; forks add their copyright beside it.
9. **Windows commands in docs are always one line** (PowerShell backtick
   continuations break on copy-paste).

## Working on it

```sh
make build test vet fmt-check   # native binary + checks
make image                      # sudo podman build (run from repo root)
make run                        # flag-free podman run; UI on :8081
```

End-to-end: `sudo podman exec demo mdl-demo install --recipe
moodle/release/<version> --adminpass 'Test1234!'`, then browse
http://localhost:8080. `dev/README.md` has the full verification flow, multi-arch
builds (build on an Apple silicon Mac — fastest) and release steps.

Typical extension points: new UI feature → handler in
`webui/server.go` + template in `templates.go` (htmx section pattern);
new lifecycle behavior → `internal/site` + the `svc` seam; new run-mode
service → `initd.Run`'s proc list. When something fails in a container,
read the web UI's `/debug` page first — it exists for exactly that.
