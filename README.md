# mdl-demo

Try Moodle™ or MuTMS on your own computer — one command, no web server,
database or PHP to install.

mdl-demo is a container image with everything a Moodle site needs. Start
it with your OS's own container tool (below), open <http://localhost:8081>,
pick a Moodle version, and a few minutes later the site is at
<http://localhost:8082> — reachable only from your computer. Delete the
container and everything is gone.

## macOS

Requires [Apple container](https://github.com/apple/container) (macOS 15+ on
Apple silicon). One command starts a demo:

```sh
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Optionally, the `mdl-demo` launcher wraps that and the stop/start/delete
commands:

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

Optionally, the `mdl-demo.cmd` launcher wraps that and the stop/start/delete
commands — download it into a folder and run it from a terminal there:

```powershell
curl.exe -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo.cmd
.\mdl-demo.cmd create
```

(`curl.exe`, not `curl` — in PowerShell the bare name is an unrelated alias.)

## Linux

Rootful podman (or any compatible runtime); no launcher on Linux:

```sh
sudo podman run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

## Updating

While mdl-demo is in development the image changes often, and your container
tool silently reuses the copy it already downloaded. Pull the newest image
before creating a demo:

```sh
container image pull ghcr.io/mutms/mdl-demo    # macOS
wslc pull ghcr.io/mutms/mdl-demo               # Windows
sudo podman pull ghcr.io/mutms/mdl-demo        # Linux
```

The update reaches newly created demos only; delete and recreate a demo to
move it to the new version.

## Several demos, or other ports

Every demo has a number: the port of its console, with the site on the
next port (8081 and 8082 above). For several demos, or when those ports are
taken, give each its own number:

```sh
./mdl-demo create 7777 --name="Moodle 5.2 workshop"
./mdl-demo create 7800 --name="MuTMS preview" --password=secret --tag=v0.1.2
./mdl-demo list
./mdl-demo stop 7777
./mdl-demo start 7777
./mdl-demo delete 7800
```

(On Windows: `.\mdl-demo.cmd`.) `--name` becomes the site name and the
console heading; `--password` sets the console password up front (otherwise
the console asks on the first visit); `--tag` picks a released image version.

Without the launcher: name the container after the number, pass it as
`MDL_DEMO_PORT`, and map the number and the next one onto the container's
fixed ports 8081 and 8082 (`MDL_DEMO_NAME`, `MDL_DEMO_PASSWORD` are the
optional equivalents of `--name`, `--password`):

```sh
container run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

(`wslc run …` on Windows, `sudo podman run …` on Linux, same arguments.)

## Managing demo containers

The usual container commands work everywhere; the launcher's `list`, `stop`,
`start` and `delete` are the same with the number filled in:

|        | macOS                           | Windows                    | Linux                             |
|--------|---------------------------------|----------------------------|-----------------------------------|
| list   | `container ls -a`               | `wslc ps -a`               | `sudo podman ps -a`               |
| stop   | `container stop mdl-demo-8081`  | `wslc stop mdl-demo-8081`  | `sudo podman stop mdl-demo-8081`  |
| start  | `container start mdl-demo-8081` | `wslc start mdl-demo-8081` | `sudo podman start mdl-demo-8081` |
| delete | `container rm mdl-demo-8081`    | `wslc rm mdl-demo-8081`    | `sudo podman rm mdl-demo-8081`    |

Stop keeps the site; delete removes it and all its data.

## When something misbehaves

The console's `/debug` page shows service states and recent logs as one
block — paste it into a [bug report](https://github.com/mutms/mdl-demo/issues).

## Contributing

Forks and contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
AI assistants get a head start from [AGENTS.md](AGENTS.md).

The site code is assembled by [mudev](https://github.com/mutms/mudev) from
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
