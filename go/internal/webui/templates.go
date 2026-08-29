package webui

// Templates live as Go string constants rather than embedded .html files so
// a section and the view type that feeds it can be read side by side (same
// convention as mpd's portal, whose look this UI follows). Every value is
// escaped by html/template; nothing here interpolates raw.
//
// User-facing strings render through {{t .Lang "…"}} (tr in lang.go): the
// English text is the catalog key and its own fallback.

import "html/template"

// page holds the whole template set: the shell plus one named template per
// section/page. Sections are addressable on their own — that is what htmx
// fetches to refresh one card without touching the rest.
var page = template.Must(template.New("page").Funcs(template.FuncMap{"t": tr}).Parse(
	shellHTML + siteHTML + usersHTML + mailHTML + servicesHTML + helpHTML + progressHTML +
		loginHTML + setupHTML + installHTML + debugHTML + footerHTML + copyHTML + secretHTML + ssoHTML + topctlHTML))

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

// topctlHTML is the header's right-side controls, on every page including
// login/setup: the language switcher (cookie via GET /lang) and the theme
// toggle cycling auto → light → dark (localStorage, handler in scriptHTML).
const topctlHTML = `{{define "topctl"}}<div class="row topctl">
  <nav class="langs">
    <a href="/lang?set=en"{{if eq .Lang "en"}} class="cur"{{end}}>EN</a><a href="/lang?set=cs"{{if eq .Lang "cs"}} class="cur"{{end}}>CS</a><a href="/lang?set=de"{{if eq .Lang "de"}} class="cur"{{end}}>DE</a>
  </nav>
  <button id="themebtn" type="button" title="{{t .Lang "Theme"}}">◐</button>
</div>{{end}}`

// The inline theme snippet runs before the body parses, so the chosen theme
// is stamped on <html> before first paint — no light flash for dark users.
const styleHTML = `<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<script>try{var _t=localStorage.getItem('mdl-demo-theme');if(_t==='light'||_t==='dark')document.documentElement.dataset.theme=_t}catch(e){}</script>
<script src="/static/htmx.min.js"></script>
{{/* Pico is the base layer; the <style> below is the design and wins every
     conflict. Pico's html[data-theme] contract matches the theme toggle. */}}
<link rel="stylesheet" href="/static/pico.min.css">
<style>
  /* Pico owns the design system; this sheet is the theme plus the components
     Pico does not have. Identity colors map onto Pico's variables, so Pico's
     own light/dark switching (matching the data-theme toggle) drives them —
     only the status palette needs a dark override of its own. */
  :root {
    --fg: var(--pico-color); --dim: var(--pico-muted-color);
    --line: var(--pico-muted-border-color);
    --card: var(--pico-card-background-color); --bg: var(--pico-background-color);
    --accent: var(--pico-primary);
    --ok: #15803d; --okbg: #16a34a1f; --idle: #6b7280; --idlebg: #9ca3af22;
    --err: #b91c1c; --errbg: #dc26261a;
    --pico-border-radius: 8px;
  }
  @media (prefers-color-scheme: dark) {
    :root:not([data-theme="light"]) {
      --ok: #4ade80; --okbg: #16a34a26; --err: #f87171; --errbg: #dc262626; }
  }
  :root[data-theme="dark"] {
      --ok: #4ade80; --okbg: #16a34a26; --err: #f87171; --errbg: #dc262626; }
  html { font-size: 93.75%; }
  body { margin: 0 auto; padding: 2rem 1.5rem 4rem; max-width: 52rem; }
  header { margin-bottom: 2rem; display: flex; justify-content: space-between; align-items: baseline; }
  h1 { font-size: 1.35rem; margin: 0; letter-spacing: -.01em; }
  h1 span { color: var(--dim); font-weight: 400; }
  .sub { color: var(--dim); margin: .2rem 0 0; font-size: .85rem; }
  h2 { font-size: .78rem; text-transform: uppercase; letter-spacing: .07em;
       color: var(--dim); margin: 0 0 .75rem; font-weight: 600; }
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
  form.stack { max-width: 26rem; }
  {{/* Pico makes buttons full-width form blocks with generous padding; this
       console uses them inline in rows, tables and headers, and quieter. */}}
  button { width: auto; margin-bottom: 0; padding: .4rem .85rem; }
  .error { color: var(--err); background: var(--errbg); border-radius: var(--pico-border-radius);
    padding: .5rem .7rem; font-size: .88rem; }
  pre.log { font: .78rem/1.45 ui-monospace, SFMono-Regular, monospace;
    background: var(--bg); border: 1px solid var(--line); border-radius: 7px;
    padding: .7rem .8rem; overflow: auto; white-space: pre-wrap; margin: 0;
    max-height: 22rem; }
  pre.log.short { max-height: 5cm; }
  .row { display: flex; gap: .6rem; align-items: center; }
  .topctl { gap: .45rem; }
  .langs a { font-size: .75rem; color: var(--dim); text-decoration: none;
    padding: .15rem .3rem; border-radius: 5px; }
  .langs a.cur { background: var(--idlebg); color: var(--fg); font-weight: 600; }
  #themebtn { background: none; border: 0; color: var(--dim); font-size: 1rem;
    cursor: pointer; padding: .15rem .3rem; line-height: 1; }
  #themebtn:hover { color: var(--fg); }
  .spin { display: inline-block; width: .85em; height: .85em; margin-right: .35em;
    border: 2px solid var(--line); border-top-color: var(--accent);
    border-radius: 50%; vertical-align: -.1em;
    animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  button.copy, button.reveal, button.qr { background: none; border: 0; padding: 0 .1rem;
    margin-left: .15rem; color: var(--dim); cursor: pointer;
    vertical-align: -.15em; line-height: 1; }
  button.copy:hover, button.reveal:hover, button.qr:hover { color: var(--fg); }
  dialog > article { width: min(30rem, 92vw); min-height: 8rem;
    position: relative; /* anchors .dlgclose */ }
  dialog#qrdialog article { background: #fff; width: auto; min-height: 0; }
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
// Theme toggle: auto → light → dark. Auto = no data-theme, no stored value.
document.addEventListener('click', function (e) {
  var b = e.target.closest('#themebtn');
  if (!b) return;
  var cur = document.documentElement.dataset.theme || 'auto';
  var next = cur === 'auto' ? 'light' : cur === 'light' ? 'dark' : 'auto';
  try {
    if (next === 'auto') localStorage.removeItem('mdl-demo-theme');
    else localStorage.setItem('mdl-demo-theme', next);
  } catch (err) {}
  if (next === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = next;
  b.textContent = next === 'auto' ? '◐' : next === 'light' ? '☀' : '☾';
});
document.addEventListener('DOMContentLoaded', function () {
  var b = document.getElementById('themebtn');
  if (!b) return;
  var t = document.documentElement.dataset.theme || 'auto';
  b.textContent = t === 'light' ? '☀' : t === 'dark' ? '☾' : '◐';
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
<html lang="{{.Lang}}">
` + styleHTML + `
<header>
  <div>
    <h1>{{.ID}} <span>— {{if .Name}}{{.Name}}{{else}}{{t .Lang "Moodle demo console"}}{{end}}</span></h1>
    <p class="sub">{{.Version}}</p>
  </div>
  <div class="row">
    {{template "topctl" .}}
    <form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="secondary">{{t .Lang "Log out"}}</button></form>
  </div>
</header>

{{template "site" .}}
{{template "users" .}}
{{template "mail" .}}
{{template "progress" .}}
{{template "help" .}}
{{/* Pico's modal shape: the dialog is the full-screen overlay, the box is a
     child <article> (which also gives it the card look for free). */}}
<dialog id="qrdialog" onclick="this.close()"><article><button class="dlgclose" type="button" aria-label="Close">×</button><img alt="QR code for the tunnel URL"></article></dialog>
{{/* The close button sits outside #ssobody so htmx stage swaps keep it. */}}
<dialog id="ssodialog" onclick="if(event.target===this)this.close()"><article><button class="dlgclose" type="button" aria-label="Close">×</button><div id="ssobody"></div></article></dialog>
{{/* Unconditional, unlike the button that opens it (the Accounts card):
     htmx swaps that card in the moment an install finishes, and a dialog
     rendered only {{if .Installed}} would not exist yet on a page opened
     before the install — the button would target null until a reload. */}}
<dialog id="createdialog" onclick="if(event.target===this)this.close()">
  <article>
  <button class="dlgclose" type="button" aria-label="Close">×</button>
  {{/* The password is never asked for: generated like the admin's, shown
       masked on the Accounts card — the whole point of console-side users. */}}
  <form class="stack" method="post" action="/users/create">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <label>{{t .Lang "Username"}}
      <input name="username" required pattern="[a-z0-9._-]+" title="{{t .Lang "lowercase letters, digits, . _ -"}}">
    </label>
    <label>{{t .Lang "First name"}} <input name="firstname" required maxlength="100"></label>
    <label>{{t .Lang "Last name"}} <input name="lastname" required maxlength="100"></label>
    <label>{{t .Lang "Global role"}}
      <select name="role">
        <option value="">{{t .Lang "None (plain user)"}}</option>
        <option value="manager">{{t .Lang "Manager"}}</option>
        <option value="admin">{{t .Lang "Administrator"}}</option>
      </select>
    </label>
    <div><button>{{t .Lang "Create user"}}</button></div>
  </form>
  </article>
</dialog>
{{template "footer" .}}
</html>{{end}}`

const siteHTML = `{{define "site"}}
<article id="site" hx-get="/section/site" hx-trigger="every 5s" hx-swap="outerHTML">
  <h2>{{t .Lang "Demo site"}}</h2>
  {{if .Installed}}
  {{/* One site per container, so its details are a description list, not a
       one-row table. */}}
  <dl class="site">
    <dt>{{t .Lang "Recipe"}}</dt><dd class="name">{{.Recipe}}</dd>
    {{/* New tab on purpose: landing inside Moodle in the same tab loses
         people — they forget the management UI's address to get back. */}}
    {{/* While the tunnel runs, the tunnel URL IS the site's URL (wwwroot is
         rewritten to it); showing the local one here would send clicks on a
         301-hop through the original host — and nowhere at all off-LAN. */}}
    {{if .TunnelURL}}
    <dt>{{t .Lang "URL"}}</dt><dd><a href="{{.TunnelURL}}" target="_blank" rel="noopener">{{.TunnelURL}}</a><button class="qr" type="button" title="QR" aria-label="QR"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3z"/><path d="M21 14v.01M14 21v.01M21 21v.01M17.5 17.5v.01"/></svg></button> <span class="badge on">{{t .Lang "tunnel"}}</span></dd>
    {{else}}
    <dt>{{t .Lang "URL"}}</dt><dd><a href="{{.Wwwroot}}" target="_blank" rel="noopener">{{.Wwwroot}}</a></dd>
    {{end}}
    <dt>{{t .Lang "Installed"}}</dt><dd>{{.InstalledAt}}</dd>
  </dl>
  {{/* Actions live in a row so future ones slot in beside Reset. Restore backup
       appears in both states: here (site installed) it swaps data onto the
       current tree; from empty it reinstalls the backup's recipe first. */}}
  <div class="row" style="margin:.9rem 0 0">
    <form method="post" action="/reset"
          onsubmit="return confirm('{{t .Lang "Wipe the demo site? The database, code tree and all data are deleted."}}')">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="secondary">{{t .Lang "Reset site…"}}</button>
    </form>
    <button class="secondary" disabled title="{{t .Lang "Coming soon"}}">{{t .Lang "Back up data…"}}</button>
    <button class="secondary" disabled title="{{t .Lang "Coming soon"}}">{{t .Lang "Restore backup…"}}</button>
    {{if .TunnelURL}}
    <form method="post" action="/tunnel/stop">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="secondary">{{t .Lang "Stop tunnel"}}</button>
    </form>
    {{else}}
    {{/* Quick Tunnel (try.cloudflare.com): a public trycloudflare.com URL
         for the site, e.g. to hand an audience during a presentation. */}}
    <form method="post" action="/tunnel/start">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="secondary">{{t .Lang "Quick Tunnel…"}}</button>
    </form>
    {{end}}
  </div>
  {{else if .Busy}}
  <p class="empty"><span class="spin"></span>{{t .Lang "Working — see progress below."}}</p>
  {{else}}
  <p class="empty">{{t .Lang "No demo site installed yet."}}</p>
  {{/* Restore backup works from empty because a backup records the recipe it
       came from — restoring can reinstall that code tree, then load the data.
       (The installed-state Restore only swaps data onto the current tree.) */}}
  <div class="row" style="margin:.9rem 0 0">
    <a href="/install"><button>{{t .Lang "Install a demo site…"}}</button></a>
    <button class="secondary" disabled title="{{t .Lang "Coming soon"}}">{{t .Lang "Restore backup…"}}</button>
  </div>
  {{end}}
</article>
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
<article>
  <h2>{{t .Lang "Accounts"}}</h2>
  <table>
    <tr><th>{{t .Lang "User"}}</th><th>{{t .Lang "Password"}}</th><th></th></tr>
    {{range .Users}}
    <tr>
      <td class="name">{{.Username}} <span class="role">{{t $.Lang .Role}}</span></td>
      <td>{{template "secret" .Password}}</td>
      <td><button class="secondary" hx-get="/sso/dialog?user={{.Username}}" hx-target="#ssobody"
          onclick="document.getElementById('ssobody').innerHTML='';document.getElementById('ssodialog').showModal()">{{t $.Lang "Log in…"}}</button></td>
    </tr>
    {{end}}
  </table>
  <div class="row" style="margin:.9rem 0 0">
    <button class="secondary" onclick="document.getElementById('createdialog').showModal()">{{t .Lang "Create user…"}}</button>
  </div>
</article>
{{end}}
</div>
{{end}}`

const servicesHTML = `{{define "services"}}
<article id="services" hx-get="/section/services" hx-trigger="every 5s" hx-swap="outerHTML">
  <h2>{{t .Lang "Services"}}</h2>
  <table>
    <tr><th>{{t .Lang "Service"}}</th><th>{{t .Lang "Status"}}</th></tr>
    {{range .Services}}
    <tr>
      <td class="name">{{.Name}}</td>
      <td><span class="badge {{if .Running}}on{{end}}">{{.Status}}</span></td>
    </tr>
    {{end}}
  </table>
</article>
{{end}}`

// The mail card links the Mailpit catcher (proxied under /mail): the demo
// site's entire outbox, for showing an audience what Moodle sends.
const mailHTML = `{{define "mail"}}
{{if .Installed}}
<article>
  <h2>{{t .Lang "Mail"}}</h2>
  <p class="empty">{{t .Lang "Everything the demo site sends lands here — no mail ever leaves the container."}}</p>
  <div class="row" style="margin:.9rem 0 0">
    <a href="/mail/" target="_blank" rel="noopener"><button class="secondary">{{t .Lang "Open the mail catcher…"}}</button></a>
  </div>
</article>
{{end}}
{{end}}`

// help is an unmarked (headingless) section on the dashboard — the diagnostics
// pointer for now, and room for a note or two later.
const helpHTML = `{{define "help"}}
<article>
  <p class="empty">{{t .Lang "Something misbehaving?"}} <a href="/debug">{{t .Lang "The diagnostics page"}}</a>
     {{t .Lang "has a report you can copy into a bug report."}}</p>
</article>
{{end}}`

const debugHTML = `{{define "debug"}}<!doctype html>
<html lang="{{.Lang}}">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>{{t .Lang "— diagnostics"}}</span></h1>
<p class="sub"><a href="/">{{t .Lang "← back"}}</a></p></div>{{template "topctl" .}}</header>
{{template "services" .}}
<article>
  <p class="empty">{{t .Lang "Copy the whole block below into a bug report"}}
     (<a href="https://github.com/mutms/mdl-demo/issues">github.com/mutms/mdl-demo/issues</a>).
     {{t .Lang "It contains service states and recent log lines, no passwords."}}</p>
  <pre class="log cred">{{.DebugReport}}</pre>
</article>
{{template "footer" .}}
</html>{{end}}`

// The progress section is split so the two halves refresh independently: the
// status header polls itself and swaps whole (it is tiny), while the log below
// it is never re-rendered wholesale — it only grows. jobstatus and logtail are
// both addressable fragments (/section/jobstatus and /joblog).
const progressHTML = `{{define "jobstatus"}}
<div id="jobstatus"{{if .Job.Running}} hx-get="/section/jobstatus" hx-trigger="every 2s" hx-swap="outerHTML"{{end}}>
  <h2>{{t .Lang "Progress log"}}
      {{if .Job.Running}}<span class="spin"></span><span class="badge on">{{t .Lang "running"}}</span>
      {{else if .Job.Failed}}<span class="badge err">{{t .Lang "failed"}}</span>
      {{else}}<span class="badge on">{{t .Lang "done"}}</span>{{end}}</h2>
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
<article id="progress">
  {{template "jobstatus" .}}
  <pre id="joblog" class="log short">{{template "logtail" .}}</pre>
</article>
{{end}}
{{end}}`

// ssoHTML is the two-stage "Log in…" dialog: stage 1 offers a direct new-tab
// login plus a QR code; stage 2 shows a single-use QR and polls until the code
// is claimed, then closes the dialog — the presenter clicks Log in… → QR code…
// again for the next person, one fresh token each.
const ssoHTML = `{{define "ssodialog"}}
<p class="empty">{{t .Lang "Open the demo site as"}} <code>{{.SSOUser}}</code>
   {{t .Lang "— in a new tab here, or on a phone via a single-use QR code."}}</p>
<div class="row">
  {{/* The new tab takes over; onsubmit closes the dialog behind it. */}}
  <form method="post" action="/sso/login" target="_blank"
        onsubmit="document.getElementById('ssodialog').close()">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <input type="hidden" name="user" value="{{.SSOUser}}">
    <button>{{t .Lang "Log in as"}} {{.SSOUser}}</button>
  </form>
  <button class="secondary" hx-post="/sso/qr" hx-vals='{"csrf":"{{.CSRF}}","user":"{{.SSOUser}}"}' hx-target="#ssobody">{{t .Lang "QR code…"}}</button>
</div>
{{end}}

{{define "ssoqr"}}
<p class="empty">{{t .Lang "Scan to log in as"}} <code>{{.SSOUser}}</code>.
   {{t .Lang "Single use — this dialog closes once the code is claimed; open it again for the next person."}}</p>
<img class="ssoqr" src="{{.SSOQR}}" alt="QR">
{{template "ssopoll" .}}
{{end}}

{{define "ssopoll"}}<div id="ssopoll" hx-get="/sso/status?id={{.SSOTokenID}}" hx-trigger="every 2s" hx-swap="outerHTML"></div>{{end}}`

const loginHTML = `{{define "login"}}<!doctype html>
<html lang="{{.Lang}}">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>{{t .Lang "— log in"}}</span></h1></div>{{template "topctl" .}}</header>
<article>
  {{if .Error}}<p class="error">{{t .Lang .Error}}</p>{{end}}
  <form class="stack" method="post" action="/login">
    <label>{{t .Lang "Management password"}}
      <input type="password" name="password" autofocus required>
    </label>
    <div><button>{{t .Lang "Log in"}}</button></div>
  </form>
</article>
{{template "footer" .}}
</html>{{end}}`

const setupHTML = `{{define "setup"}}<!doctype html>
<html lang="{{.Lang}}">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>{{t .Lang "— first-time setup"}}</span></h1></div>{{template "topctl" .}}</header>
<article>
  <p class="empty">{{t .Lang "Set the management password for this container."}}
  {{t .Lang "(You can also provide it at container creation with"}} <code>-e MDL_DEMO_PASSWORD=…</code>.)</p>
  {{if .Error}}<p class="error">{{t .Lang .Error}}</p>{{end}}
  <form class="stack" method="post" action="/setup">
    <label>{{t .Lang "New password"}}
      <input type="password" name="password" minlength="8" autofocus required>
    </label>
    <label>{{t .Lang "Repeat password"}}
      <input type="password" name="password2" minlength="8" required>
    </label>
    <div><button>{{t .Lang "Set password"}}</button></div>
  </form>
</article>
{{template "footer" .}}
</html>{{end}}`

const installHTML = `{{define "install"}}<!doctype html>
<html lang="{{.Lang}}">
` + styleHTML + `
<header><div><h1>{{.ID}}{{if .Name}} · {{.Name}}{{end}} <span>{{t .Lang "— install a demo site"}}</span></h1>
<p class="sub"><a href="/">{{t .Lang "← back"}}</a></p></div>{{template "topctl" .}}</header>
<article>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form class="stack" method="post" action="/install">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <label>{{t .Lang "Site recipe"}}
      <select name="recipe" required>
        {{range .Recipes}}<option value="{{.ID}}">{{.ID}} — {{.Name}}</option>
        {{end}}
      </select>
    </label>
    <label>{{t .Lang "Site name"}}
      <input type="text" name="fullname" value="{{.Fullname}}" placeholder="{{t .Lang "the recipe's name"}}" maxlength="254">
    </label>
    <label>{{t .Lang "Short name (shown in the navigation)"}}
      <input type="text" name="shortname" value="{{.Shortname}}" maxlength="100">
    </label>
    <div><button>{{t .Lang "Install"}}</button></div>
  </form>
  <p class="empty" style="margin-top:.9rem">{{t .Lang "A strong Moodle admin password is generated automatically and shown in the Accounts section once the site is ready. Installation clones several git repositories and runs the Moodle installer — expect several minutes."}}</p>
</article>
{{template "footer" .}}
</html>{{end}}`
