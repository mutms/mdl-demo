#!/usr/bin/env bash
#
# Turns the two raw frame captures (intro_frames/ + frames/) into the finished
# demo.webp / demo.gif. It does NOT record anything - run record.mjs and
# intro.mjs first (see README.md), THEN this.
#
#       bash dev/demo-recording/build.sh
#
# The whole point of this script is the CUT: a screencast of a real install is
# minutes of a scrolling log nobody wants to watch. So we film only the clicks
# and a beat of the log, then jump straight to the logged-in payoff. That means
# a few frame-number boundaries have to be tuned BY EYE for each fresh recording
# (screencast timing drifts run to run). They live right below - open the frames
# they point at, check they're on the right beat, adjust, re-run. It's seconds.
#
# Everything scratch goes under temp/ (gitignored). Outputs land next to it and
# get copied to www by hand (README says where).
#
#   --skodak
#
set -euo pipefail
cd "$(dirname "$0")"

# ---- tunables: eyeball these against the frames after each recording ----
FRAMES="temp/frames"        # console recording (record.mjs), f%04d.jpg
INTRO="temp/intro_frames"   # terminal intro (intro.mjs),      i%04d.jpg
FPS=24                   # playback rate; total secs = frames / FPS
W=900                    # output width

# Console frames split into three beats. n is 0-based (f0001.jpg = n0).
#   A = interaction: cursor glides, pick recipe, click Install   -> keep, dedupe
#   B = installing:  a live slice of the streaming log           -> subsample
#   C = payoff:      installed dashboard -> login -> Moodle       -> keep, dedupe
# Find the two boundaries by reading frames around them:
#   A_END   last interaction frame BEFORE the log floods (look at f0140-ish)
#   C_START last log frame; C_START+1 is the installed dashboard (f0365-ish)
A_END=140
B_LO=140; B_HI=250; B_STEP=2   # every B_STEP-th log frame in [B_LO,B_HI] = the live "installing" stretch
C_START=365

HOLD_END_S=2.0           # freeze on the ready Moodle dashboard at the end
HOLD_INTRO_S=0.8         # freeze on "Console ready" before cutting to the console
DEDUPE="mpdecimate=hi=64*12:lo=64*5:frac=0.1"   # drops near-identical (static) frames
# ------------------------------------------------------------------------

WORK="temp/build"   # scratch, gitignored (dev/*/temp/)
rm -rf "$WORK"; mkdir -p "$WORK"/{intro,a,b,c,seq}

sel() {  # sel <srcdir> <pattern> <select-expr> <outdir> [extra-filters]
  ffmpeg -y -framerate 30 -i "$1/$2" -vf "select='$3'${5:+,$5},setpts=N/TB" -vsync vfr "$4/%05d.png" 2>/dev/null
}
hold() { # hold <dir> <seconds> : append copies of the last frame
  local d="$1" secs="$2" last cnt j n; last="$d/$(ls "$d" | sort | tail -1)"
  cnt=$(ls "$d" | wc -l); for j in $(seq 1 "$(printf '%.0f' "$(echo "$secs*$FPS" | bc -l)")"); do
    printf -v n "%05d.png" $((cnt+j)); cp "$last" "$d/$n"; done
}

# intro: dedupe holds, keep every 2nd typing frame (punchier), then a beat
sel "$INTRO" "i%04d.jpg" "not(mod(n\,2))" "$WORK/intro" "$DEDUPE"
hold "$WORK/intro" "$HOLD_INTRO_S"
# A / B / C
sel "$FRAMES" "f%04d.jpg" "lt(n\,$A_END)"                          "$WORK/a" "$DEDUPE"
sel "$FRAMES" "f%04d.jpg" "between(n\,$B_LO\,$B_HI)*not(mod(n\,$B_STEP))" "$WORK/b"
sel "$FRAMES" "f%04d.jpg" "gt(n\,$C_START)"                        "$WORK/c" "$DEDUPE"

# concat intro -> a -> b -> c, then freeze on the last frame
i=0
for d in intro a b c; do for f in $(ls "$WORK/$d"/*.png | sort); do
  printf -v n "%05d.png" $i; cp "$f" "$WORK/seq/$n"; i=$((i+1)); done; done
hold "$WORK/seq" "$HOLD_END_S"
echo "assembled $(ls "$WORK/seq" | wc -l) frames -> $(node -e "console.log(($(ls "$WORK/seq"|wc -l)/$FPS).toFixed(1)+'s')")"

# webp (ship this) + gif (fallback / preview). Both land in temp/ (gitignored) -
# copy the webp to mutms/www to publish it (see README).
ffmpeg -y -framerate "$FPS" -i "$WORK/seq/%05d.png" -vf "scale=$W:-1:flags=lanczos,palettegen=stats_mode=diff" "$WORK/pal.png" >/dev/null 2>&1
ffmpeg -y -framerate "$FPS" -i "$WORK/seq/%05d.png" -i "$WORK/pal.png" -lavfi "scale=$W:-1:flags=lanczos[x];[x][1:v]paletteuse" temp/demo.gif >/dev/null 2>&1
ffmpeg -y -framerate "$FPS" -i "$WORK/seq/%05d.png" -vf "scale=$W:-1:flags=lanczos" -loop 0 -c:v libwebp_anim -q:v 72 temp/demo.webp >/dev/null 2>&1
rm -rf "$WORK"
ls -lh temp/demo.webp temp/demo.gif
