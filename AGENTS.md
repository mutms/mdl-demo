# mdl-demo — agent/contributor brief

A self-contained OCI image: users run one container and get a throwaway
Moodle/MuTMS demo site (Apache on container port 8082) managed by a small
web console (8081). Outside, the console port NNNN (`MDL_DEMO_PORT`, default
8081) is the demo's identity — container `mdl-demo-NNNN`, site on NNNN+1 —
so several demos run side by side. Runs identically, with no special flags,
on rootful podman, Apple `container` (macOS) and WSL containers (Windows);
`launcher/` holds the macOS and Windows user-facing launcher scripts.

## Architecture

One Go binary (`go/`, module `github.com/mutms/mdl-demo/go`) with three jobs:
PID 1 of the container, management web UI, and CLI.

- `cmd/mdl-demo` — subcommand dispatch (`init`, `serve`, `recipes`,
  `install`, `status`, `reset`, `backup`, `restore`, `url`, `cron`, `version`).
- `internal/initd` — **the container's init**: starts and supervises
  postgresql/php-fpm/apache2/mailpit (restart with backoff), central zombie reaper,
  per-minute Moodle cron ticker, ordered shutdown on SIGTERM. The web UI
  runs in-process.
- `internal/svc` — seam for all service actions (Apache reload, cron, status
  for the dashboard and `/debug`): `Supervisor` when called inside PID 1,
  `standalone` (pidfile/`state.json`-based) when the CLI runs via `exec`.
  Never call service tooling directly from feature code — go through this.
- `internal/site` — the install/reset orchestrator (used by CLI and UI).
- `internal/webui` — stdlib HTTP + `html/template` files (`templates/`,
  embedded; one file per named template, parsed in `templates.go`)
  + vendored htmx 2 and Pico CSS 2 (`static/`, see
  `VENDOR.md`; Pico owns the design system, `static/app.css` is theme +
  custom components — deliberately so forks building branded demos restyle
  via Pico's variables instead of untangling custom CSS; `static/app.js`
  holds the few delegated handlers, driven by `data-*` attributes). Nothing
  is inline: `auth.go` sets a Content-Security-Policy that allows only
  same-origin files, beside the CSRF cookie, the Origin and Fetch Metadata
  checks and the Host allow-list (see invariant 10);
  single-flight background job in
  `job.go`; en/cs/de UI strings in `lang.go`; diagnostics on the Settings page
  (`/settings`; `/debug` redirects there). The dashboard's **Tools card** is a
  3×3 grid of navigation cards (each a sub-page): upstream caps it at **8** so a
  fork can drop in its own card (first or last) as the 9th and still fit the
  glanceable grid — beyond that, nest inside a card (as Settings does) rather
  than add tiles. The optional **poster** (an image-baked custom card, see
  `poster.go`/`assets/poster/`) takes the first slot; the **Camp registry** card
  the last. Both are removable, so the grid stays glanceable in every build.
- `internal/tunnel` — the optional Cloudflare Quick Tunnel (one `cloudflared`
  child; rewrites the site's wwwroot to the public URL while it runs).
- `internal/sso` — single-use login tokens behind the console's "Log in…"
  buttons and QR codes; only `sha256(token)` ever reaches the dataroot.
- `internal/camp` — loads and queries the baked Camp registry catalogue
  (camp-registry.org's `camp-index`, ~6,400 plugin YAMLs + advisories, cloned to
  `/srv/extra/camp` at build; ODbL, credited on the Camp page). Powers the
  `/camp` browse/install page and the installed-plugins dialog's "Latest on Camp"
  link + advisory badge. Camp installs reuse the git-URL install path. Two
  boot-time kill switches (no lock-in): `MDL_DEMO_NO_CAMP` removes Camp entirely
  (card, page, links, Settings button — the data is not even loaded) and
  `MDL_DEMO_NO_PLUGIN_URL` disables adding plugins from a URL (the git-URL box
  *and* Camp installs); both enforced in the Go handlers, not just templates.
- `internal/backup` — the `.mdb` backup file format (validation, safe
  extraction); the backup/restore orchestration is in `internal/site`.
  `assets/backups/*.mdb` in the repo is baked into the image at `/srv/backups`
  (pre-bundled demo sites for forks; see `assets/backups/README.md`),
  `assets/recipes/<vendor>/<stream>/<version>.yaml` overlays merge into the
  recipe catalogue at image build (see `assets/recipes/README.md`), and bare git
  repos in `assets/repos/` bake to `/srv/extra/repos` as `file://` recipe sources
  for private/offline demos (see `assets/repos/README.md`). The three fork-asset
  dirs live under `assets/`.
- `internal/moodle`, `internal/apache`, `internal/pgdb`, `internal/recipes`,
  `internal/state`, `internal/execx` — Moodle tree handling, vhost
  generation, DB provisioning, recipe catalogue scan, `/etc/mdl-demo/state.json`
  (demo identity adopted once from `MDL_DEMO_PORT`/`MDL_DEMO_NAME`, the site
  URL override from `mdl-demo url`, installed site), command runner with
  line-streamed logs.
- `container/php/` — the console's Moodle-side PHP: web endpoints at the top
  (e.g. the single-use login handler), CLI scripts under `container/php/cli/`.
  Baked into the image at `/usr/share/mdl-demo/php` and copied into the docroot
  as `mdl-demo/` on site install (root-owned, like the whole tree).
- `launcher/mdl-demo` (bash, Apple `container`) and `launcher/mdl-demo.cmd`
  (pure batch, `wslc`) — `create|start|stop|delete|gc [NNNN]` / `list` / print-only
  `install`|`uninstall`; the only place the outside port mapping and env vars are
  spelled out for users.
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
   console+1 — never add a second port knob. A site URL behind a
   proxy/tunnel comes from `mdl-demo url --site`, not from guessing at Host
   headers; the console has no such override on purpose (invariant 10).
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
   Third-party Go modules are allowed when they earn their place (a QR
   encoder, say) — keep them few and well known, and commit `go.sum`. Each
   new one is the maintainer's call, not an implementation detail to slip
   in: propose the dependency and what it replaces, and wait to be told yes.
   The bar is deliberately lower than mpd's proxy, which is stdlib-only
   because it runs as root on the developer's Mac and must limit the blast
   radius of bad code; this console runs inside a throwaway container, so it
   must be secure, but a well-reviewed dependency is not that kind of risk.
   `mudev` stays a shelled-out binary, not an import (it is shipped and
   promoted as a tool in its own right).
8. **The web UI footer is a GPL-3.0 §5(d) Appropriate Legal Notice** — keep
   it displayed; forks add their copyright beside it.
9. **Windows commands in docs are always one line** (PowerShell backtick
   continuations break on copy-paste).
10. **The console is a local port: it answers only to IP addresses and
    `localhost`, and what guards it lives in `webui/auth.go`** — a
    SameSite=Strict CSRF cookie (double submit, no server-side sessions), an
    Origin check, and a Host allow-list. Do not introduce a credential
    prompt, and do not give the console a setting that lets it answer to a
    hostname: the allow-list is what stops DNS rebinding, and where the port
    is published is what decides who can reach it. The *site* on 8082 is the
    opposite case — it is meant to be shared (`mdl-demo url --site`, Quick
    Tunnel).

    Every URL mdl-demo builds for itself — the CLI's output, the wwwroot a
    CLI install bakes in, the docs, the launchers — uses `127.0.0.1`
    (`state.DefaultHost`), never `localhost`: the ports are published on
    IPv4 only and `localhost` resolves to `::1` first on some machines. The
    allow-list keeps accepting the name because people type it.

    The container boundary is the perimeter, and it is the test for whether a
    proposed defence is worth its complexity. Anything crossing it outward —
    the wider web reaching the user's machine through a published port, as
    DNS rebinding does — is worth defending against. Anything staying inside
    it (the site reaching the console, the console reaching the database) is
    not: both sides are disposable, and an attacker holding one already holds
    everything of value in the other.

## Working on it

```sh
make build test vet fmt-check   # native binary + checks
make image                      # sudo podman build (run from repo root)
make run                        # mpd-VM test container mpd-test-mdl-demo on 6381/6382;
                                # console at http://<vm-ip>:6381, site published by
                                # mpd's caddy as https://site.mdl-demo.<vm>.mpd.test
make hotpatch                   # rebuild the Go binary, swap it into the running
                                # test container and restart — seconds, no image rebuild
```

**Agents: get your own container, don't share.** `make run`/`hotpatch` take a
`PORT` override so several test containers coexist — always work on your own so
you never disturb a human's `mpd-test-mdl-demo` (which may be mid-install).
`PORT=6381` (the default) keeps the bare name; any other port suffixes it:

```sh
make run PORT=6391        # → mpd-test-mdl-demo-6391 on 6391/6392 (uses current image)
make hotpatch PORT=6391   # rebuild + inject + restart THAT container
```

`hotpatch` covers Go/template/CSS/JS (all embedded in the binary); it does NOT
update `container/php/` — rebuild the image (`make image`) for PHP changes. The site, DB
and dataroot survive the restart, and the console's epoch bumps so open browser
tabs reload themselves. Requires the image to exist (`make image`) and an amd64
mpd VM (the target the cp'd binary is built for).

Driving the console from an agent: `curl` via `podman exec` for GET pages
(console binds :8081 inside; the Host allow-list accepts 127.0.0.1), and the
`mdl-demo` CLI for state changes (`install`/`reset`) — cleaner than forging the
console's CSRF+Origin checks. A text browser adds nothing (no JS); leave the
interactive/visual checks (htmx swaps, spinners, the epoch reload) to a human.

End-to-end: `sudo podman exec mpd-test-mdl-demo-<port> mdl-demo install --recipe
moodle/release/<version> --adminpass 'Test1234!'`, then browse the site
(https://site-<port>.mdl-demo.<vm>.mpd.test, or http://127.0.0.1:<port+1> on the
VM). `DEV.md` covers dev-environment setup; multi-arch release builds and
the publish steps live in `dev/github-publish.sh` (run on an Apple silicon Mac).

Typical extension points: new UI feature → handler in
`webui/server.go` + a template file in `webui/templates/` (htmx section
pattern);
new lifecycle behavior → `internal/site` + the `svc` seam; new run-mode
service → `initd.Run`'s proc list. When something fails in a container,
read the web UI's `/debug` page first — it exists for exactly that.
