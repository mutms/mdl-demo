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
		loginHTML + setupHTML + installHTML + debugHTML))

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
  a { color: inherit; text-decoration-color: var(--line); }
  a:hover { text-decoration-color: currentColor; }
  form.stack { display: grid; gap: .8rem; max-width: 26rem; }
  label { display: grid; gap: .25rem; font-size: .85rem; color: var(--dim); }
  input, select { font: inherit; color: var(--fg); background: var(--bg);
    border: 1px solid var(--line); border-radius: 7px; padding: .45rem .6rem; }
  button { font: inherit; font-weight: 600; border: 0; border-radius: 7px;
    padding: .5rem .9rem; background: var(--accent); color: #fff; cursor: pointer; }
  button.subtle { background: var(--idlebg); color: var(--fg); font-weight: 400; }
  .error { color: var(--err); background: var(--errbg); border-radius: 7px;
    padding: .5rem .7rem; font-size: .88rem; }
  pre.log { font: .78rem/1.45 ui-monospace, SFMono-Regular, monospace;
    background: var(--bg); border: 1px solid var(--line); border-radius: 7px;
    padding: .7rem .8rem; overflow-x: auto; white-space: pre-wrap; margin: 0; }
  .row { display: flex; gap: .6rem; align-items: center; }
</style>`

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

{{template "progress" .}}
{{template "site" .}}
{{template "services" .}}
</html>{{end}}`

const siteHTML = `{{define "site"}}
<section id="site" hx-get="/section/site" hx-trigger="every 5s" hx-swap="outerHTML">
  <h2>Demo site</h2>
  {{if .Installed}}
  <table>
    <tr><th>Recipe</th><th>URL</th><th>Admin</th><th>Installed</th></tr>
    <tr>
      <td class="name">{{.Recipe}}</td>
      {{/* New tab on purpose: landing inside Moodle in the same tab loses
           people — they forget the management UI's address to get back. */}}
      <td class="meta"><a href="{{.Wwwroot}}" target="_blank" rel="noopener">{{.Wwwroot}}</a></td>
      <td class="meta">admin</td>
      <td class="meta">{{.InstalledAt}}</td>
    </tr>
  </table>
  <form method="post" action="/reset" class="row" style="margin:.9rem 0 0"
        onsubmit="return confirm('Wipe the demo site? The database, code tree and all data are deleted.')">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <button class="subtle">Reset site…</button>
  </form>
  {{else if .Busy}}
  <p class="empty">Working — see progress above.</p>
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
</html>{{end}}`

const progressHTML = `{{define "progress"}}
{{if .Job.Kind}}
<section id="progress" {{if .Job.Running}}hx-get="/section/progress" hx-trigger="every 2s" hx-swap="outerHTML"{{end}}>
  <h2>{{if eq .Job.Kind "install"}}Installation{{else}}Reset{{end}}
      {{if .Job.Running}}<span class="badge on">running</span>
      {{else if .Job.Failed}}<span class="badge err">failed</span>
      {{else}}<span class="badge on">done</span>{{end}}</h2>
  {{if .Job.Failed}}<p class="error">{{.Job.Error}}</p>{{end}}
  {{if and (not .Job.Running) (not .Job.Failed) .Job.Wwwroot}}
  <p>Demo site ready: <a href="{{.Job.Wwwroot}}" target="_blank" rel="noopener">{{.Job.Wwwroot}}</a> —
     log in as <code class="cred">admin</code> / <code class="cred">{{.Job.AdminPass}}</code></p>
  {{end}}
  <pre class="log">{{range .Job.Tail}}{{.}}
{{end}}</pre>
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
</html>{{end}}`
