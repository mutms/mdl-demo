// Package webui is the management UI on container port 8081 (the outside
// port is the demo's identity, see state.State.ConsolePort): a small
// password-protected dashboard (mpd-portal style — htmx fragments over
// html/template string constants) that installs and resets the demo site.
package webui

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mutms/mdl-demo/go/internal/recipes"
	"github.com/mutms/mdl-demo/go/internal/site"
	"github.com/mutms/mdl-demo/go/internal/sso"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/svc"
	"github.com/mutms/mdl-demo/go/internal/tunnel"
	qrcode "github.com/skip2/go-qrcode"
)

const addr = ":8081"

//go:embed static
var static embed.FS

type Server struct {
	version  string
	sessions *sessions
	job      *job
}

// Serve runs the UI until the process is stopped; the init supervisor owns
// the lifecycle. Before the listener opens, the container environment is
// adopted into state once: MDL_DEMO_PASSWORD (if no password is stored yet),
// MDL_DEMO_PORT and MDL_DEMO_NAME. Anything exec'd into the container after
// this point (the CLI, `mdl-demo url`) can rely on the identity being there.
func Serve(out io.Writer, version string) error {
	s := &Server{version: version, sessions: newSessions(), job: &job{}}

	if err := adoptEnv(out); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.auth(s.handleHome))
	for _, name := range []string{"site", "users", "services", "progress", "jobstatus"} {
		section := name
		mux.HandleFunc("GET /section/"+section, s.auth(func(w http.ResponseWriter, r *http.Request) {
			s.renderFragment(w, r, section)
		}))
	}
	mux.HandleFunc("GET /joblog", s.auth(s.handleJobLog))
	mux.HandleFunc("GET /debug", s.auth(s.handleDebug))
	mux.HandleFunc("GET /install", s.auth(s.handleInstallForm))
	mux.HandleFunc("POST /install", s.auth(s.csrf(s.handleInstall)))
	mux.HandleFunc("POST /reset", s.auth(s.csrf(s.handleReset)))
	mux.HandleFunc("POST /tunnel/start", s.auth(s.csrf(s.handleTunnelStart)))
	mux.HandleFunc("POST /tunnel/stop", s.auth(s.csrf(s.handleTunnelStop)))
	mux.HandleFunc("GET /tunnel/qr.png", s.auth(s.handleTunnelQR))
	mux.HandleFunc("GET /sso/dialog", s.auth(s.handleSSODialog))
	mux.HandleFunc("POST /sso/login", s.auth(s.csrf(s.handleSSOLogin)))
	mux.HandleFunc("POST /sso/qr", s.auth(s.csrf(s.handleSSOQR)))
	mux.HandleFunc("GET /sso/status", s.auth(s.handleSSOStatus))
	mux.HandleFunc("POST /logout", s.auth(s.csrf(s.handleLogout)))
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /setup", s.handleSetupForm)
	mux.HandleFunc("POST /setup", s.handleSetup)

	assets, err := fs.Sub(static, "static")
	if err != nil {
		return err
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(out, "mdl-demo web UI listening on %s\n", addr)
	return server.ListenAndServe()
}

func adoptEnv(out io.Writer) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	changed := false
	if pw := state.ContainerEnv("MDL_DEMO_PASSWORD"); pw != "" && st.PasswordHash == "" {
		hash, err := hashPassword(pw)
		if err != nil {
			return err
		}
		st.PasswordHash = hash
		changed = true
		fmt.Fprintln(out, "adopted management password from MDL_DEMO_PASSWORD")
	}
	warn := func(line string) { fmt.Fprintln(out, "warning: "+line) }
	if st.AdoptIdentity(state.ContainerEnv("MDL_DEMO_PORT"), state.ContainerEnv("MDL_DEMO_NAME"), warn) {
		changed = true
		fmt.Fprintf(out, "demo identity: %s (console port %d, site port %d)\n", st.Title(), st.Port(), st.SitePort())
	}
	if changed {
		return st.Save()
	}
	return nil
}

// auth gates a handler behind the password: unset password → forced setup,
// no session → login.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := state.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if st.PasswordHash == "" {
			redirect(w, r, "/setup")
			return
		}
		if _, ok := s.sessions.get(r); !ok {
			redirect(w, r, "/login")
			return
		}
		next(w, r)
	}
}

// redirect sends the browser to url. For a background htmx request (a section
// poll, say) an ordinary 303 is wrong: htmx follows it and swaps the whole
// target page into the little fragment slot, so the login/setup page ends up
// nested and repeated. The HX-Redirect header instead makes htmx navigate the
// whole window — which is what a poll that just discovered "you must log in /
// set a password" should do. This is exactly the case a container rebuild hits:
// the fresh container has no password, and the open page's polls must land on
// the setup page, not paint it inside themselves.
func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// csrf verifies the per-session token and the request origin on
// state-changing requests. Runs inside auth, so a session exists.
func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.sessions.get(r)
		if !ok || !sameOriginOK(r) || r.FormValue("csrf") != sess.csrf {
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

type view struct {
	Version string
	// ID, Name and Title identify this demo (see state.State): every page
	// shows them so several consoles open side by side stay apart.
	ID          string
	Name        string
	Title       string
	CSRF        string
	Installed   bool
	Busy        bool
	Recipe      string
	Wwwroot     string
	TunnelURL   string
	InstalledAt string
	Services    []serviceRow
	Users       []userRow
	Job         jobView
	// Page-specific fields.
	Error       string
	Recipes     []recipes.Recipe
	Fullname    string
	Shortname   string
	DebugReport string
	// SSO dialog fields (the single-use "Log in…" flow). SSOQR is a data:
	// image URL — template.URL so html/template's URL sanitizer keeps it.
	SSOUser    string
	SSOTokenID string
	SSOQR      template.URL
}

type serviceRow struct {
	Name    string
	Status  string
	Running bool
}

// userRow is one Moodle account shown in the accounts section — its plaintext
// password comes straight from state (the container owns it for a throwaway
// site). Only admin exists today; seeded teachers/students will join the list.
type userRow struct {
	Username string
	Password string
	Role     string
}

// baseView is the view every page starts from, logged in or not: version
// and the demo identity.
func (s *Server) baseView() view {
	v := view{Version: s.version}
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	v.ID, v.Name, v.Title = st.ID(), st.Name, st.Title()
	return v
}

func (s *Server) buildView(r *http.Request) view {
	v := s.baseView()
	v.Job, v.Busy = s.job.view(), !s.job.idle()
	if sess, ok := s.sessions.get(r); ok {
		v.CSRF = sess.csrf
	}
	if st, err := state.Load(); err == nil && st.Installed() {
		v.Installed = true
		v.Recipe = st.Recipe
		v.Wwwroot = st.Wwwroot
		v.InstalledAt = st.InstalledAt.Format("2006-01-02 15:04 MST")
		v.Users = demoUsers(st)
		v.TunnelURL = tunnel.URL()
	}
	for _, s := range svc.Current().Statuses() {
		v.Services = append(v.Services, serviceRow{Name: s.Name, Status: s.State, Running: s.Running})
	}
	return v
}

func (s *Server) render(w http.ResponseWriter, name string, v view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.ExecuteTemplate(w, name, v); err != nil {
		fmt.Fprintf(w, "<!-- render error: %v -->", err)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, "page", s.buildView(r))
}

func (s *Server) renderFragment(w http.ResponseWriter, r *http.Request, section string) {
	s.render(w, section, s.buildView(r))
}

// handleJobLog streams the log incrementally: given the caller's last line
// number (?from=N), it renders only the lines after it plus a fresh cursor, so
// the browser appends rather than reflowing the whole log. The "logtail"
// template ends in a self-replacing poller carrying the new line number.
func (s *Server) handleJobLog(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	s.render(w, "logtail", view{Job: s.job.logSince(from)})
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login", s.baseView())
}

// viewError is a bare page view carrying an error message.
func (s *Server) viewError(msg string) view {
	v := s.baseView()
	v.Error = msg
	return v
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOriginOK(r) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	if s.sessions.throttled(r) {
		s.render(w, "login", s.viewError("Too many failed attempts — try again later."))
		return
	}
	st, err := state.Load()
	if err != nil || st.PasswordHash == "" || !verifyPassword(st.PasswordHash, r.FormValue("password")) {
		s.sessions.recordFail(r)
		s.render(w, "login", s.viewError("Wrong password."))
		return
	}
	s.sessions.start(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSetupForm(w http.ResponseWriter, r *http.Request) {
	st, err := state.Load()
	if err == nil && st.PasswordHash != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "setup", s.baseView())
}

// handleSetup sets the initial password. Only possible while none is set —
// changing it later means recreating the container (or exec + state edit).
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !sameOriginOK(r) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	st, err := state.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st.PasswordHash != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	pw, pw2 := r.FormValue("password"), r.FormValue("password2")
	if len(pw) < 8 {
		s.render(w, "setup", s.viewError("Password must be at least 8 characters."))
		return
	}
	if pw != pw2 {
		s.render(w, "setup", s.viewError("Passwords do not match."))
		return
	}
	hash, err := hashPassword(pw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st.PasswordHash = hash
	if err := st.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.sessions.start(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleInstallForm(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	if v.Installed || v.Busy {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	list, err := recipes.List()
	if err != nil {
		v.Error = err.Error()
	}
	v.Recipes = list
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	v.Fullname = st.Name
	v.Shortname = st.Name
	if v.Shortname == "" {
		v.Shortname = "demo"
	}
	s.render(w, "install", v)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	// Never asked for: the `mdl-demo url` override when set, else the console's
	// host on port+1.
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	wwwroot, err := site.NormalizeURL(st.SiteURLFor(hostOnly(r.Host)))
	if err != nil {
		http.Error(w, "invalid demo site URL", http.StatusBadRequest)
		return
	}
	o := site.Options{
		Recipe:    r.FormValue("recipe"),
		AdminPass: randomAdminPass(),
		Wwwroot:   wwwroot,
		Fullname:  strings.TrimSpace(r.FormValue("fullname")),
		Shortname: strings.TrimSpace(r.FormValue("shortname")),
	}
	if !s.job.startInstall(o) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// demoUsers is the one list behind the Accounts card and the SSO login flow:
// only names returned here may receive a login token.
func demoUsers(st *state.State) []userRow {
	if !st.Installed() {
		return nil
	}
	return []userRow{{Username: "admin", Password: st.AdminPass, Role: "Site admin"}}
}

func validDemoUser(name string) bool {
	st, err := state.Load()
	if err != nil {
		return false
	}
	for _, u := range demoUsers(st) {
		if u.Username == name {
			return true
		}
	}
	return false
}

// siteBase is the site URL login links must work under: the tunnel when
// active (a phone cannot reach localhost), else the override/derived one.
func siteBase(r *http.Request) string {
	if u := tunnel.URL(); u != "" {
		return u
	}
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	return st.SiteURLFor(hostOnly(r.Host))
}

func (s *Server) handleSSODialog(w http.ResponseWriter, r *http.Request) {
	user := r.FormValue("user")
	if !validDemoUser(user) {
		http.Error(w, "unknown demo user", http.StatusBadRequest)
		return
	}
	v := s.buildView(r)
	v.SSOUser = user
	s.render(w, "ssodialog", v)
}

func (s *Server) handleSSOLogin(w http.ResponseWriter, r *http.Request) {
	user := r.FormValue("user")
	if !validDemoUser(user) {
		http.Error(w, "unknown demo user", http.StatusBadRequest)
		return
	}
	token, _, err := sso.Mint(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, siteBase(r)+"/mdldemo-login.php?token="+token, http.StatusSeeOther)
}

func (s *Server) handleSSOQR(w http.ResponseWriter, r *http.Request) {
	user := r.FormValue("user")
	if !validDemoUser(user) {
		http.Error(w, "unknown demo user", http.StatusBadRequest)
		return
	}
	token, id, err := sso.Mint(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	png, err := qrcode.Encode(siteBase(r)+"/mdldemo-login.php?token="+token, qrcode.Medium, 512)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v := s.buildView(r)
	v.SSOUser, v.SSOTokenID = user, id
	v.SSOQR = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
	s.render(w, "ssoqr", v)
}

var ssoIDRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (s *Server) handleSSOStatus(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if !ssoIDRe.MatchString(id) {
		http.Error(w, "bad token id", http.StatusBadRequest)
		return
	}
	if sso.Pending(id) {
		v := s.buildView(r)
		v.SSOTokenID = id
		s.render(w, "ssopoll", v)
		return
	}
	// Claimed (or expired): swap in a script that closes the dialog — htmx
	// executes scripts in swapped content.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<script>document.getElementById('ssodialog').close()</script>`)
}

// The tunnel handlers run synchronously: cloudflared announces its URL in a
// few seconds, so a plain form POST + redirect beats wiring them into the
// single-flight job. The captured log tail makes a failure diagnosable.
func (s *Server) handleTunnelStart(w http.ResponseWriter, r *http.Request) {
	st, err := state.Load()
	if err != nil || !st.Installed() {
		redirect(w, r, "/")
		return
	}
	var tail []string
	logf := func(line string) {
		tail = append(tail, line)
		if len(tail) > 30 {
			tail = tail[1:]
		}
	}
	if _, err := tunnel.Start(logf); err != nil {
		http.Error(w, "starting the tunnel failed: "+err.Error()+"\n\n"+strings.Join(tail, "\n"), http.StatusBadGateway)
		return
	}
	redirect(w, r, "/")
}

func (s *Server) handleTunnelStop(w http.ResponseWriter, r *http.Request) {
	if err := tunnel.Stop(func(string) {}); err != nil {
		http.Error(w, "stopping the tunnel failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, "/")
}

func (s *Server) handleTunnelQR(w http.ResponseWriter, r *http.Request) {
	u := tunnel.URL()
	if u == "" {
		http.NotFound(w, r)
		return
	}
	png, err := qrcode.Encode(u, qrcode.Medium, 512)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// randomAdminPass generates the Moodle admin password. It is generated, never
// asked for: a demo that may be exposed to the internet must not hang on
// someone choosing a strong password. The "Demo-…3!" shape keeps every default
// policy class present (upper, lower, digit, symbol) whatever the random middle
// is; the result is stored (plain text, for a throwaway site) and shown masked
// so the operator can retrieve it later without it ever being typed.
func randomAdminPass() string {
	return "Demo-" + randomToken()[:8] + "3!"
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if !s.job.startReset() {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.end(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleDebug renders the diagnostics page: one copy-pasteable report of
// mode, versions, state and per-service status + log tails, so an end user
// can paste it whole into a bug report.
func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	var b strings.Builder
	fmt.Fprintf(&b, "mdl-demo %s\nmode: %s\ntime: %s\n",
		s.version, svc.Current().Mode(), time.Now().UTC().Format(time.RFC3339))
	if st, err := state.Load(); err == nil {
		fmt.Fprintf(&b, "demo: %s (console port %d, site port %d; inside %d/%d)\n",
			st.Title(), st.Port(), st.SitePort(), state.ConsoleListen, state.SiteListen)
		host := hostOnly(r.Host)
		fmt.Fprintf(&b, "urls: console %s, site %s", st.ConsoleURLFor(host), st.SiteURLFor(host))
		if st.ConsoleURL != "" || st.SiteURL != "" {
			b.WriteString(" (overridden with `mdl-demo url`)")
		}
		b.WriteString("\n")
	}
	if v.Installed {
		fmt.Fprintf(&b, "site: %s (%s), installed %s\n", v.Recipe, v.Wwwroot, v.InstalledAt)
	} else {
		b.WriteString("site: none installed\n")
	}
	if v.Job.Kind != "" {
		fmt.Fprintf(&b, "last operation: %s running=%v failed=%v %s\n",
			v.Job.Kind, v.Job.Running, v.Job.Failed, v.Job.Error)
	}
	for _, d := range svc.Current().Diagnostics() {
		fmt.Fprintf(&b, "\n== %s: %s", d.Name, d.State)
		if d.PID > 0 {
			fmt.Fprintf(&b, " pid=%d", d.PID)
		}
		if d.Restarts > 0 {
			fmt.Fprintf(&b, " restarts=%d last-exit=%q", d.Restarts, d.LastExit)
		}
		b.WriteString(" ==\n")
		for _, line := range d.LogTail {
			b.WriteString(line + "\n")
		}
	}
	v.DebugReport = b.String()
	s.render(w, "debug", v)
}

func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}
