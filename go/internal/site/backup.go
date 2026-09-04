package site

// Backup and restore of the whole demo site: database + dataroot + the
// tree's live recipe, packed by internal/backup into one .mdb file. A backup
// is self-contained — restore rebuilds the exact code tree from the embedded
// recipe with no dependency on the recipe catalogue — and carries no
// passwords: account passwords are regenerated at the end of every restore.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mutms/mdl-demo/go/internal/apache"
	"github.com/mutms/mdl-demo/go/internal/backup"
	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/pgdb"
	"github.com/mutms/mdl-demo/go/internal/recipes"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/svc"
	"github.com/mutms/mdl-demo/go/internal/tunnel"
)

// dataCaches are the dataroot dirs Moodle rebuilds on demand — dead weight in
// a backup. Everything else (filedir above all, but also lang packs and muc
// config) is kept.
var dataCaches = []string{"cache", "localcache", "temp", "sessions", "trashdir", "lock"}

// meta.json speaks the machine role vocabulary ("admin", "manager", "user" —
// the same ids createuser.php takes); state carries the Accounts card's
// display labels. These convert at the boundary, tolerating unknown values.
func roleID(label string) string {
	switch label {
	case "Site admin":
		return "admin"
	case "Manager":
		return "manager"
	case "User":
		return "user"
	}
	return label
}

func roleLabel(id string) string {
	switch id {
	case "admin":
		return "Site admin"
	case "manager":
		return "Manager"
	case "user":
		return "User"
	}
	return id
}

// Backup snapshots the installed site into a new .mdb file in backup.Dir and
// returns its name. version stamps the archive metadata.
func Backup(logf execx.Logf, version, name string) (string, error) {
	st, err := state.Load()
	if err != nil {
		return "", err
	}
	if !st.Installed() || !moodle.Detected() {
		return "", fmt.Errorf("no site to back up")
	}
	if err := backup.EnsureDir(); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(backup.Dir, ".staging-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	fullname := st.Fullname
	if fullname == "" {
		fullname = st.Name
	}
	// A user-supplied name (Backup dialog or CLI arg) wins, cleaned to a safe
	// "<name>.mdb"; otherwise generate one from the site name and the time.
	name = backup.CleanName(name)
	if name == "" {
		name = backup.SuggestName(fullname, time.Now())
	}
	logf("Backing up the site into " + name)

	// Whenever the site's internals (database, data directory) are being
	// copied or modified, Apache serves the placeholder instead of the site —
	// the same guard reset and restore use, and it keeps the snapshot
	// consistent by blocking web writes. The generated vhost stays in place,
	// so EnableDemo brings the site straight back — always, failure included.
	logf("Switching the site to the placeholder page")
	if err := apache.DisableDemo(logf); err != nil {
		logf("warning: could not switch to the placeholder, backing up live: " + err.Error())
	}
	defer func() {
		if err := apache.EnableDemo(logf); err != nil {
			logf("warning: could not bring the site back: " + err.Error())
		}
	}()

	logf("Exporting the code tree's recipe")
	if err := execx.Run(logf, moodle.Root, "mudev", "recipe", "export", "--sort",
		"--file", filepath.Join(staging, backup.RecipeName)); err != nil {
		return "", err
	}

	logf("Dumping database " + pgdb.Name)
	if err := pgdb.DumpTo(logf, filepath.Join(staging, backup.DBName)); err != nil {
		return "", err
	}

	meta := backup.Meta{
		Revision: backup.Revision,
		Recipe:   st.Recipe,
		Fullname: fullname,
		Created:  time.Now().UTC(),
		Version:  version,
		// The full console account list, admin included — names and roles
		// only, never passwords (restore generates fresh ones).
		Users: []backup.MetaUser{{Username: "admin", Role: "admin"}},
	}
	for _, u := range st.Users {
		meta.Users = append(meta.Users, backup.MetaUser{Username: u.Username, Role: roleID(u.Role)})
	}
	mj, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(staging, backup.MetaName), mj, 0600); err != nil {
		return "", err
	}

	logf("Packing the archive (database dump + data directory)")
	partial := filepath.Join(backup.Dir, "."+name+".partial")
	defer os.Remove(partial)
	args := []string{"czf", partial}
	for _, c := range dataCaches {
		args = append(args, "--exclude="+backup.DataPrefix+"/"+c)
	}
	// Member order matters: meta.json must be the first entry (cheap listing).
	args = append(args, "-C", staging, backup.MetaName, backup.RecipeName, backup.DBName,
		"-C", filepath.Dir(moodle.Dataroot), backup.DataPrefix)
	if err := execx.Run(logf, "", "tar", args...); err != nil {
		return "", err
	}
	if err := os.Chmod(partial, 0600); err != nil {
		return "", err
	}
	if err := os.Rename(partial, filepath.Join(backup.Dir, name)); err != nil {
		return "", err
	}
	if fi, err := os.Stat(filepath.Join(backup.Dir, name)); err == nil {
		logf(fmt.Sprintf("Backup ready: %s (%.1f MB)", name, float64(fi.Size())/1e6))
	}
	return name, nil
}

// CurrentRecipeHash returns the SHA-256 (hex) of the code tree's
// `mudev recipe export --sort` output — the exact recipe form a backup embeds
// (see backup.RecipeHash), so the two hashes compare directly to tell whether a
// restore can keep the code already on disk. Read-only on the tree; the caller
// should only run it for an installed, idle site. mudev insists on a .yaml
// destination (no stdout), so it goes through a temp file.
func CurrentRecipeHash() (string, error) {
	if !moodle.Detected() {
		return "", fmt.Errorf("no Moodle tree to hash")
	}
	tmp, err := os.CreateTemp("", "mdl-recipe-*.yaml")
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if _, err := execx.Output(moodle.Root, "mudev", "recipe", "export", "--sort", "--file", tmp.Name()); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// RestoreOptions selects what to restore and onto which code tree.
type RestoreOptions struct {
	// File is the backup's name inside backup.Dir.
	File string
	// Wwwroot for the restored site; default: the recorded site URL for
	// 127.0.0.1 (same rule as Install).
	Wwwroot string
	// Recipe switches the restore's code tree: empty rebuilds the exact tree
	// from the backup's embedded recipe; a catalogue ID assembles that recipe
	// instead — the upgrade path (back up on 4.5, restore into 5.3), with
	// upgrade.php bridging the data.
	Recipe string
	// KeepCode restores only the database and data onto the code tree already on
	// disk, skipping the minutes-long git checkout — the fast path when the user
	// knows the current tree already matches the backup (or is a version they
	// want to upgrade the backup's data into). Requires an installed site;
	// upgrade.php still bridges the data to whatever version the tree carries.
	// Overrides Recipe.
	KeepCode bool
}

// Restore replaces the demo site with a backup's content. It always wipes and
// rebuilds both tree and data — data from one Moodle version never runs on a
// mismatched tree by accident. Fresh passwords are generated for admin and
// every account the backup lists.
func Restore(logf execx.Logf, o RestoreOptions) error {
	warn := func(what string, err error) {
		if err != nil {
			logf("warning: " + what + " failed: " + err.Error())
		}
	}

	// Everything that can be validated happens before anything is destroyed.
	path, err := backup.Path(o.File)
	if err != nil {
		return err
	}
	logf("Validating backup " + o.File)
	meta, err := backup.Validate(path)
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}
	treeRecipe := meta.Recipe
	if o.KeepCode {
		// Keeping the code means keeping the tree that is already installed, so
		// there must be one — and its recipe is what the restored site carries.
		if !st.Installed() || !moodle.Detected() {
			return fmt.Errorf("keep current code needs an installed Moodle tree")
		}
		treeRecipe = st.Recipe
	} else if o.Recipe != "" {
		rec, err := recipes.Get(o.Recipe)
		if err != nil {
			return err
		}
		treeRecipe = rec.ID
	}
	if o.Wwwroot == "" {
		o.Wwwroot = st.SiteURLFor(state.DefaultHost)
	}
	staging, err := os.MkdirTemp(backup.Dir, ".staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	// Teardown, mirroring Reset. The site state is cleared FIRST: the initd
	// cron ticker keys off Installed(), and a restore is minutes long — the
	// ticker must not fire into a half-restored site. A failure from here on
	// leaves an honest "no site" state; reset or a retry recovers.
	warn("stopping the tunnel", tunnel.Stop(logf))
	logf("Disabling Moodle cron")
	_ = svc.Current().DisableCron(logf)
	if err := clearSiteState(); err != nil {
		return err
	}
	logf("Restoring placeholder site")
	if err := apache.RestorePlaceholder(logf); err != nil {
		return err
	}
	if o.KeepCode {
		logf("Removing the current database and data (keeping the code tree)")
	} else {
		logf("Removing the current database, code tree and data")
	}
	warn("dropping the database", pgdb.Drop(logf))
	if !o.KeepCode {
		if err := clearDir(moodle.Root); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(moodle.Dataroot); err != nil {
		return err
	}

	// Rebuild: code tree from the chosen recipe (or the backup's own), then
	// the same provisioning steps as Install, with the installer replaced by
	// the dump + dataroot + upgrade.php.
	if o.KeepCode {
		logf("Keeping the current Moodle code tree (" + treeRecipe + ")")
	} else if o.Recipe != "" {
		logf("Assembling Moodle code tree from recipe " + treeRecipe + " (shallow clone of several git repositories)")
		if err := execx.Run(logf, moodle.Root, "mudev", "clone", "--shallow", treeRecipe); err != nil {
			return err
		}
	} else {
		logf("Assembling Moodle code tree from the backup's recipe (shallow clone of several git repositories)")
		recipeFile := filepath.Join(staging, backup.RecipeName)
		if err := backup.ExtractFile(path, backup.RecipeName, recipeFile); err != nil {
			return err
		}
		if err := execx.Run(logf, moodle.Root, "mudev", "clone", "--shallow", recipeFile); err != nil {
			return err
		}
	}
	if !moodle.Detected() {
		return fmt.Errorf("no Moodle tree found in %s after mudev clone", moodle.Root)
	}

	logf("Writing config.php (wwwroot " + o.Wwwroot + ")")
	if err := moodle.WriteConfig(o.Wwwroot); err != nil {
		return err
	}
	logf("Installing the console's PHP endpoints (mdl-demo/)")
	if err := moodle.InstallPHP(logf); err != nil {
		return err
	}

	logf("Provisioning PostgreSQL database " + pgdb.Name)
	if err := pgdb.Provision(logf); err != nil {
		return err
	}
	logf("Loading the database dump")
	dumpFile := filepath.Join(staging, backup.DBName)
	if err := backup.ExtractFile(path, backup.DBName, dumpFile); err != nil {
		return err
	}
	if err := pgdb.LoadFrom(logf, dumpFile); err != nil {
		return err
	}

	logf("Restoring the data directory")
	if err := makeDataroot(logf); err != nil {
		return err
	}
	if err := backup.ExtractData(path, filepath.Dir(moodle.Dataroot)); err != nil {
		return err
	}
	if err := execx.Run(logf, "", "chown", "-R", "www-data:www-data", moodle.Dataroot); err != nil {
		return err
	}
	if err := os.Chmod(moodle.Dataroot, 02777); err != nil {
		return err
	}

	logf("Upgrading the restored data to the code tree's version")
	if err := moodle.Upgrade(logf); err != nil {
		return err
	}
	warn("purging caches", moodle.PurgeCaches(logf))
	// A backup taken in maintenance mode restores into maintenance mode;
	// always lift it.
	warn("disabling maintenance mode", moodle.Maintenance(logf, false))

	// Backups carry no passwords — generate fresh ones for every account the
	// backup lists so the dashboard and the login links work immediately.
	// Losing a demo account is a warning; losing admin fails the restore.
	logf("Generating new passwords for the console's accounts")
	var adminPass string
	var users []state.DemoUser
	for _, u := range meta.Users {
		pw := RandomPassword()
		if u.Username == "admin" {
			if err := moodle.SetPassword(logf, "admin", pw); err != nil {
				return err
			}
			adminPass = pw
			continue
		}
		if err := moodle.SetPassword(logf, u.Username, pw); err != nil {
			warn("setting the password for "+u.Username, err)
			continue
		}
		users = append(users, state.DemoUser{Username: u.Username, Password: pw, Role: roleLabel(u.Role)})
	}
	if adminPass == "" {
		// A meta that does not list admin (it always exists in the site).
		adminPass = RandomPassword()
		if err := moodle.SetPassword(logf, "admin", adminPass); err != nil {
			return err
		}
	}

	docroot := moodle.Docroot()
	logf("Configuring Apache (docroot " + docroot + ")")
	if err := apache.WriteVhost(o.Wwwroot, docroot, moodle.HasRouter()); err != nil {
		return err
	}
	if err := apache.EnableDemo(logf); err != nil {
		return err
	}
	logf("Enabling Moodle cron")
	if err := svc.Current().EnableCron(logf); err != nil {
		return err
	}

	// Reload: the console may have recorded something while the restore ran.
	st, err = state.Load()
	if err != nil {
		return err
	}
	st.Recipe = treeRecipe
	st.Wwwroot = o.Wwwroot
	st.Fullname = meta.Fullname
	st.AdminPass = adminPass
	st.Users = users
	st.InstalledAt = time.Now().UTC()
	if err := st.Save(); err != nil {
		return err
	}

	logf("")
	logf("Site restored from " + o.File + ": " + o.Wwwroot)
	logf("New passwords were generated for all demo accounts — see the dashboard.")
	return nil
}
