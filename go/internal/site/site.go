// Package site orchestrates the demo site lifecycle — install, status,
// reset — shared by the CLI and the web UI. One site per container: all
// paths and credentials are fixed.
package site

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mutms/mdl-demo/internal/apache"
	"github.com/mutms/mdl-demo/internal/execx"
	"github.com/mutms/mdl-demo/internal/moodle"
	"github.com/mutms/mdl-demo/internal/pgdb"
	"github.com/mutms/mdl-demo/internal/recipes"
	"github.com/mutms/mdl-demo/internal/state"
	"github.com/mutms/mdl-demo/internal/svc"
)

type Options struct {
	Recipe    string
	AdminPass string
	Fullname  string // default: the recipe's name
	Wwwroot   string // default http://localhost:8080
}

// Install provisions the whole site: database, code tree, config, Apache,
// Moodle installer, cron timer. Fails fast if a site is already present.
func Install(logf execx.Logf, o Options) error {
	if o.Recipe == "" || o.AdminPass == "" {
		return fmt.Errorf("recipe and admin password are required")
	}
	if o.Wwwroot == "" {
		o.Wwwroot = "http://localhost:8080"
	}
	recipe, err := recipes.Get(o.Recipe)
	if err != nil {
		return err
	}
	if o.Fullname == "" {
		o.Fullname = recipe.Name
		if o.Fullname == "" {
			o.Fullname = recipe.ID
		}
	}

	entries, err := os.ReadDir(moodle.Root)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty — a site is already installed (use `mdl-demo reset` first)", moodle.Root)
	}

	logf("Creating dataroot " + moodle.Dataroot)
	if err := os.MkdirAll(moodle.Dataroot, 02777); err != nil {
		return err
	}
	if err := os.Chmod(moodle.Dataroot, 02777); err != nil {
		return err
	}
	if err := execx.Run(logf, "", "chown", "www-data:www-data", moodle.Dataroot); err != nil {
		return err
	}

	logf("Provisioning PostgreSQL database " + pgdb.Name)
	if err := pgdb.Provision(logf); err != nil {
		return err
	}
	count, err := pgdb.TableCount()
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("database %q already holds %d tables (use `mdl-demo reset` first)", pgdb.Name, count)
	}

	logf("Assembling Moodle code tree from recipe " + recipe.ID + " (shallow clone of several git repositories)")
	if err := execx.Run(logf, moodle.Root, "mudev", "clone", "--shallow", recipe.ID); err != nil {
		return err
	}
	if !moodle.Detected() {
		return fmt.Errorf("no Moodle tree found in %s after mudev clone", moodle.Root)
	}

	logf("Writing config.php (wwwroot " + o.Wwwroot + ")")
	if err := moodle.WriteConfig(o.Wwwroot); err != nil {
		return err
	}

	docroot := moodle.Docroot()
	logf("Configuring Apache (docroot " + docroot + ")")
	if err := apache.WriteVhost(docroot, moodle.HasRouter()); err != nil {
		return err
	}
	if err := apache.EnableDemo(logf); err != nil {
		return err
	}

	logf("Installing Moodle database (this also takes a few minutes)")
	if err := moodle.InstallDatabase(logf, o.Fullname, o.AdminPass); err != nil {
		return err
	}

	logf("Enabling Moodle cron")
	if err := svc.Current().EnableCron(logf); err != nil {
		return err
	}

	s, err := state.Load()
	if err != nil {
		return err
	}
	s.Recipe = recipe.ID
	s.Wwwroot = o.Wwwroot
	s.AdminPass = o.AdminPass
	s.InstalledAt = time.Now().UTC()
	if err := s.Save(); err != nil {
		return err
	}

	logf("")
	logf("Demo site ready: " + o.Wwwroot)
	logf("Log in as: admin / " + o.AdminPass)
	return nil
}

// Reset tears the site down to the just-built image state. The web UI
// password survives; the site fields are cleared.
//
// Reset is the escape hatch, so it must land the container back in a clean,
// usable "no site" state from ANY starting condition — a healthy site, a
// half-assembled tree left by an interrupted install, a broken install. Every
// step therefore runs best-effort and logs its own failure instead of aborting
// the ones after it. What makes the result "reset worked" for the user is the
// placeholder serving on 8080 and the cleared state the UI reads; a database or
// a tree that will not go away (e.g. Postgres momentarily down) is a warning to
// retry, not a reason to leave the site stuck — so only those two steps can
// fail the reset, and it stays safe to run again.
func Reset(logf execx.Logf) error {
	warn := func(what string, err error) {
		if err != nil {
			logf("warning: " + what + " failed: " + err.Error())
		}
	}

	logf("Disabling Moodle cron")
	_ = svc.Current().DisableCron(logf)

	logf("Restoring placeholder site on port 8080")
	placeholderErr := apache.RestorePlaceholder(logf)
	warn("restoring the placeholder", placeholderErr)

	logf("Dropping database " + pgdb.Name)
	warn("dropping the database", pgdb.Drop(logf))

	logf("Removing code tree and data")
	warn("removing the code tree", clearDir(moodle.Root))
	warn("removing the data directory", os.RemoveAll(moodle.Dataroot))

	logf("Clearing the recorded site")
	stateErr := clearSiteState()
	warn("clearing the recorded site", stateErr)

	// Only the two steps that make the UI usable again can fail the reset;
	// everything else is recoverable by installing or resetting once more.
	switch {
	case placeholderErr != nil:
		return fmt.Errorf("could not restore the placeholder site: %w", placeholderErr)
	case stateErr != nil:
		return fmt.Errorf("could not clear the recorded site: %w", stateErr)
	}
	return nil
}

// clearSiteState wipes the recorded site fields, leaving the web UI password
// (and anything else in state) intact.
func clearSiteState() error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	s.Recipe, s.Wwwroot, s.AdminPass = "", "", ""
	s.InstalledAt = time.Time{}
	return s.Save()
}

// clearDir empties dir without removing it (it is baked into the image).
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
