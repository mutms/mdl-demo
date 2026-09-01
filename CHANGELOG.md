# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-09-01

### Added

- `--open` on both launchers waits for the console to come up and opens it in
  the browser, on `create` and `start`

### Changed

- The management console has no password: no setup form on a new container,
  no login page, no `MDL_DEMO_PASSWORD`, no launcher `--password`
- `mdl-demo url` overrides the site URL only — the console URL is always
  derived from its port

### Security

- The console answers only to `localhost` and to IP addresses, so a web page
  cannot reach it by pointing a name it owns at 127.0.0.1 (DNS rebinding)
- CSRF protection is now a SameSite=Strict double-submit cookie, so a console
  restart no longer invalidates a page someone has open

### Fixed

- A backup listing that cannot be read says so instead of showing up empty

## [0.3.0] - 2026-08-30

### Added

- Backup and restore of demo sites as portable, self-contained .mdb files
- Backups page: create, download, upload and delete backups
- Restore into a different recipe (e.g. a 4.5 backup into a 5.3 site)
- Backup files can be pre-bundled in the container image
- Recipe overlay directory for shipping custom recipes in the image
- Get-started dashboard: recipe and backup chooser when no site is installed
- Quick Tunnel status on the diagnostics page

### Changed

- Restore always generates fresh passwords for all known demo accounts
- Outdated point releases are folded away in the recipe chooser
- Calmer row actions: links and icons instead of buttons everywhere
- Site log badge names the running activity instead of a bare "running"
- Czech and German texts use en-dashes
- Web UI templates split into individual embedded files

### Security

- All services run with no_new_privs, making setuid binaries inert

### Fixed

- Logged-in IP addresses show the real visitor, not 127.0.0.1
- Apache redirects no longer leak the internal http://…:8082 address
- Site language packs are installed by the Moodle installer itself

## [0.2.0] - 2026-08-29

### Added

- Added a Quick Tunnel button (Cloudflare, try.cloudflare.com): one click
  exposes the demo site on a public trycloudflare.com URL — with a QR code
  popup for audiences — and points Moodle at it until the tunnel stops
- Added single-use login links to the Accounts card: the Log in… button opens
  a dialog with a direct new-tab login and a one-time QR code (closes when
  claimed) so an audience can take turns on a demo account without passwords
- Added a Create user… button to the Accounts card: demo accounts with
  generated passwords and an optional global role (Manager or a second site
  administrator), ready for the single-use login links
- Added Czech and German console translations: detected from the browser
  language, switchable in the header; installing a site also installs the
  matching Moodle language pack and makes it the site default
- Added a dark mode toggle in the header (auto → light → dark)
- Added a built-in mail catcher (Mailpit): every mail the demo site sends is
  viewable from the console's Mail card and nothing leaves the container
- Restyled the console on vendored Pico CSS 2: Pico owns typography, forms,
  tables, buttons, dialogs and dark mode; a small theme keeps the console's
  identity (badges, log pane, masked credentials)
- Added setting for custom ports
- Added setting for demo name
- Added launcher scripts for macOS and Windows
- Added support for development and testing in mpd
- Added demo launchers

### Changed

- Redesigned developer docs and helper scripts
- Switched to go 1.27.0
- Simplified directory structure
- Improved docs

### Fixed

- User credentials are masked

## [0.1.2] - 2026-08-19

### Changed

- Assemble the Moodle tree with `mudev clone --shallow` to improve performance
- Management UI visual improvements

### Fixed

- Removed default management password from README
- Fixed custom init system race conditions
- More reliable site reset command
- mudev binary is built with appropriate version
- Redesigned log display area
- Fixed display issues when container rebuilt when browser open

## [0.1.1] - 2026-08-18

### Fixed

- Fixed mudev compile error

## [0.1.0] - 2026-08-18

### Added

- First preview release
