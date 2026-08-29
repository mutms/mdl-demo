package webui

// Templates live as Go string constants rather than embedded .html files so
// a section and the view type that feeds it can be read side by side (same
// convention as mpd's portal, whose look this UI follows). Every value is
// escaped by html/template; nothing here interpolates raw.

import "html/template"

// page holds the whole template set: the shell plus one named template per
// section/page. Sections are addressable on their own — that is what htmx
// fetches to refresh one card without touching the rest.
var page = template.Must(template.New("page").Parse(
	shellHTML + siteHTML + usersHTML + servicesHTML + helpHTML + progressHTML +
		loginHTML + setupHTML + installHTML + debugHTML + footerHTML + copyHTML + secretHTML + ssoHTML))

// footerHTML is the GPLv3 "Appropriate Legal Notice" (GPL-3.0 §0, §5(d)):
// it identifies the author and the project on every page of the interactive
// UI, and the license requires modified versions to keep displaying it.
//
// Forking this project? Please do — that is what the GPL is for. Add your
// own copyright line right here next to the original one (the license asks
// you to keep the existing notice, not to stop at it).
const footerHTML = `{{define "footer"}}
<footer style="margin-top:2rem; font-size:.78rem; color:var(--dim)">
  © 2026 Petr Skoda —
  <a href="https://github.com/mutms/mdl-demo">mdl-demo</a>, part of the
  <a href="https://github.com/mutms">MuTMS</a> project —
  <a href="https://www.gnu.org/licenses/gpl-3.0.html">GPL-3.0 or later</a>
</footer>
{{end}}`

const styleHTML = `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<script src="/static/htmx.min.js"></script>
<style>
  :root {
    color-scheme: light dark;
    --fg: #111827; --dim: #6b7280; --line: #d1d5db66;
    --ok: #15803d; --okbg: #16a34a1f; --idle: #6b7280; --idlebg: #9ca3af22;
    --err: #b91c1c; --errbg: #dc26261a;
    --card: #ffffff; --bg: #f9fafb; --accent: #1d4ed8;
  }
  @media (prefers-color-scheme: dark) {
    :root { --fg: #e5e7eb; --dim: #9ca3af; --line: #37415188;
            --ok: #4ade80; --okbg: #16a34a26; --err: #f87171; --errbg: #dc262626;
            --card: #111827; --bg: #0b0f19; --accent: #60a5fa; }
  }
  * { box-sizing: border-box; }
  body { font: 15px/1.55 system-ui, -apple-system, "Segoe UI", sans-serif;
         color: var(--fg); background: var(--bg);
         margin: 0 auto; padding: 2rem 1.5rem 4rem; max-width: 52rem; }
  header { margin-bottom: 2rem; display: flex; justify-content: space-between; align-items: baseline; }
  h1 { font-size: 1.35rem; margin: 0; letter-spacing: -.01em; }
  h1 span { color: var(--dim); font-weight: 400; }
  .sub { color: var(--dim); margin: .2rem 0 0; font-size: .85rem; }
  section { background: var(--card); border: 1px solid var(--line);
            border-radius: 10px; padding: 1rem 1.15rem 1.15rem; margin-bottom: 1.25rem; }
  h2 { font-size: .78rem; text-transform: uppercase; letter-spacing: .07em;
       color: var(--dim); margin: 0 0 .75rem; font-weight: 600; }
  table { border-collapse: collapse; width: 100%; }
  th { text-align: left; font-size: .72rem; text-transform: uppercase;
       letter-spacing: .05em; color: var(--dim); font-weight: 600;
       padding: 0 .6rem .4rem 0; }
  td { padding: .4rem .6rem .4rem 0; border-top: 1px solid var(--line);
       vertical-align: top; }
  td.meta { color: var(--dim); font-family: ui-monospace, SFMono-Regular, monospace;
            font-size: .82rem; }
  .name { font-weight: 600; }
  .role { color: var(--dim); font-weight: 400; font-size: .82rem; }
  .badge { display: inline-block; font-size: .72rem; padding: .05rem .5rem;
           border-radius: 999px; background: var(--idlebg); color: var(--idle); }
  .badge.on { background: var(--okbg); color: var(--ok); }
  .badge.err { background: var(--errbg); color: var(--err); }
  .empty { color: var(--dim); font-size: .88rem; padding: .3rem 0; }
  .cred { user-select: all; }
  dl.site { display: grid; grid-template-columns: max-content 1fr;
            gap: .45rem .9rem; margin: 0; align-items: baseline; }
  dl.site dt { font-size: .72rem; text-transform: uppercase; letter-spacing: .05em;
               color: var(--dim); font-weight: 600; }
  dl.site dd { margin: 0; font-family: ui-monospace, SFMono-Regular, monospace;
               font-size: .82rem; color: var(--dim); overflow-wrap: anywhere; }
  dl.site dd.name { font-family: inherit; font-size: inherit; font-weight: 600; color: var(--fg); }
  a { color: inherit; text-underline-offset: 2px; }
  a:hover { color: var(--accent); text-decoration-color: var(--accent); }
  form.stack { display: grid; gap: .8rem; max-width: 26rem; }
  label { display: grid; gap: .25rem; font-size: .85rem; color: var(--dim); }
  input, select { font: inherit; color: var(--fg); background: var(--bg);
    border: 1px solid var(--line); border-radius: 7px; padding: .45rem .6rem; }
  button { font: inherit; font-weight: 600; border: 0; border-radius: 7px;
    padding: .5rem .9rem; background: var(--accent); color: #fff; cursor: pointer; }
  button.subtle { background: var(--idlebg); color: var(--fg); font-weight: 400; }
  button:disabled { opacity: .5; cursor: not-allowed; }
  .error { color: var(--err); background: var(--errbg); border-radius: 7px;
    padding: .5rem .7rem; font-size: .88rem; }
  pre.log { font: .78rem/1.45 ui-monospace, SFMono-Regular, monospace;
    background: var(--bg); border: 1px solid var(--line); border-radius: 7px;
    padding: .7rem .8rem; overflow: auto; white-space: pre-wrap; margin: 0;
    max-height: 22rem; }
  pre.log.short { max-height: 5cm; }
  .row { display: flex; gap: .6rem; align-items: center; }
  .spin { display: inline-block; width: .85em; height: .85em; margin-right: .35em;
    border: 2px solid var(--line); border-top-color: var(--accent);
    border-radius: 50%; vertical-align: -.1em;
    animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  button.copy, button.reveal, button.qr { background: none; border: 0; padding: 0 .1rem;
    margin-left: .15rem; color: var(--dim); cursor: pointer;
    vertical-align: -.15em; line-height: 1; }
  button.copy:hover, button.reveal:hover, button.qr:hover { color: var(--fg); }
  dialog { border: 1px solid var(--line); border-radius: 12px; padding: 1.1rem;
    background: var(--card); color: var(--fg); width: min(30rem, 92vw); min-height: 8rem; }
  dialog::backdrop { background: rgba(0,0,0,.55); }
  dialog#qrdialog { background: #fff; width: auto; min-height: 0; }
  dialog#qrdialog img { display: block; width: min(75vmin, 520px); height: auto; }
  button.dlgclose { position: absolute; top: .45rem; right: .55rem; background: none;
    border: 0; padding: .2rem; color: var(--dim); font-size: 1.15rem; line-height: 1;
    cursor: pointer; }
  button.dlgclose:hover { color: var(--fg); }
  img.ssoqr { display: block; width: min(70vmin, 440px); height: auto; margin: .7rem auto 0; }
  button.copy.ok { color: var(--ok); }
  button.reveal { margin-left: .55rem; }
  button.reveal.on { color: var(--accent); }
  .secret code[data-copy] { cursor: pointer; }
  .secret-val { user-select: all; }
</style>` + scriptHTML

// copyHTML is the copy-to-clipboard button; invoked as {{template "copy" <text>}}.
// The click handler lives in scriptHTML (delegated, so it survives htmx swaps).
const copyHTML = `{{define "copy"}}<button class="copy" type="button" data-copy="{{.}}" title="Copy to clipboard"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg></button>{{end}}`

// secretHTML masks a credential by default — a classroom console lives on a
// projector, and an admin password does not belong on the wall. The value is
// shown as dots with a reveal (eye) toggle and the copy button, so it can be
// grabbed without ever being displayed. Not real secrecy — the value is in the
// DOM for copy to reach — just "off the screen unless asked". Invoked as
// {{template "secret" <text>}}; the reveal handler lives in scriptHTML.
// Order and spacing are deliberate: copy (the safe, everyday action) sits right
// next to the value, and reveal (which puts the password on screen) is pushed
// off to the side with a gap, so a reach for copy cannot accidentally unmask it.
const secretHTML = `{{define "secret"}}<span class="secret"><code class="cred secret-val" data-secret="{{.}}" data-copy="{{.}}" title="Click to copy">••••••••</code>{{template "copy" .}}<button class="reveal" type="button" title="Reveal" aria-label="Reveal"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg></button></span>{{end}}`

// navigator.clipboard needs a secure context — http://localhost qualifies, but
// browsing via a LAN/VM IP does not, hence the hidden-textarea fallback.
const scriptHTML = `<script>
document.addEventListener('click', function (e) {
  var b = e.target.closest('[data-copy]');
  if (!b) return;
  var text = b.dataset.copy;
  // Flash the check on the copy icon, never on the masked value — swapping the
  // dots' content would make them jump. Clicking the dots or the icon both land
  // here; either way the feedback is on the icon and the ••••• stays put.
  var wrap = b.closest('.secret');
  var fb = (wrap && wrap.querySelector('button.copy')) || b;
  var done = function () {
    // Skip if the check is already showing (a double-click) — re-entering here
    // would capture the ✓ as "old" and restore it to ✓, leaving it stuck. The
    // copy above still happened; only the redundant flash is dropped.
    if (fb.classList.contains('ok')) return;
    var old = fb.innerHTML;
    fb.classList.add('ok');
    fb.innerHTML = '✓';
    setTimeout(function () { fb.classList.remove('ok'); fb.innerHTML = old; }, 1200);
  };
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(done);
  } else {
    var t = document.createElement('textarea');
    t.value = text;
    t.style.position = 'fixed';
    t.style.opacity = '0';
    document.body.appendChild(t);
    t.select();
    try { document.execCommand('copy'); done(); } catch (err) {}
    t.remove();
  }
});
document.addEventListener('click', function (e) {
  var c = e.target.closest('button.dlgclose');
  if (c) c.closest('dialog').close();
});
// The QR dialog lives outside the polled #site section, so a 5s refresh swap
// cannot close it mid-presentation; any click (or Esc) dismisses it.
document.addEventListener('click', function (e) {
  var q = e.target.closest('button.qr');
  if (!q) return;
  var d = document.getElementById('qrdialog');
  if (!d) return;
  d.querySelector('img').src = '/tunnel/qr.png?' + Date.now();
  d.showModal();
});
document.addEventListener('click', function (e) {
  var r = e.target.closest('button.reveal');
  if (!r) return;
  var val = r.parentNode.querySelector('.secret-val');
  if (!val) return;
  var shown = r.classList.toggle('on');
  val.textContent = shown ? val.dataset.secret : '••••••••';
  r.title = shown ? 'Hide' : 'Reveal';
});
</script>`

const shellHTML = `{{define "page"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header>
  <div>
    <h1>{{.ID}} <span>— {{if .Name}}{{.Name}}{{else}}Moodle demo console{{end}}</span></h1>
    <p class="sub">{{.Version}}</p>
  </div>
  <form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="subtle">Log out</button></form>
</header>

{{template "site" .}}
{{template "users" .}}
{{template "progress" .}}
{{template "help" .}}
<dialog id="qrdialog" onclick="this.close()"><button class="dlgclose" type="button" aria-label="Close">×</button><img alt="QR code for the tunnel URL"></dialog>
{{/* The close button sits outside #ssobody so htmx stage swaps keep it. */}}
<dialog id="ssodialog" onclick="if(event.target===this)this.close()"><button class="dlgclose" type="button" aria-label="Close">×</button><div id="ssobody"></div></dialog>
{{if .Installed}}
<dialog id="createdialog" onclick="if(event.target===this)this.close()">
  <button class="dlgclose" type="button" aria-label="Close">×</button>
  {{/* The password is never asked for: generated like the admin's, shown
       masked on the Accounts card — the whole point of console-side users. */}}
  <form class="stack" method="post" action="/users/create">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <label>Username
      <input name="username" required pattern="[a-z0-9._-]+" title="lowercase letters, digits, . _ -">
    </label>
    <label>First name <input name="firstname" required maxlength="100"></label>
    <label>Last name <input name="lastname" required maxlength="100"></label>
    <label>Global role
      <select name="role">
        <option value="">None (plain user)</option>
        <option value="manager">Manager</option>
        <option value="admin">Administrator</option>
      </select>
    </label>
    <div><button>Create user</button></div>
  </form>
</dialog>
{{end}}
{{template "footer" .}}
</html>{{end}}`

const siteHTML = `{{define "site"}}
<section id="site" hx-get="/section/site" hx-trigger="every 5s" hx-swap="outerHTML">
  <h2>Demo site</h2>
  {{if .Installed}}
  {{/* One site per container, so its details are a description list, not a
       one-row table. */}}
  <dl class="site">
    <dt>Recipe</dt><dd class="name">{{.Recipe}}</dd>
    {{/* New tab on purpose: landing inside Moodle in the same tab loses
         people — they forget the management UI's address to get back. */}}
    <dt>URL</dt><dd><a href="{{.Wwwroot}}" target="_blank" rel="noopener">{{.Wwwroot}}</a></dd>
    {{if .TunnelURL}}<dt>Tunnel</dt><dd><a href="{{.TunnelURL}}" target="_blank" rel="noopener">{{.TunnelURL}}</a><button class="qr" type="button" title="Show QR code" aria-label="Show QR code"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3z"/><path d="M21 14v.01M14 21v.01M21 21v.01M17.5 17.5v.01"/></svg></button></dd>{{end}}
    <dt>Installed</dt><dd>{{.InstalledAt}}</dd>
  </dl>
  {{/* Actions live in a row so future ones slot in beside Reset. Restore backup
       appears in both states: here (site installed) it swaps data onto the
       current tree; from empty it reinstalls the backup's recipe first. */}}
  <div class="row" style="margin:.9rem 0 0">
    <form method="post" action="/reset"
          onsubmit="return confirm('Wipe the demo site? The database, code tree and all data are deleted.')">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="subtle">Reset site…</button>
    </form>
    <button class="subtle" disabled title="Coming soon">Back up data…</button>
    <button class="subtle" disabled title="Coming soon">Restore backup…</button>
    {{if .TunnelURL}}
    <form method="post" action="/tunnel/stop">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="subtle">Stop tunnel</button>
    </form>
    {{else}}
    {{/* Quick Tunnel (try.cloudflare.com): a public trycloudflare.com URL
         for the site, e.g. to hand an audience during a presentation. */}}
    <form method="post" action="/tunnel/start">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="subtle">Quick Tunnel…</button>
    </form>
    {{end}}
  </div>
  {{else if .Busy}}
  <p class="empty"><span class="spin"></span>Working — see progress below.</p>
  {{else}}
  <p class="empty">No demo site installed yet.</p>
  {{/* Restore backup works from empty because a backup records the recipe it
       came from — restoring can reinstall that code tree, then load the data.
       (The installed-state Restore only swaps data onto the current tree.) */}}
  <div class="row" style="margin:.9rem 0 0">
    <a href="/install"><button>Install a demo site…</button></a>
    <button class="subtle" disabled title="Coming soon">Restore backup…</button>
  </div>
  {{end}}
</section>
{{end}}`

// The accounts section sits between the site summary and the log — its own
// space to grow as teachers and students are seeded, and the home for the
// per-user login links. Deliberately NOT polled: it holds revealable passwords,
// and a periodic swap would re-mask one the moment you showed it.
// The accounts list refreshes only while a job runs and stops once idle — a
// bare wrapper div polls during install/reset so the section appears the moment
// the site is ready, then goes quiet so a revealed password is never re-masked
// out from under you. The card itself renders only when a site is installed.
const usersHTML = `{{define "users"}}
<div id="users"{{if .Busy}} hx-get="/section/users" hx-trigger="every 2s" hx-swap="outerHTML"{{end}}>
{{if .Installed}}
<section>
  <h2>Accounts</h2>
  <table>
    <tr><th>User</th><th>Password</th><th></th></tr>
    {{range .Users}}
    <tr>
      <td class="name">{{.Username}} <span class="role">{{.Role}}</span></td>
      <td>{{template "secret" .Password}}</td>
      <td><button class="subtle" hx-get="/sso/dialog?user={{.Username}}" hx-target="#ssobody"
          onclick="document.getElementById('ssobody').innerHTML='';document.getElementById('ssodialog').showModal()">Log in…</button></td>
    </tr>
    {{end}}
  </table>
  <div class="row" style="margin:.9rem 0 0">
    <button class="subtle" onclick="document.getElementById('createdialog').showModal()">Create user…</button>
  </div>
</section>
{{end}}
</div>
{{end}}`

const servicesHTML = `{{define "services"}}
<section id="services" hx-get="/section/services" hx-trigger="every 5s" hx-swap="outerHTML">
  <h2>Services</h2>
  <table>
    <tr><th>Service</th><th>Status</th></tr>
    {{range .Services}}
    <tr>
      <td class="name">{{.Name}}</td>
      <td><span class="badge {{if .Running}}on{{end}}">{{.Status}}</span></td>
    </tr>
    {{end}}
  </table>
</section>
{{end}}`

// help is an unmarked (headingless) section on the dashboard — the diagnostics
// pointer for now, and room for a note or two later.
const helpHTML = `{{define "help"}}
<section>
  <p class="empty">Something misbehaving? The <a href="/debug">diagnostics page</a>
     has a report you can copy into a bug report.</p>
</section>
{{end}}`

const debugHTML = `{{define "debug"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>— diagnostics</span></h1>
<p class="sub"><a href="/">← back</a></p></div></header>
{{template "services" .}}
<section>
  <p class="empty">Copy the whole block below into a bug report
     (<a href="https://github.com/mutms/mdl-demo/issues">github.com/mutms/mdl-demo/issues</a>).
     It contains service states and recent log lines, no passwords.</p>
  <pre class="log cred">{{.DebugReport}}</pre>
</section>
{{template "footer" .}}
</html>{{end}}`

// The progress section is split so the two halves refresh independently: the
// status header polls itself and swaps whole (it is tiny), while the log below
// it is never re-rendered wholesale — it only grows. jobstatus and logtail are
// both addressable fragments (/section/jobstatus and /joblog).
const progressHTML = `{{define "jobstatus"}}
<div id="jobstatus"{{if .Job.Running}} hx-get="/section/jobstatus" hx-trigger="every 2s" hx-swap="outerHTML"{{end}}>
  <h2>Progress log
      {{if .Job.Running}}<span class="spin"></span><span class="badge on">running</span>
      {{else if .Job.Failed}}<span class="badge err">failed</span>
      {{else}}<span class="badge on">done</span>{{end}}</h2>
  {{if .Job.Failed}}<p class="error">{{.Job.Error}}</p>{{end}}
</div>
{{end}}

{{/* logtail is both the initial log body and every incremental poll response:
     the batch of lines, then — while the job runs — a self-replacing cursor
     that fetches only what comes after .Job.Next and, on swap, scrolls the log
     box to the bottom. When the job ends the cursor is absent, so polling
     stops. */}}
{{define "logtail"}}{{range .Job.Log}}{{.}}
{{end}}{{if .Job.Running}}<span id="logcursor" hx-get="/joblog?from={{.Job.Next}}" hx-trigger="every 500ms" hx-target="this" hx-swap="outerHTML scroll:#joblog:bottom"></span>{{end}}{{end}}

{{define "progress"}}
{{if .Job.Kind}}
<section id="progress">
  {{template "jobstatus" .}}
  <pre id="joblog" class="log short">{{template "logtail" .}}</pre>
</section>
{{end}}
{{end}}`

// ssoHTML is the two-stage "Log in…" dialog: stage 1 offers a direct new-tab
// login plus a QR code; stage 2 shows a single-use QR and polls until the code
// is claimed, then closes the dialog — the presenter clicks Log in… → QR code…
// again for the next person, one fresh token each.
const ssoHTML = `{{define "ssodialog"}}
<p class="empty">Open the demo site as <code>{{.SSOUser}}</code> — in a new
   tab here, or on a phone via a single-use QR code.</p>
<div class="row">
  {{/* The new tab takes over; onsubmit closes the dialog behind it. */}}
  <form method="post" action="/sso/login" target="_blank"
        onsubmit="document.getElementById('ssodialog').close()">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <input type="hidden" name="user" value="{{.SSOUser}}">
    <button>Log in as {{.SSOUser}}</button>
  </form>
  <button class="subtle" hx-post="/sso/qr" hx-vals='{"csrf":"{{.CSRF}}","user":"{{.SSOUser}}"}' hx-target="#ssobody">QR code…</button>
</div>
{{end}}

{{define "ssoqr"}}
<p class="empty">Scan to log in as <code>{{.SSOUser}}</code>. Single use —
   this dialog closes once the code is claimed; open it again for the next
   person.</p>
<img class="ssoqr" src="{{.SSOQR}}" alt="Single-use login QR code">
{{template "ssopoll" .}}
{{end}}

{{define "ssopoll"}}<div id="ssopoll" hx-get="/sso/status?id={{.SSOTokenID}}" hx-trigger="every 2s" hx-swap="outerHTML"></div>{{end}}`

const loginHTML = `{{define "login"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>— log in</span></h1></div></header>
<section>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form class="stack" method="post" action="/login">
    <label>Management password
      <input type="password" name="password" autofocus required>
    </label>
    <div><button>Log in</button></div>
  </form>
</section>
{{template "footer" .}}
</html>{{end}}`

const setupHTML = `{{define "setup"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>— first-time setup</span></h1></div></header>
<section>
  <p class="empty">Set the management password for this container. (You can also
  provide it at container creation with <code>-e MDL_DEMO_PASSWORD=…</code>.)</p>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form class="stack" method="post" action="/setup">
    <label>New password
      <input type="password" name="password" minlength="8" autofocus required>
    </label>
    <label>Repeat password
      <input type="password" name="password2" minlength="8" required>
    </label>
    <div><button>Set password</button></div>
  </form>
</section>
{{template "footer" .}}
</html>{{end}}`

const installHTML = `{{define "install"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>— install a demo site</span></h1>
<p class="sub"><a href="/">← back</a></p></div></header>
<section>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form class="stack" method="post" action="/install">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <label>Site recipe
      <select name="recipe" required>
        {{range .Recipes}}<option value="{{.ID}}">{{.ID}} — {{.Name}}</option>
        {{end}}
      </select>
    </label>
    <label>Site name
      <input type="text" name="fullname" value="{{.Fullname}}" placeholder="the recipe's name" maxlength="254">
    </label>
    <label>Short name (shown in the navigation)
      <input type="text" name="shortname" value="{{.Shortname}}" maxlength="100">
    </label>
    <div><button>Install</button></div>
  </form>
  <p class="empty" style="margin-top:.9rem">A strong Moodle admin password is
  generated automatically and shown in the Accounts section once the site is
  ready. Installation
  clones several git repositories and runs the Moodle installer — expect several
  minutes.</p>
</section>
{{template "footer" .}}
</html>{{end}}`
