# mdl-demo

Try Moodle™ or MuTMS on your own computer. One command is all it takes.
You do not need to install a web server, a database or PHP.

mdl-demo is a container image. Think of a container as a small virtual
computer inside your own. It has its own files and programs and cannot
touch the rest of your computer. This one has everything a Moodle site
needs.

Here is how it works:

1. Start the container with the command for your system (see below).
2. Open <http://localhost:8081> in your browser. This is the console.
3. Pick a Moodle version and click install.
4. A few minutes later your site is ready at <http://localhost:8082>.

Only your computer can see the site. When you want to show it to others,
one click in the console shares it on a temporary public link. When you
are done, delete the container and everything is gone.

## macOS

You need [Apple container](https://github.com/apple/container). It works
on macOS 15 or newer with Apple silicon.

Open the Terminal app and paste this command. It starts a demo and opens
the console in your browser:

```sh
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo && open http://localhost:8081
```

If you do not want the browser to open, leave out the `&& open …` part.

There is also a small helper script called `mdl-demo`. It makes creating,
stopping, starting and deleting demos easier:

```sh
curl -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo && chmod +x mdl-demo
./mdl-demo create --open
```

The `--open` option waits until the console is ready and then opens it.
It works with `create` and `start`.

## Windows 11

You need the new [WSL containers](https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/)
preview. To get it, run `wsl --update --pre-release`.

Open PowerShell and paste this command to start a demo:

```powershell
wslc run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Then open <http://localhost:8081> in your browser.

There is also a small helper script called `mdl-demo.cmd`. Download it into
a folder, open PowerShell in that folder and run it:

```powershell
curl.exe -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo.cmd
.\mdl-demo.cmd create
```

Note: type `curl.exe`, not `curl`. In PowerShell, `curl` is a different
command.

## Sharing your demo with others

Out of the box, only your own computer can reach the console and the
site. The commands above use `127.0.0.1`, which means exactly that.

To show the site to other people, click the Quick Tunnel button in the
console. It gives your site a temporary public address and a QR code.
Anyone with the link can open the site until you stop the tunnel. The
console itself stays private on your computer.

While the tunnel runs, the site is public. The demo accounts and anything
you put into the site can be seen by anyone who has the link. Do not put
anything in it that strangers should not see.

Do not remove the `127.0.0.1:` part from the commands unless you know what
you are doing. Anyone who can reach the console can do everything: install,
reset, read the site's mail and log in as the administrator.

## Getting the newest version

mdl-demo is still in development and changes often. Your container tool
keeps the copy it already downloaded and does not check for updates.

Run this before creating a new demo to get the newest version:

```sh
container image pull ghcr.io/mutms/mdl-demo    # macOS
wslc pull ghcr.io/mutms/mdl-demo               # Windows
```

Only new demos use the new version. To update an existing demo, delete it
and create it again.

## More than one demo

Every demo has a number. It is the port of the console. The site uses the
next number. In the examples above the console is 8081 and the site is 8082.

You can run several demos at the same time. Give each one its own number.
This also helps when 8081 is already in use:

```sh
./mdl-demo create 7777 --name="Moodle 5.2 workshop"
./mdl-demo create 7800 --name="MuTMS preview" --tag=v0.1.2
./mdl-demo list
./mdl-demo stop 7777
./mdl-demo start 7777
./mdl-demo delete 7800
```

On Windows use `.\mdl-demo.cmd` instead of `./mdl-demo`.

- `--name` sets the site name and the console heading.
- `--tag` picks a released image version.
- `--open` opens the console when it is ready.

You can do the same without the helper script. Name the container after
the number, set `MDL_DEMO_PORT` to the number and map the number and the
next one to the container's ports 8081 and 8082. `MDL_DEMO_NAME` is
optional and works like `--name`:

```sh
container run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

On Windows use `wslc run …` with the same arguments.

## Stopping, starting and deleting

You can manage demos with the normal container commands. The helper
script's `list`, `stop`, `start` and `delete` do the same thing with the
number filled in:

|        | macOS                           | Windows                    |
|--------|---------------------------------|----------------------------|
| list   | `container ls -a`               | `wslc ps -a`               |
| stop   | `container stop mdl-demo-8081`  | `wslc stop mdl-demo-8081`  |
| start  | `container start mdl-demo-8081` | `wslc start mdl-demo-8081` |
| delete | `container rm mdl-demo-8081`    | `wslc rm mdl-demo-8081`    |

Stop keeps your site. Delete removes the site and all its data.

## Something is not working?

Open the `/debug` page of the console, for example
<http://localhost:8081/debug>. It shows the state of all services and the
recent logs in one block. Copy it into a
[bug report](https://github.com/mutms/mdl-demo/issues) and we will have a
look.

## Contributing

Forks and contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).
AI assistants can start with [AGENTS.md](AGENTS.md).

The site code is put together by [mudev](https://github.com/mutms/mudev)
from [site recipes](https://github.com/mutms/mdl-recipes) and a
[plugin catalogue](https://github.com/mutms/mdl-plugins). New recipes and
plugins are welcome too.

## AI disclosure

Most of this project was written with the help of Claude (Anthropic). A
human maintainer reviewed, corrected and accepted everything before it was
committed. The design decisions and the final code are the maintainers'
own.

## License

Copyright (C) 2026 Petr Skoda. [GPL-3.0](LICENSE) or later.

Moodle is a registered trademark of [Moodle Pty Ltd](https://moodle.com).
