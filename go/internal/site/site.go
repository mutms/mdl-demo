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

	logf("Assembling Moodle code tree from recipe " + recipe.ID + " (this clones several git repositories — expect a few minutes)")
	if err := execx.Run(logf, moodle.Root, "mudev", "clone", recipe.ID); err != nil {
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
// password survives; the site fields are cleared. Idempotent.
func Reset(logf execx.Logf) error {
	logf("Disabling Moodle cron")
	_ = svc.Current().DisableCron(logf)

	logf("Restoring placeholder site on port 8080")
	if err := apache.RestorePlaceholder(logf); err != nil {
		return err
	}

	logf("Dropping database " + pgdb.Name)
	if err := pgdb.Drop(logf); err != nil {
		return err
	}

	logf("Removing code tree and data")
	if err := clearDir(moodle.Root); err != nil {
		return err
	}
	if err := os.RemoveAll(moodle.Dataroot); err != nil {
		return err
	}

	s, err := state.Load()
	if err != nil {
		return err
	}
	s.Recipe, s.Wwwroot = "", ""
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
