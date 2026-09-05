# demo-recording — the animated console walkthrough

This is the tooling that made the `demo.webp` in the www news post ("Try MuTMS
in one command"). It drives the real console over CDP, screencasts it, and cuts
the result down to a punchy ~15s loop:

> terminal types the `container run …` command → console appears → pick MuTMS 5.2
> → Install → a beat of the live install log → **cut** → log in as admin → land
> in the running Moodle, hold.

Nothing here ships in the image. It's a dev toy for regenerating the demo when
the console UI changes enough that the old capture looks stale.

## What you need

- An **mpd VM** (amd64) with a **running, image-built** test container you don't
  mind resetting - `record.mjs` calls the console's install/reset, so it wipes
  and rebuilds the site. Get your own, don't borrow a human's:
  `make run PORT=6391` (→ `mpd-test-mdl-demo-6391`, console 6391 / site 6392).
- The container's **wwwroot must be a reachable IP**, not 127.0.0.1 - otherwise
  the admin login at the end lands on a Moodle nobody can screencast. The VM
  bridge IP works: `mdl-demo url --site http://<bridge-ip>:6392` (or install with
  it already set). The capture above used `http://10.163.222.1:6392`.
- Build the recording container in **offline mode** (baked repos) so the
  minutes-long install `record.mjs` waits out doesn't depend on the network -
  no mid-recording rate-limit or flaky clone. `dev/prepare-offline-build.sh`
  fills `assets/` with local mirrors, then build the image the normal way; the
  install then assembles from `/srv/extra/repos` over `file://`.
- **cdp.mjs** - the dependency-free CDP client at
  `/opt/mpd/assets/agents/browser/cdp.mjs` (mpd ships it). Both `.mjs` files
  import it by that absolute path; fix the path if yours lives elsewhere.
- `node`, `ffmpeg` (with `libwebp_anim`), `bc`.

## The three steps

Run these from THIS directory. Everything lands under `temp/` (gitignored
scratch) - raw screencasts and the encoded output both.

```sh
# 1. the terminal intro (no container needed - it's a static HTML page).
#    Edit the CMD + the "Console ready" URL in intro.mjs / intro.html to match
#    the port your recording actually shows, or it looks wrong.
node intro.mjs                          # -> temp/intro_frames/

# 2. the console flow. Resets the container, installs MuTMS 5.2, logs in as
#    admin. Minutes - it waits out the real install. Point it at your console:
node record.mjs http://<vm-ip>:6391     # -> temp/frames/

# 3. cut + stitch + encode -> temp/demo.webp (+ temp/demo.gif). See below.
bash build.sh
```

The finished `demo.webp` does NOT live in this repo - it belongs to the website,
**[github.com/mutms/www](https://github.com/mutms/www)**, at
`public/news/<slug>/demo.webp`, and gets committed and published there. If you
have that repo checked out next door it's just:

```sh
cp temp/demo.webp ../../../www/public/news/mdl-demo/demo.webp   # then commit in mutms/www
```

## The cut is hand-tuned (this is the fiddly bit)

A screencast of a real install is minutes of scrolling log. `build.sh` throws
almost all of it away and keeps three beats of the console recording -
interaction, a short *live* slice of the install log, then the payoff. Where one
beat ends and the next begins are **frame numbers you tune by eye** at the top of
`build.sh` (`A_END`, `B_LO/B_HI`, `C_START`), because the screencast's timing
drifts every run. To find them, just look at the frames:

```sh
# open the frames around each boundary and check they're on the right beat
xdg-open temp/frames/f0140.jpg   # A_END: last click before the log floods
xdg-open temp/frames/f0365.jpg   # C_START: last log frame; +1 is the installed dashboard
```

Nudge the numbers, re-run `build.sh` (seconds), repeat until it flows. The other
knobs (`FPS`, `HOLD_END_S`, the intro subsample) are there if you want it faster
or slower.

## Files

- `intro.html` / `intro.mjs` - the fake terminal that types the run command.
- `record.mjs` - drives the console: visible injected cursor (CDP screencasts
  never capture the OS one), pauses capture during the real install, flips the
  SSO login form to `_self` so the screencast follows into the site.
- `build.sh` - the cut + concat + encode. All the tunables live at the top.
