# mdl-demo

mdl-demo is an OCI container fully configured for installation of MuTMS and Moodle
for demo purposes. It contains a simplified UI for setting up demo sites.

mdl-demo is optimised for use in Apple containers and Microsoft WSL containers,
it does not require any 3rd party software. It can also be run via rootful podman
or docker on Linux.

The container exposes two ports, both meant to be mapped to localhost only:

- **8080** — the Moodle demo site (Apache)
- **8081** — the management web UI, where you pick a site recipe, set the Moodle
  admin password and install the demo site

One demo site per container: to try a different Moodle version, create a new
container and delete the old one.

## Installation in macOS

Requires [Apple container](https://github.com/apple/container) (macOS 15+ on Apple
silicon). Then:

```sh
container run -d --name demo --cap-add SYS_ADMIN \
    -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 \
    -e MDL_DEMO_PASSWORD=choose-a-password \
    ghcr.io/mutms/mdl-demo
```

Open <http://localhost:8081>, log in with the password you chose, pick a recipe and
install. The demo site then lives at <http://localhost:8080>.

`--cap-add SYS_ADMIN` is required: the container boots a real systemd, which
needs it to mount its API filesystems (/run, cgroups) at startup.
`-e MDL_DEMO_PASSWORD=…` is optional — without it the UI asks you to set a password
on first access.

TODO: image publishing to a registry is not set up yet; see DEV.md for building it
yourself in the meantime.

## Installation of Windows 11

Requires WSL with the WSL containers preview (`wsl --update --pre-release`).
The preview cannot boot systemd yet, so the container starts mdl-demo's
built-in service supervisor instead (same image, different entrypoint — note
the `--tmpfs /run`, which the supervisor needs):

```powershell
wslc run -d --name demo --tmpfs /run --entrypoint /usr/bin/mdl-demo -p 127.0.0.1:8080:8080 -p 127.0.0.1:8081:8081 -e MDL_DEMO_PASSWORD=choose-a-password ghcr.io/mutms/mdl-demo init
```

Open <http://localhost:8081> and continue as on macOS. Once WSL containers
can boot systemd, the plain macOS-style command will work here too.

## AI disclosure

Majority of this project was written with the help of Claude (Anthropic). Everything it
produced was reviewed, corrected where needed and accepted by a human maintainer before
being committed; the design decisions and the final state of the code are the maintainers'.

## License

Copyright (C) 2026 Petr Skoda. [GPL-3.0](LICENSE) or later.

Moodle is a registered trademark of [Moodle Pty Ltd](https://moodle.com).
