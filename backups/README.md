# Pre-bundled site backups

Any `*.mdb` file in this directory is baked into the container image at
`/srv/backups/` and appears on the console's **Backups** page of every new
container — ready to restore with one click, no installation wait.

This is how a fork ships a prepared demo site in its own image:

1. Build your demo site once (install, add courses, users, content).
2. On the Backups page, **Back up data**, then **Download** the `.mdb` file.
3. Drop the file here and build the image.

A `.mdb` is self-contained — it carries the database, the uploaded files and
the exact code recipe — so it restores with no dependency on the recipe
catalogue. It contains no passwords: every restore generates fresh ones.

The upstream repository ignores `*.mdb` here (they are large binaries);
remove that line from `.gitignore` in your fork if you want to commit yours,
or copy the file in as a build step.
