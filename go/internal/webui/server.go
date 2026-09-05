// Package webui is the management UI on container port 8081 (the outside
// port is the demo's identity, see state.State.ConsolePort): a small
// dashboard (mpd-portal style — htmx fragments over html/template files)
// that installs and resets the demo site. auth.go holds the checks that
// guard it.
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
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mutms/mdl-demo/go/internal/backup"
	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
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
	version string
	job     *job
	// epoch identifies this process instance. A page embeds it and a tiny
	// poller re-checks it; when it differs the process restarted (a rebuilt
	// container, say) and the open page — now showing stale state — reloads.
	epoch string
	// poster is the optional image-baked custom Tools card, loaded once at boot;
	// nil in the stock image (see poster.go).
	poster *poster
}

// Serve runs the UI until the process is stopped; the init supervisor owns
// the lifecycle. Before the listener opens, the container environment is
// adopted into state once: MDL_DEMO_PORT and MDL_DEMO_NAME. Anything exec'd
// into the container after this point (the CLI, `mdl-demo url`) can rely on
// the identity being there.
func Serve(out io.Writer, version string) error {
	s := &Server{version: version, job: &job{}, epoch: strconv.FormatInt(time.Now().UnixNano(), 36)}
	s.poster = loadPoster()
	logSink.Store(s.job)

	if err := adoptEnv(out); err != nil {
		return err
	}

	// No route is behind a login — there is none. s.guard wraps the whole mux
	// below (host check + CSRF cookie); s.csrf gates the state-changing ones.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleHome)
	for _, name := range []string{"site", "install", "users", "tools", "progress", "jobstatus", "statuspill"} {
		section := name
		mux.HandleFunc("GET /section/"+section, func(w http.ResponseWriter, r *http.Request) {
			s.renderFragment(w, r, section)
		})
	}
	mux.HandleFunc("GET /joblog", s.handleJobLog)
	mux.HandleFunc("GET /settings", s.handleSettingsPage)
	// /debug was the old diagnostics URL; it now lives on the Settings page.
	mux.HandleFunc("GET /debug", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
	})
	mux.HandleFunc("POST /install", s.csrf(s.handleInstall))
	mux.HandleFunc("POST /reset", s.csrf(s.handleReset))
	mux.HandleFunc("POST /tunnel/start", s.csrf(s.handleTunnelStart))
	mux.HandleFunc("POST /tunnel/stop", s.csrf(s.handleTunnelStop))
	mux.HandleFunc("GET /tunnel/qr.png", s.handleTunnelQR)
	mux.HandleFunc("POST /users/create", s.csrf(s.handleUserCreate))
	mux.HandleFunc("POST /settings/update-catalogues", s.csrf(s.handleSettingsUpdate))
	mux.HandleFunc("GET /plugins", s.handlePluginsPage)
	mux.HandleFunc("POST /plugins/refs", s.csrf(s.handlePluginRefs))
	mux.HandleFunc("POST /plugins/add", s.csrf(s.handlePluginAdd))
	mux.HandleFunc("GET /recommends", s.handleRecommendsPage)
	mux.HandleFunc("GET /poster", s.handlePosterPage)
	mux.HandleFunc("GET /backups", s.handleBackupsPage)
	mux.HandleFunc("GET /section/backuplist", s.handleBackupList)
	mux.HandleFunc("POST /backups/create", s.csrf(s.handleBackupCreate))
	mux.HandleFunc("POST /backups/restore", s.csrf(s.handleBackupRestore))
	mux.HandleFunc("POST /backups/delete", s.csrf(s.handleBackupDelete))
	mux.HandleFunc("GET /backups/download", s.handleBackupDownload)
	// Upload is NOT behind s.csrf: its FormValue call would spool the whole
	// multipart body (gigabytes) to a temp copy before the handler ever runs.
	// The handler streams instead and checks origin + csrf itself.
	mux.HandleFunc("POST /backups/upload", s.handleBackupUpload)
	mux.HandleFunc("GET /sso/dialog", s.handleSSODialog)
	mux.HandleFunc("POST /sso/login", s.csrf(s.handleSSOLogin))
	mux.HandleFunc("POST /sso/qr", s.csrf(s.handleSSOQR))
	mux.HandleFunc("GET /sso/status", s.handleSSOStatus)
	mux.HandleFunc("GET /lang", s.handleLang)
	mux.HandleFunc("GET /alive", s.handleAlive)

	// Mailpit's UI (all the site's outgoing mail) proxied under /mail — the
	// presenter's tool, reachable on the same terms as the rest of the
	// console. The proxy passes Mailpit's live-update WebSocket through as-is.
	mailpit := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "127.0.0.1:8025"})
	mux.Handle("/mail/", mailpit)
	mux.HandleFunc("GET /mail", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/mail/", http.StatusMovedPermanently)
	})

	assets, err := fs.Sub(static, "static")
	if err != nil {
		return err
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))

	server := &http.Server{
		Addr:              addr,
		Handler:           secureHeaders(s.guard(mux)),
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

// redirect sends the browser to url. For an htmx request an ordinary 303 is
// wrong: htmx follows it and swaps the whole target page into the little
// fragment slot, leaving a complete page nested inside one card. The
// HX-Redirect header instead makes htmx navigate the whole window, which is
// what an action ending in "go look at the dashboard" wants.
func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

type view struct {
	Version string
	// ID, Name and Title identify this demo (see state.State): every page
	// shows them so several consoles open side by side stay apart.
	ID    string
	Name  string
	Title string
	Lang  string
	// StateSig captures the coarse state the page was rendered for (process
	// instance + installed/busy/recipe). The page's watcher reloads when it
	// changes — a reset (web or CLI), an install finishing, a rebuilt container.
	StateSig string
	// Snapshot marks a point-in-time page (the diagnostics report) that must
	// hold still — it opts out of the live-reload watcher.
	Snapshot bool
	// Path is the current request path, so the language switcher can return
	// here (the console sends no Referer).
	Path      string
	CSRF      string
	Installed bool
	Busy      bool
	Recipe    string
	// MoodleRelease is the installed Moodle version, e.g. "4.5.13 (20250109)".
	MoodleRelease  string
	SiteName       string
	Wwwroot        string
	TunnelURL      string
	TunnelStarting bool
	// TunnelEnabled is false when MDL_DEMO_NO_TUNNEL is set — a custom/offline
	// image that hides the Quick Tunnel card and disables its routes.
	TunnelEnabled bool
	InstalledAt   string
	// ServiceProblems is the supervised services that are NOT running — shown
	// (only when non-empty) so the diag page flags trouble instead of listing
	// everything that is fine. cloudflared's normal "no tunnel" is not a problem.
	ServiceProblems []serviceRow
	Users           []userRow
	Job             jobView
	// Section is a sub-page's label (a translation key like "Backups"); empty on
	// the dashboard. The sticky top bar uses it to switch the brand from a plain
	// identity into a "← back to console" link that also names the sub-page.
	Section string
	// Page-specific fields.
	// Error is a page-level failure message (the backups listing not being
	// readable, say) — not to be confused with Job.Error, which reports the
	// background install/restore.
	Error string
	// Chooser name fields (prefilled defaults for the one-click install).
	Fullname     string
	Shortname    string
	DebugReport  string
	RecipeExport string
	// SSO dialog fields (the single-use "Log in…" flow). SSOQR is a data:
	// image URL — template.URL so html/template's URL sanitizer keeps it.
	SSOUser    string
	SSOTokenID string
	SSOQR      template.URL
	// Backups page.
	Backups []backupRow
	// BackupName is the Backup dialog's prefilled default name, without the .mdb
	// extension (added on save).
	BackupName string
	// Plugins page: additional plugins bucketed by type.
	PluginGroups []pluginGroup
	// Add-a-plugin: the repo URL under consideration and its refs (/plugins/refs).
	PluginURL string
	Refs      site.Refs
	// Recommendations page.
	Recommends []recommendRow
	// The empty dashboard's recipe chooser: vendor tabs of version streams.
	VendorTabs []vendorTab
	// Settings page: result of a catalogue git pull, rendered back into it.
	CatUpdates []catUpdate
	// Poster is the optional image-baked custom Tools card (nil in the stock image).
	Poster *poster
}

// catUpdate is one catalogue's git-pull result for the Settings page.
type catUpdate struct {
	Name string
	Msg  string
	OK   bool
}

// vendorTab is one tab of the install chooser — all of a vendor's streams.
// Active marks the tab shown first.
type vendorTab struct {
	Vendor  string
	Label   string
	Active  bool
	Streams []recipeGroup
}

// recipeGroup is one vendor/stream group inside a tab. Current holds the newest
// version of each series (5.2.x, 5.1.x, …) — outdated point releases are
// derived, never curated: they fold into Older automatically the moment a newer
// one lands in the catalogue. Label/Desc are the (translatable) stream copy;
// Total is the badge count.
type recipeGroup struct {
	Vendor, Stream string
	Label, Desc    string
	Total          int
	Current, Older []recipes.Recipe
}

// streamCopy carries the chooser's stream display text (vendor labels come from
// recipes.VendorLabel). English strings are translation keys (see lang.go); an
// unlisted vendor/stream shows its slug.
type streamText struct{ Label, Desc string }

var streamCopy = map[string]streamText{
	"moodle/release": {"Release", "Plain Moodle — core only, no plugins."},
	"moodle/dev":     {"Development", "Plain Moodle, still in development — a look at what's coming."},
	"mutms/release":  {"Full suite", "Patched Moodle core with multi-tenancy and every MuTMS plugin."},
	"mutms/moodle":   {"On plain Moodle", "All MuTMS plugins on plain Moodle core — no multi-tenancy."},
	"mutms/dev":      {"Development", "The full MuTMS suite in active development, on the latest stable Moodle."},
	"iomad/dev":      {"Development", "IOMAD, a multi-tenant Moodle distribution — its current stable branches."},
}

// recommendRow is one entry on the maintainer's recommendations page. The list
// is deliberately curated and personal (recommends()), clearly the maintainer's
// own view — and forkable: a fork replaces it with its own picks.
type recommendRow struct{ Name, URL, Blurb string }

func recommends() []recommendRow {
	return []recommendRow{
		{"MuTMS", "https://mutms.org", "My suite of open-source plugins adding multi-tenancy, programs, certifications and other improvements to Moodle."},
		{"mudev", "https://github.com/mutms/mudev", "Wires multiple plugin git repos into a Moodle tree — for dev work, CI and test environments."},
		{"mpd", "https://github.com/mutms/mpd", "Self-contained Moodle development VMs and tooling used for MuTMS development."},
		{"Camp Registry", "https://camp-registry.org/", "An open, community catalogue of Moodle plugins."},
		{"mdlshield", "https://mdlshield.com", "Security scanning and hardening for Moodle."},
	}
}

// vendorRank orders the chooser's tabs: the three known vendors first in this
// order, then any other vendor after them (alphabetically, via stable sort).
func vendorRank(vendor string) int {
	switch vendor {
	case "moodle":
		return 0
	case "mutms":
		return 1
	case "iomad":
		return 2
	default:
		return 3
	}
}

// streamRank orders streams within a tab: release first, then the plugins-only
// stream, then development, then anything else.
func streamRank(stream string) int {
	switch stream {
	case "release":
		return 0
	case "moodle":
		return 1
	case "dev":
		return 2
	default:
		return 3
	}
}

// series is the version's maintenance branch: "5.2.2" → "5.2", and so is
// a deeper build like MuTMS's "5.2.2.01". Only point releases fold — a
// two-part version like a dev stream's "5.3" IS the branch, so it is its
// own series and never hides behind a sibling.
func series(version string) string {
	if parts := strings.Split(version, "."); len(parts) >= 3 {
		return parts[0] + "." + parts[1]
	}
	return version
}

// groupRecipes folds the sorted catalogue list into vendor tabs, each holding
// its streams (release first), each stream split into current-per-series vs
// older versions. The first vendor's tab starts active.
func groupRecipes(list []recipes.Recipe) []vendorTab {
	// One recipeGroup per vendor/stream, in catalogue (vendor,stream asc) order.
	var groups []recipeGroup
	seen := map[string]bool{}
	for _, rec := range list {
		if n := len(groups); n == 0 || groups[n-1].Vendor != rec.Vendor || groups[n-1].Stream != rec.Stream {
			c := streamCopy[rec.Vendor+"/"+rec.Stream]
			label := c.Label
			if label == "" {
				label = rec.Stream
			}
			groups = append(groups, recipeGroup{Vendor: rec.Vendor, Stream: rec.Stream, Label: label, Desc: c.Desc})
			clear(seen)
		}
		g := &groups[len(groups)-1]
		g.Total++
		// Versions sort newest-first within a group, so the first of a series
		// is its newest.
		if s := series(rec.Version); !seen[s] {
			seen[s] = true
			g.Current = append(g.Current, rec)
		} else {
			g.Older = append(g.Older, rec)
		}
	}
	// Fold streams into vendor tabs, preserving vendor order of first sighting.
	var tabs []vendorTab
	idx := map[string]int{}
	for _, g := range groups {
		i, ok := idx[g.Vendor]
		if !ok {
			i = len(tabs)
			idx[g.Vendor] = i
			tabs = append(tabs, vendorTab{Vendor: g.Vendor, Label: recipes.VendorLabel(g.Vendor)})
		}
		tabs[i].Streams = append(tabs[i].Streams, g)
	}
	for i := range tabs {
		slices.SortStableFunc(tabs[i].Streams, func(a, b recipeGroup) int {
			return streamRank(a.Stream) - streamRank(b.Stream)
		})
	}
	// Known vendors first (Moodle, MuTMS, IOMAD); any others after, kept in the
	// catalogue's alphabetical order by the stable sort.
	slices.SortStableFunc(tabs, func(a, b vendorTab) int {
		return vendorRank(a.Vendor) - vendorRank(b.Vendor)
	})
	if len(tabs) > 0 {
		tabs[0].Active = true
	}
	return tabs
}

type backupRow struct {
	Name    string
	Size    string
	Created string
	Recipe  string
	// Restorable is false for a file that is not a readable .mdb archive
	// (a foreign upload, say): it can be downloaded or deleted, not restored.
	Restorable bool
	// SameCode is true when this backup's recipe matches the code tree already
	// on disk (equal recipe hashes) — the restore dialog then defaults to the
	// fast "keep current code" path. Only set for an installed, idle site.
	SameCode bool
}

type serviceRow struct {
	Name    string
	Status  string
	Running bool
}

// pluginRow is one additional plugin on the Plugins page: its frankenstyle
// component, the localized name, where it sits in the tree, and its version.
type pluginRow struct {
	Component   string
	Type        string
	DisplayName string
	Relpath     string
	// Version is Moodle's numeric version (the YYYYMMDDXX code date, always
	// present); Release is the human tag like "5.0.9.01" (often missing).
	Version string
	Release string
	// SourceURL is the plugin's repository on GitHub/GitLab, "" when unknown.
	SourceURL string
}

// pluginGroup is one plugin-type's plugins, for the type-grouped Plugins page.
type pluginGroup struct {
	Type    string
	Label   string
	Plugins []pluginRow
}

// pluginTypeLabels give Moodle's frankenstyle plugin-type slugs a friendly name;
// an unlisted type falls back to its slug (see pluginTypeLabel). English only —
// these are secondary labels on a technical page.
var pluginTypeLabels = map[string]string{
	"mod": "Activity modules", "block": "Blocks", "tool": "Admin tools",
	"local": "Local plugins", "enrol": "Enrolment methods", "auth": "Authentication",
	"theme": "Themes", "format": "Course formats", "report": "Reports",
	"qtype": "Question types", "qbank": "Question bank", "qbehaviour": "Question behaviours",
	"filter": "Filters", "editor": "Editors", "repository": "Repositories",
	"portfolio": "Portfolios", "webservice": "Web services", "gradingform": "Grading methods",
	"gradereport": "Grade reports", "gradeexport": "Grade exports", "gradeimport": "Grade imports",
	"availability": "Availability conditions", "customfield": "Custom fields",
	"dataformat": "Data formats", "profilefield": "Profile fields", "antivirus": "Antivirus",
	"search": "Search engines", "media": "Media players", "plagiarism": "Plagiarism",
	"contenttype": "Content bank", "fileconverter": "Document converters",
	"assignsubmission": "Assignment submissions", "assignfeedback": "Assignment feedback",
	"quizaccess": "Quiz access rules", "atto": "Atto editor", "tiny": "TinyMCE editor",
	"certificateelement": "Certificate elements", "cachestore": "Cache stores",
}

func pluginTypeLabel(t string) string {
	if l, ok := pluginTypeLabels[t]; ok {
		return l
	}
	return t
}

// userRow is one Moodle account shown in the accounts section — its plaintext
// password comes straight from state (the container owns it for a throwaway
// site). Only admin exists today; seeded teachers/students will join the list.
type userRow struct {
	Username string
	Password string
	Role     string
}

// baseView is the view every page starts from, logged in or not: version,
// the demo identity, and the display language.
func (s *Server) baseView(r *http.Request) view {
	v := view{Version: s.version, Lang: requestLang(r), Path: r.URL.Path}
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	v.ID, v.Name, v.Title = st.ID(), st.Name, st.Title()
	// Recording/screenshot mode (MDL_DEMO_HIDE_PORT): drop the "-NNNN" port from
	// the shown identity so it reads "mdl-demo". A boot-time container setting,
	// not a per-viewer toggle, so it is right on the very first paint — no flash
	// of the port. The port stays the demo's real identity everywhere else
	// (container name, URLs, the ID the CLI prints).
	if boolEnv("MDL_DEMO_HIDE_PORT") {
		v.ID = strings.TrimSuffix(v.ID, "-"+strconv.Itoa(st.Port()))
		v.Title = v.ID
		if v.Name != "" {
			v.Title = v.ID + " · " + v.Name
		}
	}
	v.Poster = s.poster
	return v
}

// boolEnv reads a container env var as a boolean (1/true/yes/on, case-folded).
func boolEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(state.ContainerEnv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// tunnelEnabled reports whether the Quick Tunnel is available (off when the
// custom-image env MDL_DEMO_NO_TUNNEL is set).
func tunnelEnabled() bool { return !boolEnv("MDL_DEMO_NO_TUNNEL") }

func (s *Server) buildView(r *http.Request) view {
	v := s.baseView(r)
	v.Job, v.Busy = s.job.view(), !s.job.idle()
	v.CSRF = csrfToken(r)
	v.TunnelEnabled = tunnelEnabled()
	if st, err := state.Load(); err == nil && st.Installed() {
		v.Installed = true
		v.Recipe = st.Recipe
		v.MoodleRelease, _ = moodle.Version()
		v.SiteName = st.Fullname
		v.Wwwroot = st.Wwwroot
		v.InstalledAt = st.InstalledAt.Format("2006-01-02 15:04 MST")
		v.Users = demoUsers(st)
		v.TunnelURL = tunnel.URL()
		v.TunnelStarting = tunnel.Starting()
	} else {
		// The empty dashboard IS the install chooser. Restoring a backup is the
		// other way to fill an empty demo, but that lives on the Backups page
		// (Tools card) so it doesn't crowd the chooser or push Tools down.
		list, _ := recipes.List()
		v.VendorTabs = groupRecipes(list)
		if err == nil {
			v.Fullname = st.Name
			v.Shortname = st.Name
		}
		if v.Shortname == "" {
			v.Shortname = "demo"
		}
	}
	// Flag only the supervised services that are down; a healthy one needs no
	// mention, and cloudflared is on-demand (its status is the report's tunnel
	// line), not a fault.
	for _, s := range svc.Current().Statuses() {
		if !s.Running {
			v.ServiceProblems = append(v.ServiceProblems, serviceRow{Name: s.Name, Status: s.State})
		}
	}
	v.StateSig = sigString(s.epoch, v.Installed, v.Busy, v.Recipe)
	return v
}

// sigString is the coarse page-state signature the liveness watcher compares
// (see handleAlive). Deliberately excludes tunnel state — that changes through
// smooth section swaps and should not force a full reload.
func sigString(epoch string, installed, busy bool, recipe string) string {
	return fmt.Sprintf("%s|%t|%t|%s", epoch, installed, busy, recipe)
}

// stateSig recomputes the current signature from live state, for handleAlive to
// compare against the one the page was rendered with.
func (s *Server) stateSig() string {
	installed, recipe := false, ""
	if st, err := state.Load(); err == nil && st.Installed() {
		installed, recipe = true, st.Recipe
	}
	return sigString(s.epoch, installed, !s.job.idle(), recipe)
}

func (s *Server) render(w http.ResponseWriter, name string, v view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := page.ExecuteTemplate(w, name, v); err != nil {
		fmt.Fprintf(w, "<!-- render error: %v -->", err)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, "page", s.buildView(r))
}

// handleAlive is the open-page liveness check: the page polls it with the state
// signature it was rendered against; a mismatch means the coarse state changed
// under it — a reset (web or CLI), an install/backup finishing, or a rebuilt
// container (new process) — so the page is stale and wants reloading.
//
// It fires a "stale-page" event rather than htmx's own HX-Refresh: app.js holds
// the reload back while a modal <dialog> is open, so a job finishing (busy flips
// in the signature) can never yank a dialog out from under the user mid-action.
func (s *Server) handleAlive(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("s") != s.stateSig() {
		w.Header().Set("HX-Trigger", "stale-page")
	}
	// 200 with an empty body (hx-swap="none" changes nothing); htmx dispatches
	// the HX-Trigger event on a 200.
	w.WriteHeader(http.StatusOK)
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
		AdminPass: site.RandomPassword(),
		Wwwroot:   wwwroot,
		Fullname:  strings.TrimSpace(r.FormValue("fullname")),
		Shortname: strings.TrimSpace(r.FormValue("shortname")),
		Lang:      requestLang(r),
	}
	// Resolve the "<Vendor> Demo" default now (not inside site.Install) so the
	// busy card can show the site's name while it installs. site.Install still
	// defaults a blank name too, for the CLI path.
	if o.Fullname == "" && o.Recipe != "" {
		o.Fullname = site.DefaultFullname(strings.SplitN(o.Recipe, "/", 2)[0])
	}
	if !s.job.startInstall(o) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// backupRows shapes the backups listing for the template. Reading each file's
// metadata is cheap by format contract: meta.json is always the first tar
// entry.
// backupRows shapes the listing. currentHash, when non-empty, is the installed
// tree's recipe hash (site.CurrentRecipeHash): a restorable backup whose recipe
// hashes equal it is flagged SameCode, so the dialog can default to keeping the
// code. Empty currentHash (not installed, or busy) leaves every row SameCode
// false — the safe default.
func backupRows(currentHash string) ([]backupRow, error) {
	list, err := backup.List()
	if err != nil {
		return nil, err
	}
	var rows []backupRow
	for _, b := range list {
		row := backupRow{
			Name:    b.Name,
			Size:    fmt.Sprintf("%.1f MB", float64(b.Size)/1e6),
			Created: b.ModTime.Format("2006-01-02 15:04"),
		}
		if b.Meta != nil {
			row.Recipe = b.Meta.Recipe
			row.Restorable = true
			if currentHash != "" {
				if p, err := backup.Path(b.Name); err == nil {
					if h, err := backup.RecipeHash(p); err == nil {
						row.SameCode = h == currentHash
					}
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Server) backupsView(r *http.Request) view {
	v := s.buildView(r)
	// For an installed, idle site, hash the current tree's recipe once so the
	// listing can flag which backups match it (fast "keep current code"
	// restore). Skipped while busy — the tree may be mid-rebuild — and when
	// empty, where there is nothing to keep.
	var currentHash string
	if v.Installed && !v.Busy {
		currentHash, _ = site.CurrentRecipeHash()
	}
	rows, err := backupRows(currentHash)
	if err != nil {
		v.Error = err.Error()
	}
	v.Backups = rows
	if v.Installed {
		v.BackupName = strings.TrimSuffix(backup.SuggestName(v.SiteName, time.Now()), ".mdb")
	}
	v.Section = "Backups"
	return v
}

func (s *Server) handleBackupsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "backups", s.backupsView(r))
}

func (s *Server) handleBackupList(w http.ResponseWriter, r *http.Request) {
	s.render(w, "backuplist", s.backupsView(r))
}

// pluginRows lists the site's additional plugins for the Plugins page.
func pluginRows() ([]pluginRow, error) {
	plugins, err := moodle.ExportPlugins()
	if err != nil {
		return nil, err
	}
	// Source repos come from the tree's live recipe; a plugin (or subplugin)
	// links to the checkout whose relpath is the longest prefix of its own.
	sources, _ := moodle.PluginSources()
	rows := make([]pluginRow, 0, len(plugins))
	for _, p := range plugins {
		rows = append(rows, pluginRow{
			Component:   p.Component,
			Type:        p.Type,
			DisplayName: p.DisplayName,
			Relpath:     p.Relpath,
			Version:     p.VersionDisk.String(),
			Release:     p.Release,
			SourceURL:   sourceURLFor(p.Relpath, sources),
		})
	}
	// core_plugin_manager's order is the upgrade sequence, not anything a reader
	// cares about; sort by tree path so related plugins (and subplugins) sit
	// together predictably.
	slices.SortFunc(rows, func(a, b pluginRow) int {
		return strings.Compare(a.Relpath, b.Relpath)
	})
	return rows, nil
}

// groupPlugins buckets the rows by plugin type into groups ordered by their
// friendly label, so the Plugins page reads as "Activity modules … / Blocks …"
// instead of one long technical table.
func groupPlugins(rows []pluginRow) []pluginGroup {
	byType := map[string][]pluginRow{}
	for _, r := range rows {
		byType[r.Type] = append(byType[r.Type], r)
	}
	groups := make([]pluginGroup, 0, len(byType))
	for t, ps := range byType {
		groups = append(groups, pluginGroup{Type: t, Label: pluginTypeLabel(t), Plugins: ps})
	}
	// Order the type groups by where they live in the tree (each group's plugins
	// are already relpath-sorted, so Plugins[0] is the group's first path), so a
	// nested type sits under its parent — certificate elements after admin tools,
	// not alphabetically adrift.
	slices.SortFunc(groups, func(a, b pluginGroup) int {
		return strings.Compare(a.Plugins[0].Relpath, b.Plugins[0].Relpath)
	})
	return groups
}

// sourceURLFor returns the URL of the checkout whose relpath is the longest
// prefix of the plugin's relpath — so a subplugin nested inside a repo (e.g.
// certificateelement_* under tool_certificate) links to that repo.
func sourceURLFor(relpath string, sources []moodle.PluginSource) string {
	best := ""
	bestLen := -1
	for _, s := range sources {
		if (relpath == s.Relpath || strings.HasPrefix(relpath, s.Relpath+"/")) && len(s.Relpath) > bestLen {
			best, bestLen = s.URL, len(s.Relpath)
		}
	}
	return best
}

func (s *Server) pluginsView(r *http.Request) view {
	v := s.buildView(r)
	if v.Installed {
		rows, err := pluginRows()
		if err != nil {
			v.Error = err.Error()
		}
		v.PluginGroups = groupPlugins(rows)
	}
	v.Section = "Installed plugins"
	return v
}

func (s *Server) handlePluginsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "plugins", s.pluginsView(r))
}

// handlePluginRefs reads a git repo's branches/tags (with a proposed default)
// and swaps the ref picker into the Add-a-plugin form.
func (s *Server) handlePluginRefs(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	v.PluginURL = strings.TrimSpace(r.FormValue("url"))
	refs, err := site.ListRefs(v.PluginURL)
	if err != nil {
		v.Error = err.Error()
	} else {
		v.Refs = refs
	}
	s.render(w, "pluginrefs", v)
}

// handlePluginAdd starts the single-flight job that clones and installs the
// plugin; progress streams to the Plugins page log.
func (s *Server) handlePluginAdd(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(r.FormValue("url"))
	ref := strings.TrimSpace(r.FormValue("ref"))
	backupFirst := r.FormValue("backupfirst") != ""
	if !s.job.startAddPlugin(url, ref, backupFirst, s.version) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/plugins", http.StatusSeeOther)
}

// handleSettingsUpdate git-pulls the catalogues and swaps the result back into
// the Settings page. CSRF-guarded like every state-changing POST.
func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	v.CatUpdates = updateCatalogues()
	s.render(w, "catresult", v)
}

func (s *Server) handleRecommendsPage(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	v.Recommends = recommends()
	v.Section = "Recommendations"
	s.render(w, "recommends", v)
}

// handlePosterPage serves the optional poster's sub-page; 404 when the image
// ships none. The breadcrumb shows the poster's own title (tr passes it through
// unchanged, as it is not a translation key).
func (s *Server) handlePosterPage(w http.ResponseWriter, r *http.Request) {
	if s.poster == nil {
		http.NotFound(w, r)
		return
	}
	v := s.buildView(r)
	v.Section = s.poster.Title
	s.render(w, "poster", v)
}

func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	// A name from the Backup dialog is cleaned to a safe "<name>.mdb"; blank (or
	// nothing usable left) falls back to the generated name in site.Backup.
	name := backup.CleanName(r.FormValue("name"))
	if name != "" {
		if err := backup.CheckName(name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if !s.job.startBackup(s.version, name) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/backups", http.StatusSeeOther)
}

func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	file := r.FormValue("file")
	if err := backup.CheckName(file); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Same wwwroot rule as install; an installed site keeps its URL.
	st, err := state.Load()
	if err != nil {
		st = &state.State{}
	}
	wwwroot := st.Wwwroot
	if wwwroot == "" {
		wwwroot = st.SiteURLFor(hostOnly(r.Host))
	}
	wwwroot, err = site.NormalizeURL(wwwroot)
	if err != nil {
		http.Error(w, "invalid demo site URL", http.StatusBadRequest)
		return
	}
	// The "Keep current codebase" checkbox restores data onto the code tree
	// already on disk, skipping the git checkout (the fast path). Unchecked (or
	// no site installed) rebuilds the backup's own codebase.
	o := site.RestoreOptions{
		File:     file,
		Wwwroot:  wwwroot,
		KeepCode: r.FormValue("keepcode") != "",
	}
	if !s.job.startRestore(o) {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	// A restore rebuilds the whole site, so it always lands on the dashboard —
	// progress is watched there (busy site card + log), and the restored site
	// shows up when the job finishes, exactly like install and reset. Wherever
	// it was started from (Backups page or the empty-dashboard chooser), the
	// Backups page is the wrong place to sit afterwards.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleBackupDelete(w http.ResponseWriter, r *http.Request) {
	// A running restore may be reading the file it was started from.
	if !s.job.idle() {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	if err := backup.Delete(r.FormValue("file")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/backups", http.StatusSeeOther)
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("f")
	path, err := backup.Path(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	// ServeFile keeps the Content-Type set above and adds Range support —
	// a gigabyte download over a flaky link can resume.
	http.ServeFile(w, r, path)
}

// handleBackupUpload streams a .mdb into the backups directory. It bypasses
// the s.csrf wrapper (see the route comment) and instead checks the origin
// up front and requires the form's csrf field to arrive BEFORE the file part
// — which the form guarantees by input order.
func (s *Server) handleBackupUpload(w http.ResponseWriter, r *http.Request) {
	if !sameOriginOK(r) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "expected a multipart upload", http.StatusBadRequest)
		return
	}
	csrfOK := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			http.Error(w, "no file in the upload", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch part.FormName() {
		case "csrf":
			val, _ := io.ReadAll(io.LimitReader(part, 1024))
			csrfOK = csrfMatches(r, string(val))
		case "file":
			if !csrfOK {
				http.Error(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
			name := part.FileName()
			if err := backup.CheckName(name); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			dest, err := backup.Path(name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if _, err := os.Stat(dest); err == nil {
				http.Error(w, "a backup named "+name+" already exists", http.StatusConflict)
				return
			}
			if err := s.receiveBackup(part, dest); err != nil {
				http.Error(w, "upload failed: "+err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/backups", http.StatusSeeOther)
			return
		}
	}
}

// receiveBackup streams the part to a working file, proves it is a readable
// backup archive, and only then gives it its real name.
func (s *Server) receiveBackup(part io.Reader, dest string) error {
	if err := backup.EnsureDir(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(backup.Dir, ".upload-*.partial")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, part); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := backup.Validate(tmp.Name()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// demoUsers is the one list behind the Accounts card and the SSO login flow:
// only names returned here may receive a login token.
func demoUsers(st *state.State) []userRow {
	if !st.Installed() {
		return nil
	}
	rows := []userRow{{Username: "admin", Password: st.AdminPass, Role: "Site admin"}}
	for _, u := range st.Users {
		rows = append(rows, userRow{Username: u.Username, Password: u.Password, Role: u.Role})
	}
	return rows
}

// ssoURL is the single-use login link for token, on the URL the visitor's
// device can actually reach.
func ssoURL(r *http.Request, token string) string {
	return siteBase(r) + "/mdl-demo/login.php?token=" + token
}

var usernameRe = regexp.MustCompile(`^[a-z0-9._-]{1,100}$`)

// roleLabels maps the createuser.php --role values to Accounts-card labels
// and doubles as the allowlist for the form's role field.
var roleLabels = map[string]string{"": "User", "manager": "Manager", "admin": "Site admin"}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	st, err := state.Load()
	if err != nil || !st.Installed() {
		redirect(w, r, "/")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	firstname := strings.TrimSpace(r.FormValue("firstname"))
	lastname := strings.TrimSpace(r.FormValue("lastname"))
	role := r.FormValue("role")
	label, roleOK := roleLabels[role]
	switch {
	case !usernameRe.MatchString(username):
		http.Error(w, "username must be 1-100 of a-z 0-9 . _ -", http.StatusBadRequest)
		return
	case firstname == "" || lastname == "" || len(firstname) > 100 || len(lastname) > 100:
		http.Error(w, "first and last name are required", http.StatusBadRequest)
		return
	case !roleOK:
		http.Error(w, "unknown role", http.StatusBadRequest)
		return
	}
	for _, u := range demoUsers(st) {
		if u.Username == username {
			http.Error(w, "user "+username+" already exists", http.StatusConflict)
			return
		}
	}
	password := site.RandomPassword()
	var tail []string
	logf := func(line string) {
		tail = append(tail, line)
		if len(tail) > 20 {
			tail = tail[1:]
		}
	}
	if err := moodle.CreateUser(logf, username, password, firstname, lastname, role); err != nil {
		http.Error(w, "creating the user failed: "+err.Error()+"\n\n"+strings.Join(tail, "\n"), http.StatusInternalServerError)
		return
	}
	// Reload: the install job may have written state while the script ran.
	st, err = state.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st.Users = append(st.Users, state.DemoUser{Username: username, Password: password, Role: label})
	if err := st.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, "/")
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
// active (a phone cannot reach the user's loopback), else the override/derived one.
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
	http.Redirect(w, r, ssoURL(r, token), http.StatusSeeOther)
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
	png, err := qrcode.Encode(ssoURL(r, token), qrcode.Medium, 512)
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
	// Claimed (or expired): stop the poll and tell the page, which closes
	// the dialog on the event (app.js). Not a script in the response — the
	// CSP forbids it, and htmx is configured not to run one anyway.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("HX-Trigger", "sso-done")
	fmt.Fprint(w, `<div id="ssopoll"></div>`)
}

// The tunnel handlers run synchronously: cloudflared announces its URL in a
// few seconds, so a plain form POST + redirect beats wiring them into the
// single-flight job. The captured log tail makes a failure diagnosable.
func (s *Server) handleTunnelStart(w http.ResponseWriter, r *http.Request) {
	if !tunnelEnabled() {
		http.NotFound(w, r)
		return
	}
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

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if !s.job.startReset() {
		http.Error(w, "another operation is already running", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// toolVersions reports the assembly toolchain for the debug report: mudev's
// version and the git revisions of the recipe and plugin catalogues (the camp
// registry will join them here). Each line degrades to "unknown" on error so
// the report always renders.
func toolVersions() string {
	var b strings.Builder
	mudevVer := "unknown"
	if out, err := execx.Output("", "mudev", "--version"); err == nil {
		mudevVer = strings.TrimSpace(out)
	}
	fmt.Fprintf(&b, "mudev: %s\n", mudevVer)
	// mudev's default catalogue paths (see Containerfile); recipes.Dir is the
	// recipe one.
	for _, c := range catalogues() {
		fmt.Fprintf(&b, "%s: %s\n", c.name, gitRev(c.dir))
	}
	// The supervised services, each self-reported (first line of --version).
	for _, s := range []struct {
		name string
		argv []string
	}{
		{"postgresql", []string{"psql", "--version"}},
		{"php", []string{"php", "--version"}},
		{"apache", []string{"apache2ctl", "-v"}},
		{"mailpit", []string{"mailpit", "version"}},
		{"cloudflared", []string{"cloudflared", "--version"}},
	} {
		fmt.Fprintf(&b, "%s: %s\n", s.name, cmdVersion(s.argv[0], s.argv[1:]...))
	}
	return b.String()
}

// osInfo reports the container OS for the bug report: distro (from
// /etc/os-release), kernel, and the binary's architecture.
func osInfo() string {
	distro := "unknown"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				distro = strings.Trim(v, `"`)
				break
			}
		}
	}
	return fmt.Sprintf("os: %s (kernel %s, %s)\n", distro, cmdVersion("uname", "-r"), runtime.GOARCH)
}

// cmdVersion runs a tool's version command and returns its first output line,
// "unknown" on error — best-effort detail for the bug report.
func cmdVersion(name string, args ...string) string {
	out, err := execx.Output("", name, args...)
	if err != nil {
		return "unknown"
	}
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		out = out[:i]
	}
	return strings.TrimSpace(out)
}

// catalogues are the git-checked-out code catalogues mudev clones from (see the
// Containerfile). The Settings page can git-pull them to pick up new recipes and
// plugins without rebuilding the image — even before the first install.
func catalogues() []struct{ name, dir string } {
	return []struct{ name, dir string }{
		{"mdl-recipes", recipes.Dir},
		{"mdl-plugins", "/srv/extra/mdl-plugins"},
	}
}

// updateCatalogues fast-forwards each catalogue checkout from its origin and
// reports per-repo. Fast-forward only: these are read-only mirrors, never edited
// in the container, so a non-ff would mean something is wrong — better to fail
// loudly than merge.
func updateCatalogues() []catUpdate {
	var out []catUpdate
	for _, c := range catalogues() {
		u := catUpdate{Name: c.name}
		if _, err := execx.Output(c.dir, "git", "pull", "--ff-only"); err != nil {
			u.Msg = err.Error()
		} else {
			u.OK = true
			u.Msg = gitRev(c.dir)
		}
		out = append(out, u)
	}
	return out
}

// gitRev is the short HEAD revision of a git checkout, "unknown" if unreadable.
func gitRev(dir string) string {
	out, err := execx.Output(dir, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

// handleSettingsPage renders the Settings page: the catalogue-update action and
// the diagnostics report — one copy-pasteable block of mode, versions, state and
// per-service status + log tails, so an end user can paste it into a bug report.
func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	v.Snapshot = true // the diagnostics report is a snapshot; don't reload it mid-read
	v.Section = "Settings"
	var b strings.Builder
	fmt.Fprintf(&b, "mdl-demo %s\nmode: %s\ntime: %s\n",
		s.version, svc.Current().Mode(), time.Now().UTC().Format(time.RFC3339))
	b.WriteString(osInfo())
	b.WriteString(toolVersions())
	if st, err := state.Load(); err == nil {
		fmt.Fprintf(&b, "demo: %s (console port %d, site port %d; inside %d/%d)\n",
			st.Title(), st.Port(), st.SitePort(), state.ConsoleListen, state.SiteListen)
		host := hostOnly(r.Host)
		fmt.Fprintf(&b, "urls: console %s, site %s", st.ConsoleURLFor(host), st.SiteURLFor(host))
		if st.SiteURL != "" {
			b.WriteString(" (overridden with `mdl-demo url`)")
		}
		b.WriteString("\n")
	}
	if v.Installed {
		fmt.Fprintf(&b, "site: %s (%s), installed %s\n", v.Recipe, v.Wwwroot, v.InstalledAt)
	} else {
		b.WriteString("site: none installed\n")
	}
	// The live recipe is kept OUT of the main report (its own box on the page):
	// it is long, and it lists plugin git sources a reporter may not want to
	// paste into a public issue — so sharing it is a separate, deliberate act.
	if v.Installed {
		if out, err := execx.Output(moodle.Root, "mudev", "recipe", "export", "--sort"); err == nil {
			v.RecipeExport = out
		}
	}
	if v.TunnelURL != "" {
		fmt.Fprintf(&b, "tunnel: %s\n", v.TunnelURL)
	} else {
		b.WriteString("tunnel: not running\n")
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
	s.render(w, "settings", v)
}

func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}
