package site

import (
	"github.com/mutms/mdl-demo/go/internal/debian"
	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/state"
)

// UpdateDebian installs the latest packages, security fixes above all, into a
// running demo. Services keep their old code until the container restarts, which
// is the user's to do: this job runs inside PID 1, and a container stops with it.
func UpdateDebian(logf execx.Logf) error {
	defer state.HoldBusy()() // pause the cron ticker while PHP and its libraries change
	changed, err := debian.Upgrade(logf)
	if err != nil {
		return err
	}
	if changed {
		logf("Done. Stop this container and start it again so running services use the new code.")
	} else {
		logf("Debian is already up to date.")
	}
	return nil
}
