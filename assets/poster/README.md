# Poster — a custom card for your demo

Drop a few files here and your image grows one extra card on the console's
**Tools** grid (first slot) plus a sub-page behind it — no code change, no
rebrand. The stock image ships none of this; the mdl-demo identity stays put, so
the poster reads as *your* addition on top. Use it for a conference shout-out
("the talk this demo is part of"), a **welcome page**, an "Our university" card,
a support link — whatever you want people to find first.

It's baked at build time (`COPY assets/poster/` in the Containerfile), so it's
part of a **custom image you build yourself** — see `dev/`. These files are
gitignored on purpose (only this README is tracked), so they never leak into the
official image.

## The files

All optional except `title.html` — no `title.html`, no poster.

| file         | what it is                                                      |
|--------------|-----------------------------------------------------------------|
| `title.html` | **plain text** — the card title, breadcrumb, and page heading   |
| `blurb.html` | one-line blurb under the tile title (HTML fragment)             |
| `page.html`  | the sub-page body (HTML fragment)                               |
| `logo.svg`   | the tile icon — an inline SVG (a star is used if you omit it)   |

## The one rule: no inline CSS

The console's Content-Security-Policy forbids inline styles (`style="…"` and
`<style>` blocks are silently dropped — it's a security guard, see
`internal/webui/auth.go`). Style with the classes the console already ships
(Pico CSS + `static/app.css`) instead. For the icon, match the built-in tiles:

```svg
<svg class="tool-i" width="17" height="17" viewBox="0 0 24 24" fill="none"
     stroke="currentColor" stroke-width="2" stroke-linecap="round"
     stroke-linejoin="round" aria-hidden="true"> … </svg>
```

The HTML in `blurb.html`/`page.html` is trusted (you baked it) and injected
verbatim — the same footing as the recipes and the recommendations list.
