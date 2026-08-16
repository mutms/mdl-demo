package svc

import (
	"os/exec"
	"strings"

	"github.com/mutms/mdl-demo/internal/execx"
)

// units the systemd mode watches; moodle-cron.timer last so the dashboard
// reads naturally (web servers first, cron arming last).
var systemdUnits = []string{"apache2", "php8.3-fpm", "postgresql", "moodle-cron.timer"}

type systemdManager struct{}

func NewSystemd() Manager { return systemdManager{} }

func (systemdManager) Mode() string { return "systemd" }

func (systemdManager) ReloadApache(logf execx.Logf) error {
	return execx.Run(logf, "", "systemctl", "reload", "apache2")
}

func (systemdManager) EnableCron(logf execx.Logf) error {
	return execx.Run(logf, "", "systemctl", "enable", "--now", "moodle-cron.timer")
}

func (systemdManager) DisableCron(logf execx.Logf) error {
	return execx.Run(logf, "", "systemctl", "disable", "--now", "moodle-cron.timer")
}

func (systemdManager) Statuses() []Status {
	var out []Status
	for _, unit := range systemdUnits {
		state := unitState(unit)
		out = append(out, Status{Name: unit, State: state, Running: state == "active"})
	}
	return out
}

func (m systemdManager) Diagnostics() []Diag {
	var out []Diag
	for _, s := range m.Statuses() {
		d := Diag{Status: s}
		if tail, err := exec.Command("journalctl", "-u", s.Name, "-n", "25", "--no-pager", "-o", "short-iso").Output(); err == nil {
			d.LogTail = strings.Split(strings.TrimRight(string(tail), "\n"), "\n")
		}
		out = append(out, d)
	}
	return out
}

// unitState is `systemctl is-active`: it exits nonzero for anything but
// active while still printing the state word we want.
func unitState(unit string) string {
	out, err := exec.Command("systemctl", "is-active", unit).Output()
	state := strings.TrimSpace(string(out))
	if state == "" && err != nil {
		return "unknown"
	}
	return state
}
