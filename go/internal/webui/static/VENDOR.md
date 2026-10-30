# Vendored front-end assets

Committed rather than fetched, and served from the mdl-demo binary itself
(`go:embed`): the management UI must render inside a container with no
guaranteed internet, and it must never reach a CDN.

| File           | Version | Source                                                              | License |
|----------------|---------|---------------------------------------------------------------------|---------|
| `htmx.min.js`  | 2.0.10  | `https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js`     | 0BSD    |
| `pico.min.css` | 2.1.1   | `https://cdn.jsdelivr.net/npm/@picocss/pico@2.1.1/css/pico.min.css` | MIT     |
| `xterm/xterm.js`, `xterm/xterm.css` | 6.0.0 | `https://registry.npmjs.org/@xterm/xterm/-/xterm-6.0.0.tgz` (`lib/`, `css/`) | MIT |
| `xterm/addon-fit.js` | 0.11.0 | `https://registry.npmjs.org/@xterm/addon-fit/-/addon-fit-0.11.0.tgz` | MIT |
| `xterm/addon-web-links.js` | 0.12.0 | `https://registry.npmjs.org/@xterm/addon-web-links/-/addon-web-links-0.12.0.tgz` | MIT |
| `xterm/addon-unicode11.js` | 0.9.0 | `https://registry.npmjs.org/@xterm/addon-unicode11/-/addon-unicode11-0.9.0.tgz` | MIT |
| `xterm/addon-clipboard.js` | 0.2.0 | `https://registry.npmjs.org/@xterm/addon-clipboard/-/addon-clipboard-0.2.0.tgz` | MIT |

```
htmx.min.js   sha256  71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de  size 51238
pico.min.css  sha256  fbc9a63fc9fc9f72d12fd7fc9806e11fa9f77ae4f9cad146b27003a1119ba3db  size 83319
xterm/xterm.js            sha256  98d0973151aff2991d335b1adbbdac2e14da26341abe329d677d4c0034402bdf  size 488630
xterm/xterm.css           sha256  854a7c0fb70e8b1a083c16797ab827299fb18744f5ad34f227b48337e33293c6  size 7112
xterm/addon-fit.js        sha256  4df3789fa7bff5d545ba38462a8cd2f7460ba971021bce3f81c2870f937a201e  size 1484
xterm/addon-web-links.js  sha256  e34337f790d1cc817312a1ac1c00c473a7c504f471a70086db57ca9cd0335ca9  size 3057
xterm/addon-unicode11.js  sha256  19b5afa48aa21fb59a0337194f2c66e09d8b19f929be91cb3d2ededc4374f4f4  size 52446
xterm/addon-clipboard.js  sha256  c7728bf2c648e971cf445d3f1545de2c2b014ca2cacb3dac361cf7c468721fca  size 6341
```

Pico's MIT license requires keeping its copyright notice — the minified file's
header comment carries it and stays.

xterm.js's MIT license requires keeping its copyright notice: `xterm/LICENSE`
carries it and stays. The sha256 values are of the files as vendored, with the
trailing `sourceMappingURL` line removed.

0BSD imposes no conditions, so there is nothing to reproduce in mdl-demo's
own GPL-3 distribution; this file records provenance, not obligation.

## Updating

```sh
cd go/internal/webui/static
curl -fsSL -o htmx.min.js https://cdn.jsdelivr.net/npm/htmx.org@<version>/dist/htmx.min.js
sha256sum htmx.min.js          # update the table above
```
