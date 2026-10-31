# Working on this demo site

This is a throwaway Moodle/MuTMS demo in a container, and you are in its CLI
console as `www-data`. The site, its database and this container are disposable,
so edit freely. Nobody's real data is here.

## Layout

- `/srv/projects/demo`: the Moodle code. Moodle 5.1+ keeps the web root in `public/`;
  `config.php` and `admin/cli/` are at the tree root on those versions, so try the
  root first and then `public/` for any script.
  - **`$MDL_DEMO_EDITABLE_CODE` is set:** this demo was created with editable code.
    Edit Moodle and its plugins freely, in place. Things may break, and resetting the
    site gives a clean copy. Purge caches after editing. Your edits aren't in backups.
  - **It isn't set:** the code is read-only and root-owned on purpose, so never try to
    change its permissions. If an edit in place is needed, tell the user it means
    creating the demo again with `-e MDL_DEMO_EDITABLE_CODE=1`; it can't be changed
    from inside.
- `/srv/projects/extrarepos/`: the place here for your own work. Put plugin
  checkouts and generator code in a directory of their own, under git.
- `/srv/projects/extrafiles/`: files the user shares with you. They upload and download
  here on the console's Settings page, so look here for what they mention uploading, and
  leave finished work here for them. Package only what they need: a zip of a plugin's own
  files, never `~/.ssh`, tokens or logins. The console shows the whole tree, so they can
  download single files from subfolders, but only one at a time: zip anything bigger.
- `~` (`/home/www-data`): scratch files, screenshots.
- `/srv/data/dataroot`: Moodle's dataroot. You share its owner with the web
  server, so scripts you run can write into it safely.

The commands you need are under Tools below. Each has `--help`.

## Rules

- Never reset or change a password to get in, least of all `admin`'s: the
  demo's presenter uses it. `mdl-login [user]` prints a single-use login URL.
- You have no root and no sudo. Everything that needs root goes through the
  console's own routes, which the helpers below use for you.
- Reset and restore in the console end this tmux session. Your files stay, and
  so does anything in `extrarepos/`, but the site and its data do not.
- Backups contain the site only, not `extrarepos/`, `extrafiles/` or `~`. A plugin added from
  `extrarepos/` will not restore on another container without that checkout.

## Tools

| Task | Command |
|---|---|
| Run PHP with Moodle loaded | `mdl-php '<code>'`, a `.php` file, or `-` for stdin; `--user=<username>` |
| Query the database | `mdl-sql 'SELECT id, username FROM {user}'`; `--write` to change data |
| Log a browser in | `mdl-login [user] [--path=/local/path]`; `--internal` for a browser in here |
| See a page as a user | `mdl-screenshot [--user=<name>] /local/path` prints a PNG's path; read the image |
| Scripted browser checks | `/usr/share/mdl-demo/cli/browser/cdp.mjs` (dependency-free CDP client; `shot.mjs` shows its use) |
| Add or update a plugin | `git commit` in `extrarepos/x`, then `mdl-repo-add x [ref]` |

`mdl-screenshot`, the browser tools and these need `claude-install` to have run:
`python3`, `rg`, `jq`, `less`, `vim`, `nano`, `tree`, `file`, `patch`, `zip`, `unzip`, `rsync`,
`strace`, `lsof`, `htop`, `fuser`, `ss`, `ip`, `dig`, `nc`. You have no root, so you
can't install anything else.

## Diagnosing a bug

- Reproduce it first, with data in a script you can rerun.
- PHP errors are always logged to `/srv/data/dataroot/php-errors.log` and never shown
  in a page. `config.php` sets this, so the admin debugging settings can't change it.
  The log goes into backups with the site, and a restore replaces it with the one
  from the backup.
- For developer-level messages from your own script, set
  `$CFG->debug = DEBUG_DEVELOPER;` at its top. They go to the same log.
- `strace` works on a command you start, such as `strace -f php script.php`, but
  can't attach to the site's php-fpm workers, so reproduce through `mdl-php`.
- To test a fix in a plugin, commit it in `extrarepos/` and deploy it with
  `mdl-repo-add`, or edit it in place if the code is editable (see Layout).
- If the problem may be in mdl-demo itself (the console, the helpers, this container),
  read its source. It's public, and its own `AGENTS.md` explains the architecture:
  `git clone https://github.com/mutms/mdl-demo extrarepos/mdl-demo`
  To match this demo, check out its version, the second word of `mdl-demo version`,
  if that's a release tag. You can't deploy changes to it from here, so report what you
  find at https://github.com/mutms/mdl-demo/issues.

## Adding a private plugin

1. Without a key yet, run `ssh-keygen -t ed25519`, then show the user
   `~/.ssh/id_ed25519.pub` and ask them to add it to the repository as a deploy key.
   Over HTTPS, ask for their username and an access token instead.
2. `git clone git@github.com:you/moodle-local_example.git extrarepos/example`
3. `mdl-repo-add example` installs it into the site. To deploy later changes, commit
   them in `extrarepos/example` and run it again.

## Saving and resuming work

Deleting the demo deletes `extrarepos/`, so before stopping, offer to push the work or
leave a zip in `extrafiles/`. To resume from a zip the user uploaded, unzip it into
`extrarepos/`, run `git init` and commit if it has no `.git`, then use `mdl-repo-add`.

## Reaching the site from in here

Use `http://127.0.0.1:8082` for anything running in this container: `curl`, a browser,
`mdl-login --internal`. The site's public URL (`mdl-php 'echo $CFG->wwwroot'`) carries
the port published outside, and doesn't work from in here. Give people that public
URL, and don't save the internal one in site content. For a browser you start yourself,
pass Chromium `--no-sandbox --disable-dev-shm-usage`.

Mail the site sends never leaves the container; the console's Redirected emails
page shows it.
