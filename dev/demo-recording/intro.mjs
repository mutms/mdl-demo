// Records the typed-command intro: a terminal window types the one-line
// `container run …` command, presses enter, shows a brief start line, then
// holds. Frames are screencast the same way (1000x720 → scaled to 900 at
// assembly) so the intro concatenates seamlessly before the console frames.
//
//   node intro.mjs <frames-dir>
import { launch, openPage } from '/opt/mpd/assets/agents/browser/cdp.mjs';
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const outDir = process.argv[2] || './temp/intro_frames';
await mkdir(outDir, { recursive: true });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const CMD = 'container run -d --name mdl-demo-6391 -p 127.0.0.1:6391:8081 -p 127.0.0.1:6392:8082 ghcr.io/mutms/mdl-demo';

const browser = await launch();
const conn = browser.conn;
const page = await openPage(conn, pathToFileURL(join(process.cwd(), 'intro.html')).href, { width: 1000, height: 720 });
const S = page.sessionId;

let n = 0, capturing = false;
conn.ws.addEventListener('message', async (ev) => {
  let msg; try { msg = JSON.parse(ev.data); } catch { return; }
  if (msg.method !== 'Page.screencastFrame' || msg.sessionId !== S) return;
  conn.send('Page.screencastFrameAck', { sessionId: msg.params.sessionId }, S).catch(() => {});
  if (!capturing) return;
  const idx = String(++n).padStart(4, '0');
  await writeFile(join(outDir, `i${idx}.jpg`), Buffer.from(msg.params.data, 'base64'));
});
await conn.send('Page.startScreencast', { format: 'jpeg', quality: 80, everyNthFrame: 1 }, S);

capturing = true;
await sleep(500);            // hold on the empty prompt

// Type the command. Screencast fires on each DOM change (repaint).
for (let i = 1; i <= CMD.length; i++) {
  await page.eval(`document.getElementById('cmd').textContent = ${JSON.stringify(CMD.slice(0, i))}`);
  await sleep(28);           // brisk, human-ish typing
}
await sleep(600);            // pause on the full command
// Enter: drop the caret, reveal the output lines.
await page.eval(`document.getElementById('caret').style.display='none';
  document.getElementById('o1').style.opacity='1';`);
await sleep(700);
await page.eval(`document.getElementById('o2').style.opacity='1';`);
await sleep(1100);           // hold on "Console ready" before the cut

await conn.send('Page.stopScreencast', {}, S).catch(() => {});
await sleep(150);
await browser.close();
console.log(`intro: captured ${n} frames -> ${outDir}`);
