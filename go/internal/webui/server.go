// Package webui is the management UI on container port 8081: a small
// password-protected dashboard (mpd-portal style — htmx fragments over
// html/template string constants) that installs and resets the demo site.
package webui

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mutms/mdl-demo/internal/recipes"
	"github.com/mutms/mdl-demo/internal/site"
	"github.com/mutms/mdl-demo/internal/state"
	"github.com/mutms/mdl-demo/internal/svc"
)

const addr = ":8081"

//go:embed static
var static embed.FS

type Server struct {
	version  string
	sessions *sessions
	job      *job
}

// Serve runs the UI until the process is stopped; systemd owns the
// lifecycle. If MDL_DEMO_PASSWORD was passed at container creation and no
// password is stored yet, it is adopted now.
func Serve(out io.Writer, version string) error {
	s := &Server{version: version, sessions: newSessions(), job: &job{}}

	if pw := passwordFromPID1(); pw != "" {
		st, err := state.Load()
		if err != nil {
			return err
		}
		if st.PasswordHash == "" {
			hash, err := hashPassword(pw)
			if err != nil {
				return err
			}
			st.PasswordHash = hash
			if err := st.Save(); err != nil {
				return err
			}
			fmt.Fprintln(out, "adopted management password from MDL_DEMO_PASSWORD")
		}
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
	Version     string
	CSRF        string
	Installed   bool
	Busy        bool
	Recipe      string
	Wwwroot     string
	InstalledAt string
	Services    []serviceRow
	Users       []userRow
	Job         jobView
	// Page-specific fields.
	Error        string
	Recipes      []recipes.Recipe
	SuggestedURL string
	DebugReport  string
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

func (s *Server) buildView(r *http.Request) view {
	v := view{Version: s.version, Job: s.job.view(), Busy: !s.job.idle()}
	if sess, ok := s.sessions.get(r); ok {
		v.CSRF = sess.csrf
	}
	if st, err := state.Load(); err == nil && st.Installed() {
		v.Installed = true
		v.Recipe = st.Recipe
		v.Wwwroot = st.Wwwroot
		v.InstalledAt = st.InstalledAt.Format("2006-01-02 15:04 MST")
		v.Users = []userRow{{Username: "admin", Password: st.AdminPass, Role: "Site admin"}}
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
	s.render(w, "login", view{Version: s.version})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOriginOK(r) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	if s.sessions.throttled(r) {
		s.render(w, "login", view{Error: "Too many failed attempts — try again later."})
		return
	}
	st, err := state.Load()
	if err != nil || st.PasswordHash == "" || !verifyPassword(st.PasswordHash, r.FormValue("password")) {
		s.sessions.recordFail(r)
		s.render(w, "login", view{Error: "Wrong password."})
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
	s.render(w, "setup", view{Version: s.version})
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
		s.render(w, "setup", view{Error: "Password must be at least 8 characters."})
		return
	}
	if pw != pw2 {
		s.render(w, "setup", view{Error: "Passwords do not match."})
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
	// Suggest the whole URL, not a host + a port field: "port" is jargon a
	// teacher should never meet. The console is on 8081, the site on the host
	// port mapped to 8080; if that mapping differs (or a tunnel is used), the
	// operator just edits the address.
	v.SuggestedURL = "http://" + hostOnly(r.Host) + ":8080"
	s.render(w, "install", v)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	wwwroot, err := normalizeWwwroot(r.FormValue("wwwroot"))
	if err != nil {
		http.Error(w, "invalid demo site URL", http.StatusBadRequest)
		return
	}
	o := site.Options{
		Recipe:    r.FormValue("recipe"),
		AdminPass: randomAdminPass(),
		Wwwroot:   wwwroot,
	}
	if !s.job.startInstall(o) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

// normalizeWwwroot validates the demo site URL the operator entered and returns
// the wwwroot Moodle bakes in: an http(s) URL with a host and no trailing slash
// ($CFG->wwwroot is stored without one). A path is kept — Moodle can live under
// a subpath — but the empty/garbage cases are rejected.
func normalizeWwwroot(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("not an http(s) URL")
	}
	return strings.TrimRight(u.String(), "/"), nil
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
