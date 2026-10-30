// Package debian runs apt inside the container, at run time. Package scripts
// cannot touch the services here: the base image's policy-rc.d forbids it, and
// PID 1 owns them. So an upgraded service keeps running its old code until the
// container restarts.
package debian

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

// apt runs apt-get with no prompts; a package's changed config file keeps the
// local version, because the image customizes several of them.
func apt(logf execx.Logf, args ...string) error {
	return execx.Run(logf, "", "env", append([]string{"DEBIAN_FRONTEND=noninteractive", "apt-get",
		"-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold"}, args...)...)
}

// removeLists drops the package indexes apt-get update fetched, as the
// Containerfile does.
func removeLists() error {
	lists, err := filepath.Glob("/var/lib/apt/lists/*")
	if err != nil {
		return err
	}
	for _, path := range lists {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// Install adds packages, without their recommendations.
func Install(logf execx.Logf, packages ...string) error {
	if err := apt(logf, "update"); err != nil {
		return err
	}
	if err := apt(logf, append([]string{"install", "-y", "--no-install-recommends"}, packages...)...); err != nil {
		return err
	}
	return removeLists()
}

// Upgrade brings every installed package up to date, reporting whether anything
// changed.
func Upgrade(logf execx.Logf) (changed bool, err error) {
	if err := apt(logf, "update"); err != nil {
		return false, err
	}
	changed = true
	watch := func(line string) {
		if isNoChange(line) {
			changed = false
		}
		logf(line)
	}
	if err := apt(watch, "dist-upgrade", "-y", "--no-install-recommends"); err != nil {
		return false, err
	}
	return changed, removeLists()
}

// isNoChange recognizes the summary apt prints when there was nothing to do.
func isNoChange(line string) bool {
	return strings.HasPrefix(line, "0 upgraded, 0 newly installed, 0 to remove")
}
