# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- All supervised services (postgresql, php-fpm, apache2, mailpit) and the
  tunnel now run with no_new_privs: nothing in their process trees can ever
  gain privileges, so setuid binaries are inert to a compromised web process

### Fixed

- Sign-in notifications and logs now show the visitor's real IP address
  instead of 127.0.0.1 (Moodle reads the proxy's X-Forwarded-For header)

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
