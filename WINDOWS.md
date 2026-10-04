# mdl-demo on Windows 11

Run throwaway Moodle/MuTMS demo sites on Windows with the
[WSL](https://learn.microsoft.com/en-gb/windows/wsl/) containers (`wslc`).

## Requirements

- Windows 11, 64-bit.
- WSL with containers. Open Terminal as administrator and run:

  ```powershell
  wsl --install --no-distribution
  ```

  then restart the computer. If you already have WSL, run `wsl --update`
  instead.

## The MDL Demo app (recommended)

[MDL Demo](https://github.com/mutms/mdl-demo-win) is a small Windows app that
creates, starts, stops and deletes demos for you, with no commands to type.
Download it from its
[latest release](https://github.com/mutms/mdl-demo-win/releases/latest). It is
not code-signed yet, so the first time you run it, Windows shows "Windows
protected your PC": click **More info**, then **Run anyway**.

## Command line

The same demo in one PowerShell command:

```powershell
wslc run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Then open <http://127.0.0.1:8081>, pick a version and click install. Manage it
with `wslc ps -a`, `wslc stop mdl-demo-8081`, `wslc start mdl-demo-8081` and
`wslc rm mdl-demo-8081` (which also deletes the site and its data).

To run several demos, give each its own number - the console port; the site is
on the next one. Name the container after it and set `MDL_DEMO_PORT`
(`MDL_DEMO_NAME` is optional):

```powershell
wslc run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

New demos use the image you already have; get the newest with
`wslc pull ghcr.io/mutms/mdl-demo`.

[`mdl-demo.cmd`](launcher/mdl-demo.cmd) is a script that types these commands
for you (`mdl-demo.cmd help`). Demos made with the script, the app or by hand
all show up in each other.

## Removing everything

Delete your demos, remove the image (`wslc image remove ghcr.io/mutms/mdl-demo`)
and - if you use it for nothing else - remove the WSL (`wsl --uninstall`).
