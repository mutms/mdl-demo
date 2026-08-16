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
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mutms/mdl-demo/internal/recipes"
	"github.com/mutms/mdl-demo/internal/site"
	"github.com/mutms/mdl-demo/internal/state"
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
	for _, name := range []string{"site", "services", "progress"} {
		section := name
		mux.HandleFunc("GET /section/"+section, s.auth(func(w http.ResponseWriter, r *http.Request) {
			s.renderFragment(w, r, section)
		}))
	}
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
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		if _, ok := s.sessions.get(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
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
	Job         jobView
	// Page-specific fields.
	Error         string
	Recipes       []recipes.Recipe
	Hostname      string
	SuggestedPass string
}

type serviceRow struct {
	Name    string
	Status  string
	Running bool
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
	}
	for _, unit := range []string{"apache2", "php8.3-fpm", "postgresql", "moodle-cron.timer"} {
		status := unitStatus(unit)
		v.Services = append(v.Services, serviceRow{
			Name:    unit,
			Status:  status,
			Running: status == "active",
		})
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
	v.Hostname = hostOnly(r.Host)
	// "Demo-…3!" keeps every Moodle default password-policy class present
	// (upper, lower, digit, symbol) whatever the random part contains.
	v.SuggestedPass = "Demo-" + randomToken()[:8] + "3!"
	s.render(w, "install", v)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.FormValue("port"))
	host := hostOnly(strings.TrimSpace(r.FormValue("host")))
	adminpass := r.FormValue("adminpass")
	if err != nil || port < 1 || port > 65535 || host == "" || len(adminpass) < 8 {
		http.Error(w, "invalid form values", http.StatusBadRequest)
		return
	}
	o := site.Options{
		Recipe:    r.FormValue("recipe"),
		AdminPass: adminpass,
		Wwwroot:   fmt.Sprintf("http://%s:%d", host, port),
	}
	if !s.job.startInstall(o) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

// unitStatus is `systemctl is-active`, which exits nonzero for anything but
// active — the output word (inactive/failed/…) is still what we want.
func unitStatus(unit string) string {
	out, err := execOutput("systemctl", "is-active", unit)
	if out == "" && err != nil {
		return "unknown"
	}
	return out
}

// execOutput keeps stdout even on nonzero exit (is-active exits 3 for
// "inactive" while still printing the word).
func execOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}
