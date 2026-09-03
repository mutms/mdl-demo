// Package moodle knows the demo site's code tree: layout detection across
// the Moodle 5.1 public/ split, config.php generation, and running Moodle
// CLI scripts.
//
// Layout facts (verified against MOODLE_501/502_STABLE and mpd's tooling):
// config-dist.php, lib/setup.php and admin/cli/* live at the repo ROOT in
// every supported version; 5.1+ adds public/ as the web docroot and moves
// some admin tools under it. So config.php always goes at the root, the
// docroot varies, and each CLI script path is resolved individually.
package moodle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

const (
	// Root is where mudev assembles the code tree.
	Root = "/srv/projects/demo"
	// Dataroot lives outside the webroot and is owned by www-data.
	Dataroot = "/srv/data/demo"
)

// Detected reports whether a Moodle tree is present.
func Detected() bool {
	return exists(filepath.Join(Root, "version.php")) ||
		exists(filepath.Join(Root, "public", "version.php"))
}

// Docroot is what Apache serves: public/ when the tree has the 5.1+ split.
func Docroot() string {
	if exists(filepath.Join(Root, "public", "version.php")) {
		return filepath.Join(Root, "public")
	}
	return Root
}

// HasRouter reports whether the docroot ships r.php (Moodle 5.0+); the
// Apache rewrite to /r.php must not be emitted without it or unknown URLs
// loop instead of 404ing.
func HasRouter() bool {
	return exists(filepath.Join(Docroot(), "r.php"))
}

// resolveDir returns the directory to run a CLI script from: the root, or
// public/ for the tools that moved there in 5.1.
func resolveDir(rel string) (string, error) {
	if exists(filepath.Join(Root, rel)) {
		return Root, nil
	}
	if exists(filepath.Join(Root, "public", rel)) {
		return filepath.Join(Root, "public"), nil
	}
	return "", fmt.Errorf("%s not found under %s or %s/public", rel, Root, Root)
}

// RunCLI runs a Moodle CLI script as www-data — the same user php-fpm runs
// as, so everything either writes into the dataroot is usable by the other.
func RunCLI(logf execx.Logf, rel string, args ...string) error {
	return runCLI(logf, nil, rel, args...)
}

// runCLI is RunCLI with secret values masked in the echoed command line, for a
// script invoked with a password on its argv.
func runCLI(logf execx.Logf, secrets []string, rel string, args ...string) error {
	dir, err := resolveDir(rel)
	if err != nil {
		return err
	}
	cmd := append([]string{"-u", "www-data", "--", "php", rel}, args...)
	return execx.RunSecret(logf, secrets, dir, "runuser", cmd...)
}

// InstallDatabase runs Moodle's CLI installer against the already-written
// config.php. Flags proven by mpd's install tooling. The admin email is
// deliberately hardcoded — a demo site never sends mail (noemailever).
// A non-"en" lang makes the installer download the language pack, set it as
// the site default and create the admin account in it; on a download failure
// it warns and continues in English.
func InstallDatabase(logf execx.Logf, fullname, shortname, adminpass, lang string) error {
	if lang == "" {
		lang = "en"
	}
	return runCLI(logf, []string{adminpass}, "admin/cli/install_database.php",
		"--agree-license",
		"--lang="+lang,
		"--fullname="+fullname,
		"--shortname="+shortname,
		"--summary=mdl-demo site",
		"--adminpass="+adminpass,
		"--adminemail=admin@example.com",
	)
}

// Cron runs Moodle cron (used by moodle-cron.service via `mdl-demo cron`).
func Cron(logf execx.Logf) error {
	return RunCLI(logf, "admin/cli/cron.php")
}

// Upgrade runs Moodle's CLI upgrade — after a restore it brings the data up
// to whatever version the code tree is.
func Upgrade(logf execx.Logf) error {
	return RunCLI(logf, "admin/cli/upgrade.php", "--non-interactive")
}

// PurgeCaches purges all Moodle caches, which hold absolute URLs and schema
// state that would otherwise outlive a wwwroot change or a restore.
func PurgeCaches(logf execx.Logf) error {
	return RunCLI(logf, "admin/cli/purge_caches.php")
}

// Maintenance switches Moodle's config-level maintenance mode (the setting
// lives in the database). Not used to guard backup/restore — the Apache
// placeholder does that; this only lifts a maintenance flag a restored dump
// may carry.
func Maintenance(logf execx.Logf, enable bool) error {
	flag := "--disable"
	if enable {
		flag = "--enable"
	}
	return RunCLI(logf, "admin/cli/maintenance.php", flag)
}

// SetPassword sets a known account's password via php/cli/setpassword.php
// (which warns and skips a user that no longer exists).
func SetPassword(logf execx.Logf, username, password string) error {
	return runCLI(logf, []string{password}, "mdl-demo/cli/setpassword.php",
		"--username="+username, "--password="+password)
}

// sslproxy returns the config line for an https wwwroot: TLS always ends
// outside this container (a reverse proxy or a tunnel speaks https to the
// browser and plain http to Apache), which is exactly what $CFG->sslproxy
// tells Moodle. Without it, Moodle sees an http request for an https wwwroot
// and redirects in a loop.
func sslproxy(wwwroot string) string {
	if strings.HasPrefix(wwwroot, "https://") {
		return "$CFG->sslproxy = true; // TLS is terminated in front of the container\n"
	}
	return ""
}

// WriteConfig generates config.php at the tree root. Regenerated on every
// install; the demo site is machine-owned so there is no user-editable half
// (unlike mpd's config.php / config-mpd.php split).
func WriteConfig(wwwroot string) error {
	config := `<?php
// config.php — generated by mdl-demo, regenerated on install/reset.
// Custom settings will be lost; this is a disposable demo site.

unset($CFG);
global $CFG;
$CFG = new stdClass();

$CFG->dbtype    = 'pgsql';
$CFG->dblibrary = 'native';
$CFG->dbhost    = '127.0.0.1';
$CFG->dbname    = 'demo';
$CFG->dbuser    = 'demo';
$CFG->dbpass    = 'demo';
$CFG->prefix    = 'mdl_';
$CFG->dboptions = array(
    'dbpersist' => 0,
);

$CFG->wwwroot  = '` + wwwroot + `';
` + sslproxy(wwwroot) + `
// Apache only ever sees loopback (port mapping, cloudflared); real client
// addresses arrive in X-Forwarded-For. 1 = skip HTTP_CLIENT_IP only.
$CFG->getremoteaddrconf = 1;

$CFG->dataroot = '` + Dataroot + `';
$CFG->directorypermissions = 02777;

// Router (Moodle 5.0+): Apache rewrites unknown URLs to /r.php, so clean
// URLs work; telling Moodle silences the environment check.
$CFG->routerconfigured = true;

$CFG->admin = 'admin';

// The code tree is root-owned and the web server only reads it, so web-based
// plugin installation cannot work; this makes Moodle hide those affordances
// instead of failing on them. Plugins come in via recipes/mudev only.
$CFG->disableupdateautodeploy = true;

// Demo failsafes: all outgoing mail lands in the built-in Mailpit catcher
// (console → Mail) — nothing ever leaves the container, and teachers see
// exactly what Moodle would send. site_is_public=false blocks site
// registration and other public outreach.
$CFG->smtphosts = '127.0.0.1:1025';
$CFG->noreplyaddress = 'noreply@example.com';
$CFG->site_is_public = false;

$CFG->cronclionly = 0;
$CFG->cron_keepalive = '0';

require_once(__DIR__ . '/lib/setup.php'); // Do not edit
`
	return os.WriteFile(filepath.Join(Root, "config.php"), []byte(config), 0644)
}

// PHPShare is where the image keeps the console's Moodle-side PHP (the repo's
// php/ dir): web endpoints at the top, CLI scripts under cli/.
const PHPShare = "/usr/share/mdl-demo/php"

// InstallPHP copies the console's PHP into the docroot as <docroot>/mdl-demo/
// — root-owned like the rest of the tree, so the scripts are as immutable to
// the web server as Moodle itself.
func InstallPHP(logf execx.Logf) error {
	return execx.Run(logf, "", "cp", "-r", PHPShare, filepath.Join(Docroot(), "mdl-demo"))
}

// CreateUser creates a demo account via php/cli/createuser.php. role is "",
// "manager" (system-context Manager) or "admin" (site administrator).
func CreateUser(logf execx.Logf, username, password, firstname, lastname, role string) error {
	args := []string{
		"--username=" + username,
		"--password=" + password,
		"--firstname=" + firstname,
		"--lastname=" + lastname,
	}
	if role != "" {
		args = append(args, "--role="+role)
	}
	return runCLI(logf, []string{password}, "mdl-demo/cli/createuser.php", args...)
}

// Plugin is one additional (non-core) plugin as reported by
// mdl-demo/cli/export_plugins.php. Versions come through as json.Number so a
// missing (null) versiondb stays distinguishable from 0.
type Plugin struct {
	Component   string      `json:"component"`
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	DisplayName string      `json:"displayname"`
	Relpath     string      `json:"relpath"`
	VersionDisk json.Number `json:"versiondisk"`
	VersionDB   json.Number `json:"versiondb"`
	Release     string      `json:"release"`
	Status      string      `json:"status"`
}

// ExportPlugins lists the site's additional plugins by running the read-only
// export_plugins.php as www-data and parsing its JSON. Returns nil on an empty
// site; an error only when the script cannot run or its output is not JSON.
func ExportPlugins() ([]Plugin, error) {
	const rel = "mdl-demo/cli/export_plugins.php"
	dir, err := resolveDir(rel)
	if err != nil {
		return nil, err
	}
	out, err := execx.Output(dir, "runuser", "-u", "www-data", "--", "php", rel)
	if err != nil {
		return nil, err
	}
	var plugins []Plugin
	if err := json.Unmarshal([]byte(out), &plugins); err != nil {
		return nil, fmt.Errorf("parsing plugin list: %w", err)
	}
	return plugins, nil
}

// PluginSource is one checkout as recorded in the tree's live recipe
// (.mudev.json): where it sits and the browsable URL of its origin repo.
// Relpath is normalized tree-relative to the Moodle root — the leading public/
// of the 5.1 split is stripped so it lines up with a plugin's rootdir-relative
// path from ExportPlugins.
type PluginSource struct {
	Relpath string
	URL     string
}

// PluginSources reads the tree's .mudev.json and returns one entry per recorded
// checkout with a resolvable git origin. Returns nil (no error) when the file is
// absent — a site assembled another way simply has no source links.
func PluginSources() ([]PluginSource, error) {
	data, err := os.ReadFile(filepath.Join(Root, ".mudev.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var recipe struct {
		Plugins []struct {
			Relpath string `json:"relpath"`
			Source  struct {
				Git struct {
					Remotes map[string]string `json:"remotes"`
				} `json:"git"`
			} `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		return nil, fmt.Errorf("parsing .mudev.json: %w", err)
	}
	var out []PluginSource
	for _, p := range recipe.Plugins {
		url := browseURL(p.Source.Git.Remotes["origin"])
		if url == "" {
			continue
		}
		out = append(out, PluginSource{
			Relpath: strings.TrimPrefix(p.Relpath, "public/"),
			URL:     url,
		})
	}
	return out, nil
}

// browseURL turns a git remote into a browsable https URL for the common
// GitHub/GitLab hosts, covering https and scp-style (git@host:owner/repo)
// remotes. It returns "" for anything it does not recognize rather than emit a
// link that might not resolve.
func browseURL(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	remote = strings.TrimSuffix(remote, ".git")
	// scp-style: git@github.com:owner/repo → github.com/owner/repo
	if strings.HasPrefix(remote, "git@") {
		if _, hostpath, ok := strings.Cut(remote, "@"); ok {
			remote = "https://" + strings.Replace(hostpath, ":", "/", 1)
		}
	}
	remote = strings.TrimPrefix(remote, "https://")
	remote = strings.TrimPrefix(remote, "http://")
	for _, host := range []string{"github.com/", "gitlab.com/"} {
		if strings.HasPrefix(remote, host) {
			return "https://" + remote
		}
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
