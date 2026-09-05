# mdl-demo

Try Moodle™ or MuTMS on your own computer. One command, no web server,
database or PHP to set up, and no online registration.

mdl-demo is a container image - think of a small computer inside your own,
with its own files and programs, that cannot touch the rest of your machine.
This one has everything a Moodle site needs, plus a web console that sets up
and manages the demo site for you - effortlessly.

## How it works

1. Install container support in your OS - Apple's `container` on
   macOS, Microsoft's `wslc` on Windows.
2. Run one command for your system.
3. Open the console at <http://127.0.0.1:8081>.
4. Pick a Moodle version and click install.
5. A few minutes later your site is ready at <http://127.0.0.1:8082>.

Only your computer can see it. To show it to others, one click in the console
gives it a temporary public link, and another click makes it private again.
Reset the site in the console to start over with a different version, or delete
the container when you are done testing.

<!-- A short demo video / animated walkthrough will live here. -->

## Getting started

### macOS

You need [Apple container](https://github.com/apple/container) (macOS 26 or
newer, Apple silicon). In Terminal:

```sh
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo && open http://127.0.0.1:8081
```

For named commands like `create`, `start`, `stop` and `delete`, use the
`mdl-demo` helper described in the **[full macOS guide](MACOS.md)**.

### Windows 11

You need the [WSL containers](https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/)
preview (`wsl --update --pre-release`). In PowerShell:

```powershell
wslc run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Then open <http://127.0.0.1:8081>. To manage demos more easily, use the
`mdl-demo.cmd` helper described in the **[full Windows guide](WINDOWS.md)**.

## Sharing your demo

By default only your own computer can reach the site and the console. To let
others in, turn on the **Quick Tunnel** switch in the console: it gives the
site a temporary public link (with a QR code for phones) that anyone can open
until you switch it off. While it is on the site is public, so do not put
anything private in it. The console always stays private to your computer.

## Community

Questions, ideas, or just to show what you demoed: join
**[#mdl-demo:matrix.org](https://matrix.to/#/#mdl-demo:matrix.org)** on Matrix.
Bugs and feature requests go to
[GitHub issues](https://github.com/mutms/mdl-demo/issues).

## Contributing

Forks and contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md);
AI assistants can start with [AGENTS.md](AGENTS.md).

## AI disclosure

Most of this project was written with the help of Claude (Anthropic). A human
maintainer reviewed, corrected and accepted everything before it was committed.
The design decisions and the final code are the maintainers' own.

## License

Copyright (C) 2026 Petr Skoda. [GPL-3.0](LICENSE) or later.

Moodle™ is a registered trademark of Moodle Pty Ltd. MuTMS is an independent
project not affiliated with or endorsed by Moodle Pty Ltd.
