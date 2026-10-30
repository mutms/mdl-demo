// cdp.mjs — a tiny, dependency-free Chrome DevTools Protocol client.
//
// It launches the system chromium headless, talks CDP over Node's built-in
// WebSocket (global since Node 22), and cleans up after itself. There are NO
// npm packages: the whole trusted surface is this file plus shot.mjs, so it can
// be read top-to-bottom before it rides along to another VM. Nothing here
// reaches the network except the chromium you point it at.
//
// Security notes:
//   - chromium's debug port is bound to 127.0.0.1 by chromium itself and uses
//     an unguessable /devtools/browser/<uuid> path; we never expose it.
//   - a throwaway --user-data-dir is used and removed, so no profile persists.
//   - --no-sandbox (default) is fine for driving TRUSTED local pages in a
//     disposable dev VM; set SANDBOX=1 to keep chromium's sandbox for anything
//     less trusted.
//   - CHROMIUM=/path overrides the browser binary (default /usr/bin/chromium).

import { spawn } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const CHROMIUM = process.env.CHROMIUM || '/usr/bin/chromium';

// launch starts chromium and returns { conn, close }. conn is the CDP client.
export async function launch({ sandbox = process.env.SANDBOX === '1' } = {}) {
  const profile = await mkdtemp(join(tmpdir(), 'agent-chromium-'));
  const args = [
    '--headless=new',
    '--remote-debugging-port=0', // 0 = a free port, bound to loopback
    `--user-data-dir=${profile}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-gpu',
    '--hide-scrollbars',
  ];
  if (!sandbox) args.push('--no-sandbox');
  const proc = spawn(CHROMIUM, args, { stdio: ['ignore', 'ignore', 'pipe'] });

  // chromium announces "DevTools listening on ws://127.0.0.1:PORT/devtools/..."
  // on stderr; read it rather than guess the port.
  const wsUrl = await new Promise((resolve, reject) => {
    let buf = '';
    const timer = setTimeout(() => reject(new Error('chromium did not announce a DevTools endpoint in time')), 15000);
    proc.stderr.on('data', (d) => {
      buf += d;
      const m = buf.match(/ws:\/\/\S+/);
      if (m) { clearTimeout(timer); resolve(m[0]); }
    });
    proc.on('exit', (code) => { clearTimeout(timer); reject(new Error(`chromium exited early (code ${code})`)); });
  });

  const conn = await connect(wsUrl);
  return {
    conn,
    async close() {
      try { await conn.send('Browser.close'); } catch { /* already gone */ }
      try { conn.ws.close(); } catch { /* ignore */ }
      // Wait for chromium to actually exit before removing its profile —
      // otherwise open files race the rmdir (ENOTEMPTY).
      await new Promise((resolve) => {
        if (proc.exitCode !== null) return resolve();
        const done = setTimeout(() => { try { proc.kill('SIGKILL'); } catch { /* ignore */ } resolve(); }, 3000);
        proc.on('exit', () => { clearTimeout(done); resolve(); });
        try { proc.kill('SIGTERM'); } catch { /* ignore */ }
      });
      // Best-effort: a leftover temp profile is harmless, so never throw here.
      try { await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 }); } catch { /* ignore */ }
    },
  };
}

// connect opens the WebSocket and returns a small request/response + event API.
async function connect(wsUrl) {
  const ws = new WebSocket(wsUrl);
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = () => reject(new Error('could not connect to chromium DevTools'));
  });

  let nextId = 1;
  const pending = new Map(); // id -> { resolve, reject }
  const waiters = []; // { match(msg), resolve }

  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) reject(new Error(msg.error.message));
      else resolve(msg.result);
      return;
    }
    if (msg.method) {
      for (let i = waiters.length - 1; i >= 0; i--) {
        if (waiters[i].match(msg)) waiters.splice(i, 1)[0].resolve(msg);
      }
    }
  };

  // send a CDP command; sessionId targets a page (flat session), omit for browser.
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const id = nextId++;
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify(sessionId ? { id, method, params, sessionId } : { id, method, params }));
  });

  // resolve when a CDP event fires (optionally scoped to one page session).
  const waitFor = (method, { sessionId, timeout = 30000 } = {}) => new Promise((resolve, reject) => {
    const w = { match: (m) => m.method === method && (!sessionId || m.sessionId === sessionId), resolve };
    waiters.push(w);
    setTimeout(() => {
      const i = waiters.indexOf(w);
      if (i >= 0) { waiters.splice(i, 1); reject(new Error(`timed out waiting for ${method}`)); }
    }, timeout);
  });

  return { ws, send, waitFor };
}

// openPage creates a tab, navigates to url, and returns helpers to inspect it.
export async function openPage(conn, url, { width = 900, height = 1100, timeout = 30000 } = {}) {
  const { targetId } = await conn.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await conn.send('Target.attachToTarget', { targetId, flatten: true });
  await conn.send('Page.enable', {}, sessionId);
  await conn.send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false }, sessionId);

  const loaded = conn.waitFor('Page.loadEventFired', { sessionId, timeout });
  await conn.send('Page.navigate', { url }, sessionId);
  await loaded;

  return {
    sessionId,
    // Run JS in the page and return a JSON-able value (throws on page errors).
    async eval(expression) {
      const r = await conn.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, sessionId);
      if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text);
      return r.result.value;
    },
    // Save a PNG (full page by default).
    async screenshot(path, { fullPage = true } = {}) {
      let clip;
      if (fullPage) {
        const m = await conn.send('Page.getLayoutMetrics', {}, sessionId);
        const size = m.cssContentSize || m.contentSize;
        clip = { x: 0, y: 0, width: Math.ceil(size.width), height: Math.ceil(size.height), scale: 1 };
      }
      const { data } = await conn.send('Page.captureScreenshot', { format: 'png', clip, captureBeyondViewport: fullPage }, sessionId);
      await writeFile(path, Buffer.from(data, 'base64'));
    },
  };
}
