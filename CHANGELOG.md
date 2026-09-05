# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add a plugin to the demo from any git repository, with the matching version pre-selected
- Support for offline mode
- Support for custom poster tool card
- Integration of CAMP plugin registry

### Changed

- Restore offers two choices only: keep the current code, or rebuild the backup's code

## [0.5.0] - 2026-09-05

### Added

- Settings page to update the plugin and recipe catalogues from git
- Installed plugins grouped by type with a per-plugin detail view
- Recommendations page of projects and companies the maintainer likes
- IOMAD demos in the installer
- Restore option to keep the current code and skip the download
- Moodle version shown on the demo site card
- Name a backup before creating it, with the default prefilled
- `MDL_DEMO_HIDE_PORT` to hide the port in the top bar for screen recordings

### Changed

- Sticky top bar with breadcrumb navigation and a live job-status indicator
- Unified dialogs and floating-label forms across the console
- Diagnostics and bug reporting moved onto the Settings page
- Launchers gained `gc`, `install` and `uninstall`

### Fixed

- Cron is no longer reported as a failed service before the first install
- A background reload no longer closes an open dialog

## [0.4.1] - 2026-09-02

### Changed

- Switched to Go 1.27.1
- Use `127.0.0.1` instead of `localhost`

## [0.4.0] - 2026-09-01

### Added

- Launchers accept `--open` to open the console in the browser after `create` or `start`

### Changed

- Removed the console password, setup form, login page, `MDL_DEMO_PASSWORD` and launcher `--password`
- `mdl-demo url` sets the site URL only; the console URL always comes from its port

### Security

- The console answers only to `localhost` and IP addresses to block DNS rebinding
- CSRF protection uses a SameSite=Strict double-submit cookie that survives console restarts

### Fixed

- Unreadable backup listings show an error instead of an empty list

## [0.3.0] - 2026-08-30

### Added

- Backup and restore of demo sites as portable .mdb files
- Backups page to create, download, upload and delete backups
- Restore into a different recipe, e.g. a 4.5 backup into a 5.3 site
- Backup files can be bundled in the container image
- Recipe overlay directory for custom recipes in the image
- Get-started dashboard with recipe and backup chooser when no site is installed
- Quick Tunnel status on the diagnostics page

### Changed

- Restore generates fresh passwords for all known demo accounts
- Outdated point releases are folded away in the recipe chooser
- Row actions use links and icons instead of buttons
- Site log badge shows the running activity
- Czech and German texts use en-dashes
- Web UI templates split into individual embedded files

### Security

- All services run with no_new_privs, so setuid binaries have no effect

### Fixed

- Logged-in IP addresses show the real visitor instead of 127.0.0.1
- Apache redirects no longer leak the internal port 8082 address
- Site language packs are installed by the Moodle installer

## [0.2.0] - 2026-08-29

### Added

- Quick Tunnel button that publishes the site on a trycloudflare.com URL with a QR code
- Single-use login links and QR codes for demo accounts
- Create user dialog with generated passwords and optional Manager or admin role
- Czech and German console translations, also installed as Moodle language packs
- Dark mode toggle in the header
- Built-in mail catcher (Mailpit) with a Mail card in the console
- Console restyled on vendored Pico CSS 2
- Setting for custom ports
- Setting for demo name
- Launcher scripts for macOS and Windows
- Support for development and testing in mpd

### Changed

- Redesigned developer docs and helper scripts
- Switched to Go 1.27.0
- Simplified directory structure
- Improved docs

### Fixed

- User credentials are masked

## [0.1.2] - 2026-08-19

### Changed

- Assemble the Moodle tree with `mudev clone --shallow` for faster installs
- Management UI visual improvements

### Fixed

- Removed default management password from README
- Fixed init system race conditions
- More reliable site reset command
- mudev binary is built with the correct version
- Redesigned log display area
- Fixed display issues when the container is rebuilt with the browser open

## [0.1.1] - 2026-08-18

### Fixed

- Fixed mudev compile error

## [0.1.0] - 2026-08-18

### Added

- First preview release
