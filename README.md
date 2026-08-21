# mdl-demo

Try Moodle™ or MuTMS on your own computer — one command, no web server,
database or PHP to install.

mdl-demo is a container image with everything a Moodle site needs inside.
All it takes is the container support from your OS vendor (see the sections
below for Apple container, Microsoft WSL, or podman on Linux).
Start it, open <http://localhost:8081> in your browser and pick a Moodle
version — a few minutes later your demo site is running at
<http://localhost:8082>. The site is only reachable from your own computer.

When you are done, delete the container and everything is gone — nothing was
installed on your computer.

## macOS

Requires [Apple container](https://github.com/apple/container) (macOS 15+ on
Apple silicon). One command starts a demo:

```sh
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Optionally, the small `mdl-demo` launcher wraps that command and the ones for
stopping, starting and deleting demos (see below):

```sh
curl -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo && chmod +x mdl-demo
./mdl-demo create
```

## Windows 11

Runs on the new [WSL containers](https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/)
preview (`wsl --update --pre-release`). One command starts a demo:

```powershell
wslc run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Optionally, save
[mdl-demo.cmd](https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo.cmd)
into a folder and open a terminal there — it wraps that command and the ones
for stopping, starting and deleting demos (see below):

```powershell
.\mdl-demo.cmd create
```

## Linux

Rootful podman (any compatible container runtime works the same way); there
is no launcher for Linux:

```sh
sudo podman run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

## Several demos, or other ports

Every demo has a number: the port of its management console. The commands
above use 8081 — console at <http://localhost:8081>, site at
<http://localhost:8082>. When those ports are taken, or when you want more
than one demo at a time, give each demo its own number:

```sh
./mdl-demo create 7777 --name="Moodle 5.2 workshop"
./mdl-demo create 7800 --name="MuTMS preview" --password=secret --tag=v0.1.2
./mdl-demo list
./mdl-demo stop 7777
./mdl-demo start 7777
./mdl-demo delete 7800
```

(On Windows the same commands start with `.\mdl-demo.cmd`.) The first demo's
console is then at <http://localhost:7777> and its site at
<http://localhost:7778> — the site is always on the next port.

- `--name` is shown in the console heading (handy with several open) and
  becomes the Moodle site name.
- `--password` sets the console password up front; without it the console
  asks you to choose one on the first visit.
- `--tag` picks a released image version instead of the latest.

Without the launcher it is still one command — name the container after the
number, tell the demo its number with `MDL_DEMO_PORT`, and map the number and
the next one onto the container's fixed ports 8081 and 8082 (`MDL_DEMO_NAME`
and `MDL_DEMO_PASSWORD` are the optional equivalents of `--name` and
`--password`):

```sh
container run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

(`wslc run …` on Windows, `sudo podman run …` on Linux, same arguments.)

## Managing demo containers

The usual container commands work everywhere (`mdl-demo-8081` is the name the
run commands above chose); the launcher's `list`, `stop`, `start` and `delete`
are the same commands with the number filled in:

|        | macOS                           | Windows                    | Linux                             |
|--------|---------------------------------|----------------------------|-----------------------------------|
| list   | `container ls -a`               | `wslc ps -a`               | `sudo podman ps -a`               |
| stop   | `container stop mdl-demo-8081`  | `wslc stop mdl-demo-8081`  | `sudo podman stop mdl-demo-8081`  |
| start  | `container start mdl-demo-8081` | `wslc start mdl-demo-8081` | `sudo podman start mdl-demo-8081` |
| delete | `container rm mdl-demo-8081`    | `wslc rm mdl-demo-8081`    | `sudo podman rm mdl-demo-8081`    |

Stopping keeps the demo site — start the container again and it is back.
Deleting removes the site and all its data for good.

## When something misbehaves

The management console has a diagnostics page (`/debug`) with service states
and recent logs as one copy-pasteable block — paste it into a
[bug report](https://github.com/mutms/mdl-demo/issues) and we will take a look.

## Contributing

Forks and contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
AI assistants get a head start from [AGENTS.md](AGENTS.md).

The demo site code is assembled by [mudev](https://github.com/mutms/mudev),
the Moodle/MuTMS development tool, from community-editable
[site recipes](https://github.com/mutms/mdl-recipes) and a
[plugin catalogue](https://github.com/mutms/mdl-plugins) — new recipes and
plugins are contributions too.

## AI disclosure

Majority of this project was written with the help of Claude (Anthropic). Everything it
produced was reviewed, corrected where needed and accepted by a human maintainer before
being committed; the design decisions and the final state of the code are the maintainers'.

## License

Copyright (C) 2026 Petr Skoda. [GPL-3.0](LICENSE) or later.

Moodle is a registered trademark of [Moodle Pty Ltd](https://moodle.com).
