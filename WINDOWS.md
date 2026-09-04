# mdl-demo on Windows 11

Run throwaway Moodle/MuTMS demo sites on Windows with the
[WSL containers](https://devblogs.microsoft.com/commandline/wsl-container-is-now-available-for-public-preview/)
preview (`wslc`).

## Requirements

- Windows 11 with the WSL containers preview. Install it with:

  ```powershell
  wsl --update --pre-release
  ```

  then open a new terminal window.

## One command

In PowerShell:

```powershell
wslc run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Then open <http://127.0.0.1:8081>, pick a version and click install.

## The mdl-demo.cmd helper (recommended)

A small script that fills in the container name and ports for you. Download it
into a folder and open PowerShell in that folder:

```powershell
curl.exe -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo.cmd
.\mdl-demo.cmd create
```

Type `curl.exe`, not `curl` - in PowerShell `curl` is a different command.

Then:

```powershell
.\mdl-demo.cmd create        # create a demo
.\mdl-demo.cmd list          # show your demos
.\mdl-demo.cmd stop 8081     # stop one (its site and data are kept)
.\mdl-demo.cmd start 8081    # start it again
.\mdl-demo.cmd delete 8081   # remove it, including its site and data
```

`mdl-demo.cmd help` lists every command. `mdl-demo.cmd install` and
`mdl-demo.cmd uninstall` print these setup and removal steps at any time. Put
the folder on your PATH to run `mdl-demo` from anywhere.

## More than one demo

Every demo has a number - the port of its console; the site is on the next
number. Give each demo its own number to run several at once (also handy when
8081 is taken):

```powershell
.\mdl-demo.cmd create 7777 --name="Moodle 5.2 workshop"
.\mdl-demo.cmd create 7800 --tag=v0.1.2
.\mdl-demo.cmd list
.\mdl-demo.cmd delete 7800
```

- `--name` sets the site name and the console heading.
- `--tag` picks a released image version (default: latest).
- `--open` opens the console once it is ready (works with create and start).

## Managing demos by hand

The helper is optional - the plain commands work too:

| action | command                     |
| ------ | --------------------------- |
| list   | `wslc ps -a`                |
| stop   | `wslc stop mdl-demo-8081`   |
| start  | `wslc start mdl-demo-8081`  |
| delete | `wslc rm mdl-demo-8081`     |

To create a demo on a custom port without the helper, name the container after
the number, set `MDL_DEMO_PORT`, and map the number and the next one onto the
container's ports 8081 and 8082 (`MDL_DEMO_NAME` is optional):

```powershell
wslc run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

## Getting the newest version

mdl-demo changes often, and your container tool keeps the copy it already has.
Pull the latest before creating a new demo:

```powershell
wslc pull ghcr.io/mutms/mdl-demo
```

Only new demos use it - to update an existing demo, delete it and create it
again.

## Reclaiming disk space

Old image layers pile up after updates. Remove the unused ones (containers,
sites and data are never touched):

```powershell
.\mdl-demo.cmd gc          # dangling layers left after a pull
.\mdl-demo.cmd gc --all    # every unused image, incl. the demo image - when disk is tight
```

## Removing everything

Delete your demos (`mdl-demo.cmd delete NNNN` for each in `mdl-demo.cmd list`),
remove the demo image (`wslc image remove ghcr.io/mutms/mdl-demo`), delete
`mdl-demo.cmd`, and - if you use it for nothing else - remove the WSL containers
preview. `mdl-demo.cmd uninstall` prints these steps too.
