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
	shellHTML + siteHTML + servicesHTML + progressHTML +
		loginHTML + setupHTML + installHTML + debugHTML + footerHTML + copyHTML))

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
<title>mdl-demo</title>
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
  a { color: inherit; text-decoration-color: var(--line); }
  a:hover { text-decoration-color: currentColor; }
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
  .success { color: var(--ok); background: var(--okbg); border-radius: 7px;
    padding: .5rem .7rem; margin: 0 0 .7rem; }
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
  button.copy { background: none; border: 0; padding: 0 .1rem; margin-left: .15rem;
    color: var(--dim); cursor: pointer; vertical-align: -.15em; line-height: 1; }
  button.copy:hover { color: var(--fg); }
  button.copy.ok { color: var(--ok); }
</style>` + scriptHTML

// copyHTML is the copy-to-clipboard button; invoked as {{template "copy" <text>}}.
// The click handler lives in scriptHTML (delegated, so it survives htmx swaps).
const copyHTML = `{{define "copy"}}<button class="copy" type="button" data-copy="{{.}}" title="Copy to clipboard"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg></button>{{end}}`

// navigator.clipboard needs a secure context — http://localhost qualifies, but
// browsing via a LAN/VM IP does not, hence the hidden-textarea fallback.
const scriptHTML = `<script>
document.addEventListener('click', function (e) {
  var b = e.target.closest('button.copy');
  if (!b) return;
  var text = b.dataset.copy;
  var done = function () {
    var old = b.innerHTML;
    b.classList.add('ok');
    b.innerHTML = '✓';
    setTimeout(function () { b.classList.remove('ok'); b.innerHTML = old; }, 1200);
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
</script>`

const shellHTML = `{{define "page"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header>
  <div>
    <h1>mdl-demo <span>— Moodle demo manager</span></h1>
    <p class="sub">{{.Version}}</p>
  </div>
  <form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="subtle">Log out</button></form>
</header>

{{template "site" .}}
{{template "progress" .}}
{{template "services" .}}
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
    <dt>Log in as</dt><dd>admin / <span class="cred">{{if .AdminPass}}{{.AdminPass}}{{else}}(set at install){{end}}</span>{{if .AdminPass}}{{template "copy" .AdminPass}}{{end}}</dd>
    <dt>Installed</dt><dd>{{.InstalledAt}}</dd>
  </dl>
  {{/* Actions live in a row so future ones slot in beside Reset. Back up and
       Restore both act on data only (dataroot + database), leaving the git
       code tree in place — so they belong here, with a site installed. */}}
  <div class="row" style="margin:.9rem 0 0">
    <form method="post" action="/reset"
          onsubmit="return confirm('Wipe the demo site? The database, code tree and all data are deleted.')">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button class="subtle">Reset site…</button>
    </form>
    <button class="subtle" disabled title="Coming soon">Back up data…</button>
    <button class="subtle" disabled title="Coming soon">Restore data…</button>
  </div>
  {{else if .Busy}}
  <p class="empty"><span class="spin"></span>Working — see progress below.</p>
  {{else}}
  <p class="empty">No demo site installed yet.</p>
  <p style="margin:.9rem 0 0"><a href="/install"><button>Install a demo site…</button></a></p>
  {{end}}
</section>
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
  <p class="empty" style="margin:.7rem 0 0">Something misbehaving?
     The <a href="/debug">diagnostics page</a> has a report you can copy into a bug report.</p>
</section>
{{end}}`

const debugHTML = `{{define "debug"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>mdl-demo <span>— diagnostics</span></h1>
<p class="sub"><a href="/">← back</a></p></div></header>
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
  <h2>{{if eq .Job.Kind "install"}}Installation{{else}}Reset{{end}}
      {{if .Job.Running}}<span class="spin"></span><span class="badge on">running</span>
      {{else if .Job.Failed}}<span class="badge err">failed</span>
      {{else}}<span class="badge on">done</span>{{end}}</h2>
  {{if .Job.Failed}}<p class="error">{{.Job.Error}}</p>{{end}}
  {{if and (not .Job.Running) (not .Job.Failed) .Job.Wwwroot}}
  <p class="success">Demo site ready: <a href="{{.Job.Wwwroot}}" target="_blank" rel="noopener">{{.Job.Wwwroot}}</a> —
     log in as <code class="cred">admin</code> / <code class="cred">{{.Job.AdminPass}}</code>{{template "copy" .Job.AdminPass}}</p>
  {{end}}
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
  <pre id="joblog" class="log{{if not .Job.Running}} short{{end}}">{{template "logtail" .}}</pre>
</section>
{{end}}
{{end}}`

const loginHTML = `{{define "login"}}<!doctype html>
<html lang="en">
` + styleHTML + `
<header><div><h1>mdl-demo <span>— log in</span></h1></div></header>
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
<header><div><h1>mdl-demo <span>— first-time setup</span></h1></div></header>
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
<header><div><h1>mdl-demo <span>— install a demo site</span></h1>
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
    <label>Moodle admin password
      <input type="text" name="adminpass" minlength="8" value="{{.SuggestedPass}}" required>
    </label>
    <label>Site hostname (how your browser reaches this container)
      <input type="text" name="host" value="{{.Hostname}}" required>
    </label>
    <label>Moodle port (the host port you mapped to container port 8080)
      <input type="number" name="port" value="8080" min="1" max="65535" required>
    </label>
    <div><button>Install</button></div>
  </form>
  <p class="empty" style="margin-top:.9rem">Installation clones several git
  repositories and runs the Moodle installer — expect several minutes.</p>
</section>
{{template "footer" .}}
</html>{{end}}`
