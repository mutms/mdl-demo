package webui

import (
	"net/http"

	"github.com/coder/websocket"

	"github.com/mutms/mdl-demo/go/internal/term"
)

// cliEnabled reports whether the CLI console is available (off when the
// custom-image env MDL_DEMO_NO_CLI is set — a locked-down fork drops the tile,
// the page, the socket and the tools install).
// A variable so tests can turn it off: it reads PID 1's environment.
var cliEnabled = func() bool { return !boolEnv("MDL_DEMO_NO_CLI") }

// handleCLIPage is the full-window terminal, opened in its own window from the
// Tools card. Unlike every other page it holds no /events stream: nothing here
// refreshes, and a closed socket offers a reload.
func (s *Server) handleCLIPage(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	v := s.baseView(r)
	v.CSRF = csrfToken(r)
	s.render(w, "cli", v)
}

// handleCLISocket attaches the browser to the shell. A WebSocket upgrade is a
// GET, so s.csrf does not cover it, yet it is the most powerful route here:
// cross-site WebSocket hijacking is real, and SameSite is not the only thing
// to rely on. Origin and token are checked before the upgrade, on top of the
// Host allow-list in guard and the library's own origin check. As everywhere in
// the console these are browser-side checks (see the header comment in auth.go).
func (s *Server) handleCLISocket(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	if !sameOriginOK(r) || !csrfMatches(r, r.URL.Query().Get("csrf")) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept has written the error
	}
	defer conn.CloseNow()
	if err := term.Attach(r.Context(), conn); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}
}

// handleCLIInstallTools starts the job that installs Chromium and Node. It has
// no button: the claude-install helper posts here from the terminal, because
// www-data has no other way to run apt.
func (s *Server) handleCLIInstallTools(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	if !s.job.start("tools", "", "", term.InstallTools) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
