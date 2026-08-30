# Recipe overlay

Files placed here as `<vendor>/<stream>/<version>.yaml` are merged into the
recipe catalogue (`/srv/extra/mdl-recipes`) at image build — they appear in
the console's install form and in `mdl-demo recipes` alongside the upstream
catalogue. This is how a fork ships its own site recipes in the image.

Use your own vendor directory (say `acme/demo/1.0.yaml`) rather than
shadowing upstream paths: the catalogue is a live git checkout that a running
container can `git pull`, and an overlay file that overwrites a tracked
upstream file would block that.

A recipe here is an ordinary mudev recipe — `mudev recipe export` on a
prepared workspace writes one, or start from any file in
[mdl-recipes](https://github.com/mutms/mdl-recipes).
