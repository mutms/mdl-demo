# Working on this demo site

This is a throwaway Moodle/MuTMS demo in a container, and you are in its CLI
console as `www-data`. The site, its database and this container are disposable,
so edit freely. Nobody's real data is here.

## Layout

- `/srv/projects/demo`: the Moodle code. Read-only for you, root-owned on
  purpose: never try to change its permissions. Moodle 5.1+ keeps the web root
  in `public/`; `config.php` and `admin/cli/` are at the tree root on those
  versions, so try the root first and then `public/` for any script.
- `/srv/projects/extrarepos/`: the only place here you can write. Put plugin
  checkouts and generator code in a directory of their own, under git.
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
- Backups contain the site only, not `extrarepos/` or `~`. A plugin added from
  `extrarepos/` will not restore on another container without that checkout.

## Tools

| Task | Command |
|---|---|
| Run PHP with Moodle loaded | `mdl-php '<code>'`, a `.php` file, or `-` for stdin; `--user=<username>` |
| Query the database | `mdl-sql 'SELECT id, username FROM {user}'`; `--write` to change data |
| Log a browser in | `mdl-login [user] [--path=/local/path]` |
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
- To test a fix, commit it in `extrarepos/` and deploy it with `mdl-repo-add`, since
  the `demo` tree is read-only.

## Reaching the site from in here

The site's URL (`mdl-php 'echo $CFG->wwwroot'`) carries the port published
outside the container, but inside it only port 8082 listens. `mdl-screenshot`
handles this. For `curl`, use `--connect-to ::127.0.0.1:8082` when the URL's
port is not 8082. For a browser you start yourself, pass Chromium
`--no-sandbox` and `--host-rules="MAP 127.0.0.1:<port> 127.0.0.1:8082"`.

Mail the site sends never leaves the container; the console's Redirected emails
page shows it.
