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

	"github.com/mutms/mdl-demo/internal/initd"
	"github.com/mutms/mdl-demo/internal/moodle"
	"github.com/mutms/mdl-demo/internal/pgdb"
	"github.com/mutms/mdl-demo/internal/recipes"
	"github.com/mutms/mdl-demo/internal/site"
	"github.com/mutms/mdl-demo/internal/state"
	"github.com/mutms/mdl-demo/internal/webui"
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
  url       show or override the console/site URLs (for proxies and tunnels)
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
	fs.StringVar(&o.Wwwroot, "wwwroot", "", "site URL as the browser sees it (default: the site URL from `mdl-demo url` for localhost)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return site.Install(stdoutLog, o)
}

func cmdStatus() error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	fmt.Printf("demo:       %s\n", s.Title())
	fmt.Printf("console:    %s\n", s.ConsoleURLFor("localhost"))
	fmt.Printf("site url:   %s\n", s.SiteURLFor("localhost"))
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
// at, or records overrides for when something sits in front of the container
// (a reverse proxy, a tunnel) so the install form suggests the right site
// URL. Overrides live in state.json: temporary like the container, cleared
// with --clear. Moodle bakes wwwroot in at install, so changing the site URL
// afterwards does not move an installed site.
func cmdURL(args []string) error {
	fs := flag.NewFlagSet("url", flag.ExitOnError)
	console := fs.String("console", "", "public console URL, e.g. https://demo.example.test")
	siteURL := fs.String("site", "", "public Moodle site URL, e.g. https://site.demo.example.test")
	clear := fs.Bool("clear", false, "drop both overrides (back to the port-derived URLs)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := state.Load()
	if err != nil {
		return err
	}
	changed := false
	if *clear {
		s.ConsoleURL, s.SiteURL = "", ""
		changed = true
	}
	if *console != "" {
		if s.ConsoleURL, err = site.NormalizeURL(*console); err != nil {
			return fmt.Errorf("--console: %w", err)
		}
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
	fmt.Printf("console:  %s\n", s.ConsoleURLFor("localhost"))
	fmt.Printf("site:     %s\n", s.SiteURLFor("localhost"))
	if s.Installed() && s.Wwwroot != s.SiteURLFor("localhost") {
		fmt.Printf("note: the installed site keeps wwwroot %s until it is reset and reinstalled\n", s.Wwwroot)
	}
	return nil
}

func serve() error {
	return webui.Serve(os.Stdout, version)
}
