// Package state persists the little mdl-demo needs to remember across
// restarts: the demo's identity (console port and name, adopted from the
// container environment on first start), the site URL override and what is
// currently installed.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Path is the state file location. The init supervisor's cron ticker reads
// it every minute: cron runs while it records an installed site.
const Path = "/etc/mdl-demo/state.json"

// BusyPath marks a destructive site operation in progress (install, reset,
// plugin add, restore, backup). It is a file so it works across processes — the
// console's in-process job and a `mdl-demo …` CLI run alike — and the cron
// ticker skips while it exists, so a per-minute cron run can never collide with
// a schema upgrade or a half-wiped tree (which errored mid-plugin-install).
const BusyPath = "/etc/mdl-demo/busy.lock"

// busy refcounts holders in THIS process so a nested op (AddPlugin → Backup)
// keeps the lock until the outermost releases, not the first inner one.
var (
	busyMu sync.Mutex
	busyN  int
)

// HoldBusy marks the site busy and returns a release func — defer it. Nested
// calls in one process share the lock file; it is removed when the last holder
// releases.
func HoldBusy() func() {
	busyMu.Lock()
	busyN++
	if busyN == 1 {
		_ = os.MkdirAll(filepath.Dir(BusyPath), 0755)
		_ = os.WriteFile(BusyPath, []byte("1\n"), 0600)
	}
	busyMu.Unlock()
	return func() {
		busyMu.Lock()
		if busyN--; busyN == 0 {
			_ = os.Remove(BusyPath)
		}
		busyMu.Unlock()
	}
}

// Busy reports whether a destructive site operation is in progress (any process).
func Busy() bool {
	_, err := os.Stat(BusyPath)
	return err == nil
}

// ClearBusy removes a stale lock left by a crashed operation; call once at boot.
func ClearBusy() { _ = os.Remove(BusyPath) }

// Container-internal ports. They never change: the outside world maps its
// own console port NNNN to ConsoleListen and NNNN+1 to SiteListen, so the
// "+1" rule holds on both sides of the container boundary.
const (
	ConsoleListen = 8081
	SiteListen    = 8082
	// DefaultConsolePort is what the outside console port is assumed to be
	// when the container was started without MDL_DEMO_PORT.
	DefaultConsolePort = 8081
)

type State struct {
	// ConsolePort is the port the console is reachable on from outside the
	// container (MDL_DEMO_PORT, default 8081); the site is on ConsolePort+1.
	// It is the demo's identity: the container is named mdl-demo-<port>.
	ConsolePort int `json:"console_port,omitempty"`
	// Name is the optional human label (MDL_DEMO_NAME) shown in the console
	// heading and used as the default Moodle site name.
	Name string `json:"name,omitempty"`
	// SiteURL overrides the site URL derived from ConsolePort when something
	// sits in front of the container (a reverse proxy, a tunnel). Set with
	// `mdl-demo url`; empty means "derive from the port". There is
	// deliberately no console equivalent: the console answers to loopback
	// and IP addresses only (see webui/auth.go), so a setting that named it
	// something public would be an invitation to misuse it.
	SiteURL string `json:"site_url,omitempty"`

	Recipe  string `json:"recipe,omitempty"`
	Wwwroot string `json:"wwwroot,omitempty"`
	// Fullname is the Moodle site's full name as installed (or restored) —
	// the source for suggested backup file names.
	Fullname string `json:"fullname,omitempty"`
	// AdminPass is the demo site's admin password, kept in plain text on
	// purpose: the UI shows it on the site card so users can always find
	// it (this file is root-only 0600, and the site is a throwaway demo).
	AdminPass   string    `json:"admin_pass,omitempty"`
	InstalledAt time.Time `json:"installed_at,omitzero"`
	// Users are the extra demo accounts created from the console — recorded
	// here (passwords plain, same reasoning as AdminPass) so the Accounts
	// card and the single-use login links know them.
	Users []DemoUser `json:"users,omitempty"`
}

// DemoUser is one console-created demo account; Role is the display label
// ("Manager", "Site admin", "User").
type DemoUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *State) Installed() bool { return s.Recipe != "" }

// Port returns the outside console port, falling back to the default before
// the environment has been adopted.
func (s *State) Port() int {
	if s.ConsolePort > 0 {
		return s.ConsolePort
	}
	return DefaultConsolePort
}

// SitePort is the outside port of the Moodle site: always console + 1.
func (s *State) SitePort() int { return s.Port() + 1 }

// ID is the demo's identity, "mdl-demo-<console port>" — the same string the
// launcher uses as the container name.
func (s *State) ID() string { return fmt.Sprintf("mdl-demo-%d", s.Port()) }

// Title is the console's page title: the ID, plus the name when one is set.
func (s *State) Title() string {
	if s.Name != "" {
		return s.ID() + " · " + s.Name
	}
	return s.ID()
}

// DefaultHost is the host to build a URL for when there is no browser to
// take one from — the CLI's own output, and the wwwroot a CLI install bakes
// in. It is the literal 127.0.0.1 rather than "localhost" on purpose: every
// documented run command publishes the ports on 127.0.0.1 only, and
// "localhost" resolves to ::1 first on plenty of machines, where nothing is
// listening. The console still answers to "localhost" (webui/auth.go) for
// people who type it; it just never hands the name out itself.
const DefaultHost = "127.0.0.1"

// ConsoleURLFor returns the console URL as a browser at host sees it:
// always http://host:<console port>.
func (s *State) ConsoleURLFor(host string) string {
	return fmt.Sprintf("http://%s:%d", host, s.Port())
}

// SiteURLFor returns the Moodle site URL as a browser at host sees it: the
// override when set, else http://host:<console port + 1>.
func (s *State) SiteURLFor(host string) string {
	if s.SiteURL != "" {
		return s.SiteURL
	}
	return fmt.Sprintf("http://%s:%d", host, s.SitePort())
}

// Load returns the saved state, or a zero state if none exists yet.
func Load() (*State, error) {
	data, err := os.ReadFile(Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := &State{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Save writes the state atomically; 0600 because the demo accounts'
// credentials are in it.
func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := Path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(Path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, Path)
}

// ContainerEnv returns a variable from the container's environment — the
// `-e` values the runtime hands to PID 1. Inside `mdl-demo init` that is our
// own environment; an exec'd CLI process (or `mdl-demo serve` under some
// other init) reads it off /proc/1/environ instead, which works because
// mdl-demo runs as root.
func ContainerEnv(key string) string {
	if os.Getpid() == 1 {
		return os.Getenv(key)
	}
	data, err := os.ReadFile("/proc/1/environ")
	if err != nil {
		return ""
	}
	for _, kv := range strings.Split(string(data), "\x00") {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}

// AdoptIdentity records the console port and name from MDL_DEMO_PORT /
// MDL_DEMO_NAME the first time it runs; later starts keep what was recorded
// (a container's environment cannot change anyway). It reports whether the
// state changed; warnf receives one line per ignored bad value.
func (s *State) AdoptIdentity(port, name string, warnf func(string)) bool {
	changed := false
	if s.ConsolePort == 0 {
		s.ConsolePort = ParsePort(port, warnf)
		changed = true
	}
	if s.Name == "" {
		if name = strings.TrimSpace(name); name != "" {
			s.Name = name
			changed = true
		}
	}
	return changed
}

// ParsePort turns the MDL_DEMO_PORT value into a console port: 1–65534 (the
// site needs port+1), defaulting to DefaultConsolePort for empty or bad
// input with a warning for the latter.
func ParsePort(raw string, warnf func(string)) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultConsolePort
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65534 {
		if warnf != nil {
			warnf(fmt.Sprintf("ignoring MDL_DEMO_PORT=%q: expected a port number 1–65534, using %d", raw, DefaultConsolePort))
		}
		return DefaultConsolePort
	}
	return n
}
