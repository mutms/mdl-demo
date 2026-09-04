# mdl-demo - image build and development

Building and hacking on the mdl-demo OCI image. The architecture, invariants,
testing workflow and extension points live in [AGENTS.md](AGENTS.md);
contributor basics in [CONTRIBUTING.md](CONTRIBUTING.md).

## Dev environment

An mpd VM ([mpd-virt](https://github.com/mutms/mpd-virt)) is the easy path, but
any Linux box with rootful podman, Go (go.mod picks the compiler) and make
works too.

```shell
ssh mpd-NNN
mkdir /srv/projects/mdl-demo && cd /srv/projects/mdl-demo
git clone https://github.com/mutms/mdl-demo.git .
mpd init && mpd start
```

Working with an AI agent? mpd ships helpers:

```shell
claude-install    # Claude Code
gnome-install     # Chromium + media tools (screenshots, recordings)
claude
```

## Build loop

```sh
make build test vet fmt-check   # native binary + checks
make image                      # build the OCI image
make run                        # start a test container on the VM
```

Then iterate without rebuilding the image - swap a fresh binary into the
running container (Go/templates/CSS/JS, seconds):

```sh
make hotpatch
```

`make run` prints the console URL. See AGENTS.md "Working on it" for the `PORT`
override (your own container), `hotpatch`, and the end-to-end install/verify
flow.

## Releasing

Run [`dev/github-publish.sh`](dev/github-publish.sh) on an Apple silicon Mac - the
release steps are in its header comment. It only publishes a clean, tagged
build, so nothing unreleased can reach the registry.
