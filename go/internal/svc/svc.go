// Package svc is the seam between the two processes that touch services:
// the PID-1 supervisor (`mdl-demo init`, the image's entrypoint — rich
// in-memory state) and CLI invocations exec'd into the container (separate
// processes — cross-process mechanisms only).
//
// Everything that touches services — reloading Apache after a vhost swap,
// arming Moodle cron, the dashboard's status card, the diagnostics page —
// goes through the Manager interface. The image deliberately has no
// systemd: a Docker-style runtime gives PID 1 no CAP_SYS_ADMIN and often a
// read-only cgroup tree, which systemd cannot boot under, while this
// arrangement needs no capabilities at all.
package svc

import (
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
	// Mode names the acting manager in status output.
	Mode() string
	ReloadApache(logf execx.Logf) error
	// EnableCron arms the per-minute Moodle cron (systemd timer or Go
	// ticker); DisableCron disarms it.
	EnableCron(logf execx.Logf) error
	DisableCron(logf execx.Logf) error
	Statuses() []Status
	Diagnostics() []Diag
}

var current Manager = NewStandalone()

// Use replaces the active manager; called once by `mdl-demo init` before
// anything else runs. Every other invocation keeps the standalone default.
func Use(m Manager) { current = m }

func Current() Manager { return current }
