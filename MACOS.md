# mdl-demo on macOS

Run throwaway Moodle/MuTMS demo sites on macOS with
[Apple container](https://github.com/apple/container).

## Requirements

- macOS 26 or newer, Apple silicon.
- Apple container: download the installer from
  <https://github.com/apple/container/releases>, then run `container system start`.

## One command

In Terminal:

```sh
container run -d --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo && open http://127.0.0.1:8081
```

This starts a demo and opens the console. Leave off `&& open ...` to skip the
browser. Then pick a version in the console and click install.

## The mdl-demo helper (recommended)

A small script that fills in the container name and ports for you. Download it
once, and optionally put it on your PATH:

```sh
curl -fsSLO https://raw.githubusercontent.com/mutms/mdl-demo/main/launcher/mdl-demo && chmod +x mdl-demo
mkdir -p ~/bin && mv mdl-demo ~/bin/mdl-demo    # optional: run it from anywhere
```

Then:

```sh
mdl-demo create --open      # create a demo and open the console when ready
mdl-demo list               # show your demos
mdl-demo stop 8081          # stop one (its site and data are kept)
mdl-demo start 8081 --open  # start it again
mdl-demo delete 8081        # remove it, including its site and data
```

`mdl-demo help` lists every command. `mdl-demo install` and `mdl-demo uninstall`
print these setup and removal steps at any time.

## More than one demo

Every demo has a number - the port of its console; the site is on the next
number. Give each demo its own number to run several at once (also handy when
8081 is taken):

```sh
mdl-demo create 7777 --name="Moodle 5.2 workshop"
mdl-demo create 7800 --tag=v0.1.2
mdl-demo list
mdl-demo delete 7800
```

- `--name` sets the site name and the console heading.
- `--tag` picks a released image version (default: latest).
- `--open` opens the console once it is ready (works with create and start).

## Managing demos by hand

The helper is optional - the plain commands work too:

| action | command                          |
| ------ | -------------------------------- |
| list   | `container ls -a`                |
| stop   | `container stop mdl-demo-8081`   |
| start  | `container start mdl-demo-8081`  |
| delete | `container rm mdl-demo-8081`     |

To create a demo on a custom port without the helper, name the container after
the number, set `MDL_DEMO_PORT`, and map the number and the next one onto the
container's ports 8081 and 8082 (`MDL_DEMO_NAME` is optional):

```sh
container run -d --name mdl-demo-7777 -e MDL_DEMO_PORT=7777 -e MDL_DEMO_NAME="Moodle 5.2 workshop" -p 127.0.0.1:7777:8081 -p 127.0.0.1:7778:8082 ghcr.io/mutms/mdl-demo
```

## Getting the newest version

mdl-demo changes often, and your container tool keeps the copy it already has.
Pull the latest before creating a new demo:

```sh
container image pull ghcr.io/mutms/mdl-demo
```

Only new demos use it - to update an existing demo, delete it and create it
again.

## Reclaiming disk space

Old image layers pile up after updates. Remove the unused ones (containers,
sites and data are never touched):

```sh
mdl-demo gc          # dangling layers left after a pull
mdl-demo gc --all    # every unused image, incl. the demo image - when disk is tight
```

## Upgrading Apple container

Apple container bundles a Linux kernel, so upgrading is not just replacing a
binary. Follow Apple's own instructions:
<https://github.com/apple/container#upgrade-or-downgrade>.

## Removing everything

Delete your demos (`mdl-demo delete NNNN` for each in `mdl-demo list`), remove
the demo image (`container image rm ghcr.io/mutms/mdl-demo`), delete the
`mdl-demo` helper, and - if you use it for nothing else - uninstall Apple
container itself (<https://github.com/apple/container#uninstall>).
`mdl-demo uninstall` prints these steps too.
