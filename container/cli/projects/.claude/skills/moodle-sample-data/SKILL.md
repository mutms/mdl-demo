---
name: moodle-sample-data
description: Create realistic sample data in this Moodle demo site, such as users, courses, enrolments, activities, grades and plugin-specific records. Use when asked to populate, seed or provision the site.
---

# Creating sample data

Read `/srv/projects/AGENTS.md` first for the layout and rules.

1. **Look before writing.** Check the Moodle version with
   `mdl-php 'echo $CFG->release, "\n";'`, list the installed plugins, and see
   what already exists with `mdl-sql`. Ask what the demo should show if the
   request doesn't say: audience, language, and rough volume.
2. **Write a script, not one-off commands.** Keep it in a git repo under
   `/srv/projects/extrarepos/`, so it can be rerun after a reset. Make it
   idempotent: look records up by a stable idnumber or shortname before creating.
3. **Use Moodle's APIs and generators, not direct SQL.** They fire events and
   fill in related tables.
   - Run scripts with `mdl-php path/to/script.php`; Moodle is already loaded.
   - Core generators: see how `admin/tool/generator/classes/course_backend.php`
     in this tree obtains the data generator, since it differs by version, then
     use `create_user`, `create_course`, `create_module`, `enrol_user` and the
     per-plugin generators in each plugin's `tests/generator/lib.php`.
   - For bulk volume, `admin/tool/generator/cli/maketestsite.php` and
     `maketestcourse.php` exist; check `--help`. Try the tree root, then `public/`.
   - Use direct SQL through `mdl-sql --write` only for states the APIs refuse
     to produce, such as deliberately odd timestamps.
4. **Make it look real.** Plausible names, dates spread over past months, varied
   progress and grades, a few users who haven't finished. Avoid "Test user 1".
5. **Purge caches after direct changes**, with `mdl-php 'purge_all_caches();'`.
6. **Check the result as a user would.** `mdl-screenshot --user=<name> /my/` and
   read the image. Check at least one teacher and one student view.
7. **Report** what you created, which logins show it best, and how to rerun it.

If you need code running inside Moodle, write a local plugin in `extrarepos/`,
commit it, and deploy it with `mdl-repo-add`.
