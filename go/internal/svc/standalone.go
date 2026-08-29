package svc

import (
	"os/exec"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

// standalone is the manager for processes that are NOT PID 1 in a
// no-systemd boot — e.g. `mdl-demo install` run via `podman exec` while
// `mdl-demo init` supervises. Every action works cross-process: Apache
// reloads via its pidfile, cron is driven by state.json (the init ticker
// reads it each minute), and statuses are best-effort process checks.
type standalone struct{}

func NewStandalone() Manager { return standalone{} }

func (standalone) Mode() string { return "mdl-demo-init (cli)" }

func (standalone) ReloadApache(logf execx.Logf) error {
	return execx.Run(logf, "", "apache2ctl", "graceful")
}

// EnableCron/DisableCron are deliberate no-ops: in init mode the ticker
// runs whenever state.json says a site is installed, which the install and
// reset flows already maintain.
func (standalone) EnableCron(logf execx.Logf) error {
	logf("Moodle cron runs automatically while a site is installed (mdl-demo init ticker)")
	return nil
}

func (standalone) DisableCron(logf execx.Logf) error {
	logf("Moodle cron stops automatically once the site is removed")
	return nil
}

func (standalone) Statuses() []Status {
	var out []Status
	for _, name := range []string{"postgres", "php-fpm8.3", "apache2", "mailpit"} {
		running := exec.Command("pidof", name).Run() == nil
		st := Status{Name: name, State: "inactive", Running: running}
		if running {
			st.State = "active"
		}
		out = append(out, st)
	}
	return out
}

func (s standalone) Diagnostics() []Diag {
	var out []Diag
	for _, st := range s.Statuses() {
		out = append(out, Diag{Status: st, LogTail: []string{
			"(run from CLI outside PID 1 — full logs are on the web UI diagnostics page)",
		}})
	}
	return out
}
