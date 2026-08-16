// mdl-demo is a dual-purpose binary inside the mdl-demo container:
//
//  1. CLI tool to manage the single demo site (recipes, install, status, reset).
//  2. Management web UI on port 8081 (`mdl-demo serve`), run as root under systemd.
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
  serve     run only the management web UI on port 8081 (development)
  init      run as PID 1: supervise all services and the web UI (the
            container's entrypoint)
  recipes   list available site recipes from /srv/extra/mdl-recipes
  install   install the demo site from a recipe
  status    show demo site status
  reset     wipe the demo site (database, code tree, data)
  cron      run Moodle cron for the installed site (used by moodle-cron.service)
  version   print the mdl-demo version
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
	fs.StringVar(&o.Fullname, "fullname", "", "site full name (default: the recipe's name)")
	fs.StringVar(&o.Wwwroot, "wwwroot", "", "site URL as the browser sees it (default http://localhost:8080)")
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

func serve() error {
	return webui.Serve(os.Stdout, version)
}
