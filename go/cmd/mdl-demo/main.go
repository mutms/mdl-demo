// mdl-demo is a dual-purpose binary inside the mdl-demo container:
//
//  1. CLI tool to manage the single demo site (recipes, install, status, reset).
//  2. Management web UI on container port 8081 (`mdl-demo serve`), normally
//     run in-process by `mdl-demo init`, the container's PID 1.
//
// One demo site per container: a different Moodle version means a new
// container. All paths are therefore fixed (/srv/projects/demo, /srv/data/demo,
// database "demo").
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mutms/mdl-demo/go/internal/initd"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/pgdb"
	"github.com/mutms/mdl-demo/go/internal/recipes"
	"github.com/mutms/mdl-demo/go/internal/site"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/webui"
)

// version is stamped at build time via -ldflags.
var version = "dev"

const usage = `mdl-demo — Moodle/MuTMS demo site manager

Usage:
  mdl-demo <command> [flags]

Commands:
  serve     run only the management web UI on container port 8081 (development)
  init      run as PID 1: supervise all services and the web UI (the
            container's entrypoint)
  recipes   list available site recipes from /srv/extra/mdl-recipes
  install   install the demo site from a recipe
  status    show demo identity and site status
  reset     wipe the demo site (database, code tree, data)
  backup    back up the site (database, data, code recipe) into /srv/backups
  restore   replace the site with a backup, optionally into another recipe
  url       show the URLs, or override the site's one (for proxies and tunnels)
  cron      run Moodle cron for the installed site (the init's per-minute ticker)
  version   print the mdl-demo version

Ports: the console listens on 8081 and the site on 8082 inside the container.
Outside, the console port (MDL_DEMO_PORT, default 8081) is the demo's identity
and the site is always on the next port — map NNNN:8081 and NNNN+1:8082.
`

// stdoutLog streams orchestration output for CLI use.
func stdoutLog(line string) { fmt.Println(line) }

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch cmd := os.Args[1]; cmd {
	case "version", "--version", "-v":
		fmt.Println("mdl-demo " + version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	case "serve":
		err = serve()
	case "init":
		err = initd.Run(version)
	case "recipes":
		err = cmdRecipes()
	case "install":
		err = cmdInstall(os.Args[2:])
	case "status":
		err = cmdStatus()
	case "reset":
		err = site.Reset(stdoutLog)
	case "backup":
		err = cmdBackup()
	case "restore":
		err = cmdRestore(os.Args[2:])
	case "url":
		err = cmdURL(os.Args[2:])
	case "cron":
		err = cmdCron()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mdl-demo: %v\n", err)
		os.Exit(1)
	}
}

func cmdRecipes() error {
	list, err := recipes.List()
	if err != nil {
		return err
	}
	for _, r := range list {
		fmt.Printf("%-28s  %s\n", r.ID, r.Name)
	}
	return nil
}

func cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	var o site.Options
	fs.StringVar(&o.Recipe, "recipe", "", "recipe identifier, e.g. mutms/release/5.2.2.01 (see `mdl-demo recipes`)")
	fs.StringVar(&o.AdminPass, "adminpass", "", "Moodle admin password (required)")
	fs.StringVar(&o.Fullname, "fullname", "", "site full name (default: the demo name, else the recipe's name)")
	fs.StringVar(&o.Shortname, "shortname", "", "site short name (default: the demo name, else \"demo\")")
	fs.StringVar(&o.Wwwroot, "wwwroot", "", "site URL as the browser sees it (default: the site URL from `mdl-demo url` for 127.0.0.1)")
	fs.StringVar(&o.Lang, "lang", "", "site default language, e.g. cs or de: installs the language pack (default: English)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return site.Install(stdoutLog, o)
}

func cmdBackup() error {
	_, err := site.Backup(stdoutLog, version)
	return err
}

func cmdRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	var o site.RestoreOptions
	fs.StringVar(&o.Recipe, "recipe", "", "restore into this catalogue recipe instead of the backup's own (the upgrade path)")
	fs.StringVar(&o.Wwwroot, "wwwroot", "", "site URL as the browser sees it (default: the site URL from `mdl-demo url` for 127.0.0.1)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: mdl-demo restore [flags] <file.mdb> (files live in /srv/backups)")
	}
	o.File = fs.Arg(0)
	return site.Restore(stdoutLog, o)
}

func cmdStatus() error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	fmt.Printf("demo:       %s\n", s.Title())
	fmt.Printf("console:    %s\n", s.ConsoleURLFor(state.DefaultHost))
	fmt.Printf("site url:   %s\n", s.SiteURLFor(state.DefaultHost))
	if !s.Installed() {
		fmt.Println("no demo site installed")
		return nil
	}
	fmt.Printf("recipe:     %s\n", s.Recipe)
	fmt.Printf("wwwroot:    %s\n", s.Wwwroot)
	if s.AdminPass != "" {
		fmt.Printf("log in as:  admin / %s\n", s.AdminPass)
	}
	fmt.Printf("installed:  %s\n", s.InstalledAt.Format(time.RFC3339))
	if count, err := pgdb.TableCount(); err == nil {
		fmt.Printf("db tables:  %d\n", count)
	}
	return nil
}

func cmdCron() error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	if !s.Installed() || !moodle.Detected() {
		return nil // nothing to do on a fresh container
	}
	return moodle.Cron(stdoutLog) // journald captures the output
}

// cmdURL shows the URLs the console believes it and the site are reachable
// at, and records an override for the site when something sits in front of
// the container (a reverse proxy, a tunnel) so the install form suggests the
// right URL. The override lives in state.json: temporary like the container,
// cleared with --clear. Moodle bakes wwwroot in at install, so changing the
// site URL afterwards does not move an installed site.
//
// Only the site can be overridden. The console answers to loopback and IP
// addresses only (see internal/webui/auth.go) — it is a local port, and a
// setting that pointed it at a public name would just invite exposing it.
func cmdURL(args []string) error {
	fs := flag.NewFlagSet("url", flag.ExitOnError)
	siteURL := fs.String("site", "", "public Moodle site URL, e.g. https://site.demo.example.test")
	clear := fs.Bool("clear", false, "drop the override (back to the port-derived URL)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := state.Load()
	if err != nil {
		return err
	}
	changed := false
	if *clear {
		s.SiteURL = ""
		changed = true
	}
	if *siteURL != "" {
		if s.SiteURL, err = site.NormalizeURL(*siteURL); err != nil {
			return fmt.Errorf("--site: %w", err)
		}
		changed = true
	}
	if changed {
		if err := s.Save(); err != nil {
			return err
		}
	}
	fmt.Printf("console:  %s\n", s.ConsoleURLFor(state.DefaultHost))
	fmt.Printf("site:     %s\n", s.SiteURLFor(state.DefaultHost))
	if s.Installed() && s.Wwwroot != s.SiteURLFor(state.DefaultHost) {
		fmt.Printf("note: the installed site keeps wwwroot %s until it is reset and reinstalled\n", s.Wwwroot)
	}
	return nil
}

func serve() error {
	return webui.Serve(os.Stdout, version)
}
