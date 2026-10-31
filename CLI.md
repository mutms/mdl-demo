# The CLI console

The **CLI console** tile opens a terminal inside the demo, in its own window. It's the
easy way to fill a demo with sample data using
[Claude Code](https://code.claude.com/docs), or to reproduce and diagnose a bug.

You are the `www-data` user, the same one the web server runs as, with no root. You
start in `/srv/projects`, where the Moodle code in `demo/` is read-only and `extrarepos/`
is yours to write in. The session keeps running after you close the window.

## Claude Code

Claude Code is Anthropic's product, under Anthropic's own terms, and mdl-demo doesn't
include it.

1. Run `claude-install`. It installs Claude Code with its official installer, along with
   Chromium, Node, Python and diagnostic tools.
2. Run `claude` and sign in with your own account.
3. Ask for what you need: sample data, adding a private plugin, tracking down a bug.

With a Claude subscription, run `claude remote-control` instead to drive it from
claude.ai/code or your phone.

An `AGENTS.md` in `/srv/projects` tells coding agents how to work on the site, including
some helper commands for them.

**Your Claude login stays in the demo's home directory, where any PHP code on the site
can read it**, including a plugin you are trying out. Run `claude auth logout` before you
try a plugin you don't trust, and before you delete the demo.

## Memory

Claude Code asks for at least 4 GB of memory. With less, it and Chromium can slow down,
hang or get killed, and `claude-install` warns you.

**macOS:** a demo gets 1 GB by default. The limit is set when a demo is created. To raise
it, download a backup from the console if you want to keep the site, delete the demo, and
create it again with `--memory 4g`:

```sh
container run -d --memory 4g --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo && open http://127.0.0.1:8081
```

**Windows:** memory is shared by all your WSL containers, and defaults to half of your
computer's. To change it, run `wslc settings` and set `memorySize` under `session`, as
Microsoft's
[WSL container settings](https://learn.microsoft.com/en-us/windows/wsl/wsl-container-settings)
page explains.

## Adding a private plugin

Claude Code can do this for you. By hand:

1. Make a key, and add `~/.ssh/id_ed25519.pub` to the repository as a deploy key:
   ```sh
   ssh-keygen -t ed25519
   ```
2. Clone it into `extrarepos/`:
   ```sh
   git clone git@github.com:you/moodle-local_example.git extrarepos/example
   ```
3. Install it into the site:
   ```sh
   mdl-repo-add example
   ```

To deploy later changes, commit them in `extrarepos/example` and run step 3 again. Over
HTTPS, clone with your username and an access token instead of steps 1 and 2.

If you'd rather use the [GitHub CLI](https://cli.github.com), `gh` isn't included but needs
no root, so you or Claude Code can put its release binary in `~/.local/bin`. Its login
stays in the home directory, where any PHP on the site can read it, so use a token limited
to the repositories you need.

## Editable source code

To let Claude Code edit Moodle and its plugins directly, for example to fix a bug in core,
add `-e MDL_DEMO_EDITABLE_CODE=1` when you create the demo. It can't be switched on later
without creating the demo again.

On macOS:

```sh
container run -d --memory 4g -e MDL_DEMO_EDITABLE_CODE=1 --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo && open http://127.0.0.1:8081
```

On Windows, in PowerShell:

```powershell
wslc run -d -e MDL_DEMO_EDITABLE_CODE=1 --name mdl-demo-8081 -p 127.0.0.1:8081:8081 -p 127.0.0.1:8082:8082 ghcr.io/mutms/mdl-demo
```

Things may break, and resetting the site gives you a clean copy. Backups don't include
your code changes, so a restore loses them unless they are pushed and in the recipe. Any
PHP on the site can change the code too.

## What is kept

Reset and restore end the terminal session but keep your files. Backups contain only the
site, so they don't include `extrarepos/`, a plugin added from it, or your login. Deleting
the demo deletes all of it.

## Using the terminal

- Select with the mouse, Ctrl+C copies, and Ctrl+V pastes. With nothing selected, Ctrl+C
  interrupts as usual.
- Links open on click, and the wheel scrolls. Inside Claude Code, scroll with PgUp and PgDn.
- The session is tmux, so `Ctrl+B c` opens another window.
- For a fresh session, press Ctrl+D until Reconnect appears, then click it.

## Turning it off

To create a demo without the terminal, add `-e MDL_DEMO_NO_CLI=1` when you create it.
