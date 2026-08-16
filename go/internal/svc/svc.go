// Package svc is the seam between "services run under systemd" (podman,
// Apple container — the image's default boot) and "services run under
// mdl-demo's own PID-1 supervisor" (`mdl-demo init`, for runtimes that
// cannot boot systemd, e.g. the WSL containers preview).
//
// Everything that touches services — reloading Apache after a vhost swap,
// arming Moodle cron, the dashboard's status card, the diagnostics page —
// goes through the Manager interface, never straight to systemctl.
package svc

import (
	"os"

	"github.com/mutms/mdl-demo/internal/execx"
)

// Status feeds the dashboard's services card.
type Status struct {
	Name    string
	State   string // "active", "inactive", "failed", "restarting (3rd try)", …
	Running bool
}

// Diag feeds the diagnostics page: Status plus enough context for a
// copy-pasted bug report to be actionable.
type Diag struct {
	Status
	PID      int
	Restarts int
	LastExit string
	LogTail  []string
}

type Manager interface {
	// Mode names the boot mode in status output ("systemd" | "mdl-demo-init").
	Mode() string
	ReloadApache(logf execx.Logf) error
	// EnableCron arms the per-minute Moodle cron (systemd timer or Go
	// ticker); DisableCron disarms it.
	EnableCron(logf execx.Logf) error
	DisableCron(logf execx.Logf) error
	Statuses() []Status
	Diagnostics() []Diag
}

var current Manager = NewSystemd()

// Use replaces the active manager; called once by `mdl-demo init` before
// anything else runs.
func Use(m Manager) { current = m }

func Current() Manager { return current }

// UnderSystemd reports whether systemd is PID 1 on this boot.
func UnderSystemd() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}
