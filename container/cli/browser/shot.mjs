#!/usr/bin/env node
// shot.mjs <url> [out.png] — save a full-page screenshot of a URL using the
// system chromium, so an agent can eyeball a local web UI without a real
// browser. Dependency-free (see cdp.mjs).
//
//   node shot.mjs http://127.0.0.1:8081 console.png
//
// The agent can then read the PNG. Settles ~300ms after load so late htmx/JS
// swaps are captured. Env: CHROMIUM=/path, SANDBOX=1 (see cdp.mjs).

import { launch, openPage } from './cdp.mjs';

const url = process.argv[2];
const out = process.argv[3] || 'shot.png';
if (!url) {
  console.error('usage: node shot.mjs <url> [out.png]');
  process.exit(2);
}

const browser = await launch();
try {
  const page = await openPage(browser.conn, url);
  await new Promise((r) => setTimeout(r, 300));
  await page.screenshot(out);
  console.log('saved', out);
} finally {
  await browser.close();
}
