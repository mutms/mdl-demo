// Demo recorder for the mdl-demo console. Drives the console over CDP with a
// VISIBLE injected cursor, screencasts the frames, then assemble.sh turns them
// into an animated webp/gif. Scenario: empty dashboard → install the latest full
// MuTMS 5.2 → log in as admin → land in the running Moodle site.
//
// Two fixes over the first prototype:
//   1. Mouse visibility — CDP screencast never captures the OS cursor, so we
//      inject a pointer element and glide it to each target before clicking.
//      Re-injected after every full navigation (the DOM resets).
//   2. Login transition — the console's "Log in as" form is target="_blank"
//      (new tab), which the screencast (bound to one target) couldn't follow.
//      We flip it to _self so the SAME tab navigates into the logged-in site,
//      and the screencast rolls straight through.
//
// The long install itself is not filmed: we capture the clicks + a beat of the
// streaming log, pause capture while the site builds, then resume for the login
// and the payoff. That keeps the demo short (a cut, standard for this).
//
//   node record.mjs <console-url> <frames-dir>
import { launch, openPage } from '/opt/mpd/assets/agents/browser/cdp.mjs';
import { mkdir, writeFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';

const url = process.argv[2] || 'http://10.163.222.1:6391';
const outDir = process.argv[3] || './temp/frames';
await mkdir(outDir, { recursive: true });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const browser = await launch();
const conn = browser.conn;
const page = await openPage(conn, url, { width: 1000, height: 720 });
const S = page.sessionId;

// --- screencast: always ack (so frames keep coming), only save when capturing ---
let n = 0, capturing = false;
conn.ws.addEventListener('message', async (ev) => {
  let msg; try { msg = JSON.parse(ev.data); } catch { return; }
  if (msg.method !== 'Page.screencastFrame' || msg.sessionId !== S) return;
  conn.send('Page.screencastFrameAck', { sessionId: msg.params.sessionId }, S).catch(() => {});
  if (!capturing) return;
  const idx = String(++n).padStart(4, '0');
  await writeFile(join(outDir, `f${idx}.jpg`), Buffer.from(msg.params.data, 'base64'));
});
await conn.send('Page.startScreencast', { format: 'jpeg', quality: 80, everyNthFrame: 1 }, S);

// --- visible cursor overlay (idempotent; call after every navigation) ---
const CURSOR = String.raw`(() => {
  if (document.getElementById('__cur')) return;
  const c = document.createElement('div');
  c.id = '__cur';
  c.style.cssText = 'position:fixed;left:0;top:0;z-index:2147483647;pointer-events:none;'
    + 'transition:left .4s cubic-bezier(.45,0,.25,1),top .4s cubic-bezier(.45,0,.25,1);';
  c.innerHTML = '<svg width="24" height="24" viewBox="0 0 24 24" style="display:block;'
    + 'filter:drop-shadow(0 1px 2px rgba(0,0,0,.45))"><path d="M4 2l7 17 2.3-6.7L20 10z" '
    + 'fill="#111" stroke="#fff" stroke-width="1.4" stroke-linejoin="round"/></svg>';
  document.documentElement.appendChild(c);
  window.__cur = (x, y) => { c.style.left = (x - 3) + 'px'; c.style.top = (y - 2) + 'px'; };
  window.__cur(60, 60);
})();`;
const injectCursor = () => page.eval(CURSOR).catch(() => {});

// Center of an element found by a JS expression returning an Element.
async function centerOf(expr) {
  return page.eval(`(() => { const e = (${expr}); if (!e) return null;
    e.scrollIntoView({block:'center',inline:'center'});
    const r = e.getBoundingClientRect(); return { x: Math.round(r.left+r.width/2), y: Math.round(r.top+r.height/2) }; })()`);
}
// Glide the cursor to the element, pause so the move is filmed, then click it.
async function clickExpr(expr, { settle = 650 } = {}) {
  const p = await centerOf(expr);
  if (!p) throw new Error('element not found: ' + expr);
  await page.eval(`window.__cur && window.__cur(${p.x}, ${p.y})`);
  await sleep(settle);
  await page.eval(`(${expr}).click()`);
}

// ---- Phase A: install the latest full MuTMS 5.2 ----
capturing = true;
await injectCursor();
await sleep(1000);

// MuTMS vendor tab (match by label, robust to the slug).
await clickExpr(`[...document.querySelectorAll('.tab')].find(t => /mutms/i.test(t.textContent))`);
await sleep(700);
// Latest full-suite 5.2 recipe (mutms/release/5.2.x).
await clickExpr(`document.querySelector('input[name="recipe"][value^="mutms/release/5.2"]')?.closest('.pkg')`);
await sleep(900);
// Install — selection is the confirmation.
await clickExpr(`document.querySelector('button.install')`);
await sleep(4500); // let the busy card + streaming log show for a beat

// ---- pause capture while the site actually builds (not filmed) ----
capturing = false;
process.stdout.write('installing (not filmed)…');
let installed = false;
for (let i = 0; i < 150; i++) { // up to ~7.5 min
  await sleep(3000);
  installed = await page.eval(`!!document.querySelector('.actions form[action="/reset"]')`).catch(() => false);
  process.stdout.write('.');
  if (installed) break;
}
console.log(installed ? ' done' : ' TIMEOUT');

// ---- Phase B: log in as admin, follow into the site ----
await injectCursor();
capturing = true;

// The Accounts card renders a beat behind the site card (separate poll), so
// wait for the admin "Log in" button before reaching for it.
const adminBtn = `[...document.querySelectorAll('#users tr')]`
  + `.find(tr => /\\badmin\\b/i.test(tr.querySelector('.name')?.textContent || ''))?.querySelector('.login')`;
for (let i = 0; i < 25; i++) {
  if (await page.eval(`!!(${adminBtn})`).catch(() => false)) break;
  await sleep(1000);
}
await sleep(1200); // hold on the installed dashboard (site card + accounts)

// Admin row's "Log in" button → opens the SSO dialog (htmx into #ssobody).
await clickExpr(adminBtn);
await sleep(1100); // dialog loads

// Flip the login form to the SAME tab so the screencast follows into the site,
// glide to its button, submit, and wait for Moodle to come up.
await page.eval(`(() => { const f = document.querySelector('#ssodialog form[action="/sso/login"]'); if (f) f.target = '_self'; })()`);
await clickExpr(`document.querySelector('#ssodialog form[action="/sso/login"] button')`, { settle: 800 });
const loaded = conn.waitFor('Page.loadEventFired', { sessionId: S, timeout: 30000 }).catch(() => {});
await page.eval(`(() => { const f = document.querySelector('#ssodialog form[action="/sso/login"]'); (f.requestSubmit ? f.requestSubmit() : f.submit()); })()`);
await loaded;
await sleep(2500); // Moodle logs in, redirects to the dashboard — hold on it

await conn.send('Page.stopScreencast', {}, S).catch(() => {});
await sleep(200);
await browser.close();

const files = (await readdir(outDir)).filter((f) => f.endsWith('.jpg'));
console.log(`captured ${files.length} frames -> ${outDir}`);
