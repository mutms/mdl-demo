# mdl-demo

Try Moodle™ or MuTMS on your own computer — one command, no web server,
database or PHP to install.

mdl-demo is a container image with everything a Moodle site needs inside.
All it takes is the container support from your OS vendor (see the sections
below for Apple container, Microsoft WSL, or podman/docker on Linux).
Start it, open <http://localhost:8081> in your browser, pick a Moodle version
and set an admin password — a few minutes later your demo site is running at
<http://localhost:8080>. The site is only reachable from your own computer.

When you are done, delete the container and everything is gone — nothing was
installed on your computer.

Each container holds one demo site. To try a different Moodle version, stop
the container and create a new one (two containers cannot use the same ports).

## macOS

Requires [Apple container](https://github.com/apple/container) (macOS 15+ on
Apple silicon):

```sh
container run -d --name demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 -e MDL_DEMO_PASSWORD=choose-a-password ghcr.io/mutms/mdl-demo
```

## Windows 11

Runs on the new [WSL containers](https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/)
preview (`wsl --update --pre-release`):

```powershell
wslc run -d --name demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 -e MDL_DEMO_PASSWORD=choose-a-password ghcr.io/mutms/mdl-demo
```

## Linux

Rootful podman or docker:

```sh
sudo podman run -d --name demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 -e MDL_DEMO_PASSWORD=choose-a-password ghcr.io/mutms/mdl-demo
```

## When something misbehaves

The management UI has a diagnostics page (`/debug`) with service states and
recent logs as one copy-pasteable block — paste it into a
[bug report](https://github.com/mutms/mdl-demo/issues) and we will take a look.

TODO: image publishing to a registry is not set up yet; see DEV.md for building
it yourself in the meantime.

## AI disclosure

Majority of this project was written with the help of Claude (Anthropic). Everything it
produced was reviewed, corrected where needed and accepted by a human maintainer before
being committed; the design decisions and the final state of the code are the maintainers'.

## License

Copyright (C) 2026 Petr Skoda. [GPL-3.0](LICENSE) or later.

Moodle is a registered trademark of [Moodle Pty Ltd](https://moodle.com).
