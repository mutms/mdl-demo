# Vendored front-end assets

Committed rather than fetched, and served from the mdl-demo binary itself
(`go:embed`): the management UI must render inside a container with no
guaranteed internet, and it must never reach a CDN.

| File           | Version | Source                                                              | License |
|----------------|---------|---------------------------------------------------------------------|---------|
| `htmx.min.js`  | 2.0.10  | `https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js`     | 0BSD    |
| `pico.min.css` | 2.1.1   | `https://cdn.jsdelivr.net/npm/@picocss/pico@2.1.1/css/pico.min.css` | MIT     |

```
htmx.min.js   sha256  71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de  size 51238
pico.min.css  sha256  fbc9a63fc9fc9f72d12fd7fc9806e11fa9f77ae4f9cad146b27003a1119ba3db  size 83319
```

Pico's MIT license requires keeping its copyright notice — the minified file's
header comment carries it and stays.

0BSD imposes no conditions, so there is nothing to reproduce in mdl-demo's
own GPL-3 distribution; this file records provenance, not obligation.

## Updating

```sh
cd go/internal/webui/static
curl -fsSL -o htmx.min.js https://cdn.jsdelivr.net/npm/htmx.org@<version>/dist/htmx.min.js
sha256sum htmx.min.js          # update the table above
```
