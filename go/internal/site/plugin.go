package site

// Adding a Moodle plugin to the demo site from a git repository. The tree is
// root-owned (invariant 3), so the clone/move/mudev steps run as root; only the
// read-only component→relpath resolver drops to www-data (moodle.PluginRelpath).
// The add is recorded in .mudev.json via `mudev recipe update`, so a backup's
// `mudev recipe export` captures it and restore reproduces it — the reason git
// sources are accepted and zip uploads are not.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/state"
)

// reComponent pulls a plugin's frankenstyle component from its version.php
// ($plugin->component, or the legacy $module->component) without executing the
// untrusted file.
var reComponent = regexp.MustCompile(`\$(?:plugin|module)->component\s*=\s*['"]([^'"]+)['"]`)

// reMoodleBranch matches a plugin's per-Moodle branch name, e.g. MOODLE_502_STABLE.
var reMoodleBranch = regexp.MustCompile(`^MOODLE_(\d+)_STABLE$`)

// allowedGitURL keeps the operator to git transports git actually needs, so a
// crafted "ext::" or shell-transport URL cannot be used. exec runs git without a
// shell, so this is belt-and-braces, not the only guard.
func allowedGitURL(u string) bool {
	u = strings.TrimSpace(u)
	switch {
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"),
		strings.HasPrefix(u, "git://"), strings.HasPrefix(u, "ssh://"),
		strings.HasPrefix(u, "file:///"), strings.HasPrefix(u, "/"):
		return true
	}
	// scp-style: user@host:path (no scheme, single colon before a path).
	if m, _ := regexp.MatchString(`^[^/@]+@[^/:]+:.+`, u); m {
		return true
	}
	return false
}

// Refs is a repository's branches and tags plus the ref to pre-select.
type Refs struct {
	Branches []string
	Tags     []string
	Proposed string
}

// ListRefs runs `git ls-remote` (with a hard timeout so an unreachable host
// cannot hang the console) and proposes a ref: the highest MOODLE_<n>_STABLE
// branch the repo offers that still serves this site's Moodle branch, else a
// default branch, else the first tag.
func ListRefs(url string) (Refs, error) {
	var r Refs
	if !allowedGitURL(url) {
		return r, fmt.Errorf("not a git URL")
	}
	// `timeout` caps a stuck fetch; execx.Output routes the wait through PID 1's
	// reaper and keeps stdout clean.
	out, err := execx.Output("", "timeout", "20", "git", "ls-remote", "--heads", "--tags", url)
	if err != nil {
		return r, fmt.Errorf("could not read the repository: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		i := strings.IndexByte(line, '\t')
		if i < 0 {
			continue
		}
		ref := strings.TrimSpace(line[i+1:])
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			r.Branches = append(r.Branches, strings.TrimPrefix(ref, "refs/heads/"))
		case strings.HasPrefix(ref, "refs/tags/"):
			// Drop peeled tag entries (refs/tags/x^{}).
			if !strings.HasSuffix(ref, "^{}") {
				r.Tags = append(r.Tags, strings.TrimPrefix(ref, "refs/tags/"))
			}
		}
	}
	r.Proposed = proposeRef(r.Branches, r.Tags, moodle.Branch())
	return r, nil
}

func proposeRef(branches, tags []string, siteBranch string) string {
	site, _ := strconv.Atoi(siteBranch)
	best, bestName := -1, ""
	for _, b := range branches {
		if m := reMoodleBranch.FindStringSubmatch(b); m != nil {
			n, _ := strconv.Atoi(m[1])
			// A MOODLE_n_STABLE branch serves Moodle n and later, so it fits when
			// n <= the site's branch; take the newest such.
			if (site == 0 || n <= site) && n > best {
				best, bestName = n, b
			}
		}
	}
	if bestName != "" {
		return bestName
	}
	for _, def := range []string{"main", "master"} {
		for _, b := range branches {
			if b == def {
				return b
			}
		}
	}
	if len(branches) > 0 {
		return branches[0]
	}
	if len(tags) > 0 {
		return tags[0]
	}
	return ""
}

// AddPlugin clones a plugin from a git repo at ref, places it in the tree at the
// path Moodle assigns its component, records it with mudev, and upgrades. Runs
// in the single-flight job (as root). When backupFirst is set, it takes a
// "prior-<component>-<time>.mdb" backup just before touching the tree, so a
// misbehaving plugin is one restore away from undone.
func AddPlugin(logf execx.Logf, url, ref string, backupFirst bool, version string) error {
	defer state.HoldBusy()() // pause the cron ticker across clone + upgrade
	if !allowedGitURL(url) {
		return fmt.Errorf("not a git URL")
	}
	if strings.TrimSpace(ref) == "" {
		return fmt.Errorf("no version selected")
	}
	if !moodle.Detected() {
		return fmt.Errorf("no demo site installed")
	}

	// Clone into a temp dir on the same filesystem as the tree, so the final
	// move is a rename. Kept shallow at the chosen ref.
	tmp, err := os.MkdirTemp(filepath.Dir(moodle.Root), ".plugin-add-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	checkout := filepath.Join(tmp, "src")

	logf(fmt.Sprintf("Cloning %s at %s", url, ref))
	if err := execx.Run(logf, "", "git", "clone", "--depth", "1", "--branch", ref, url, checkout); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Read the plugin's declared component from its own version.php (by regex —
	// never execute the untrusted file just to identify it).
	data, err := os.ReadFile(filepath.Join(checkout, "version.php"))
	if err != nil {
		return fmt.Errorf("no version.php — not a Moodle plugin")
	}
	m := reComponent.FindSubmatch(data)
	if m == nil {
		return fmt.Errorf("version.php declares no $plugin->component — cannot place the plugin")
	}
	component := string(m[1])
	logf("Plugin component: " + component)

	// Undo point: back up the site as it is now (without the plugin) before the
	// tree is touched, named for the plugin so it is easy to find.
	if backupFirst {
		logf("Backing up first (undo point)")
		if _, err := Backup(logf, version, "prior-"+component+"-"+time.Now().Format("20060102-150405")); err != nil {
			return fmt.Errorf("pre-install backup failed: %w", err)
		}
	}

	relpath, err := moodle.PluginRelpath(component)
	if err != nil {
		return fmt.Errorf("resolving install path: %w", err)
	}
	// Defence in depth: relpath must stay inside the tree.
	dest := filepath.Join(moodle.Root, relpath)
	if !strings.HasPrefix(dest+string(os.PathSeparator), moodle.Root+string(os.PathSeparator)) {
		return fmt.Errorf("unsafe install path %q", relpath)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists — %s is already installed", relpath, component)
	}

	logf("Installing into " + relpath)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err := os.Rename(checkout, dest); err != nil {
		return fmt.Errorf("moving the plugin into the tree: %w", err)
	}

	// Record the checkout in .mudev.json (origin + ref), so backups reproduce it.
	logf("Recording the plugin with mudev")
	if err := execx.Run(logf, moodle.Root, "mudev", "recipe", "update", relpath); err != nil {
		return fmt.Errorf("mudev recipe update: %w", err)
	}

	logf("Upgrading the site to install the plugin")
	if err := moodle.Upgrade(logf); err != nil {
		return err
	}
	if err := moodle.PurgeCaches(logf); err != nil {
		logf("warning: purging caches failed: " + err.Error())
	}
	logf("")
	logf("Plugin " + component + " added.")
	return nil
}
