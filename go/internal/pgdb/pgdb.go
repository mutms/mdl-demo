// Package pgdb provisions the demo site's PostgreSQL role and database on
// the image's local cluster, via `runuser -u postgres -- psql` (peer auth).
//
// Convention (same dev-only choice mpd documents): dbname = dbuser =
// dbpassword = "demo". The cluster only listens on loopback inside the
// container.
package pgdb

import (
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

// Name is the database, role and password of the demo site.
const Name = "demo"

func psql(args ...string) []string {
	return append([]string{"-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-qAt"}, args...)
}

// WaitReady blocks until the local cluster accepts connections — under
// `mdl-demo init` there is no systemd ordering, and after an unclean stop
// postgres spends a few seconds in WAL recovery before listening.
func WaitReady(logf execx.Logf) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := exec.Command("runuser", "-u", "postgres", "--", "pg_isready", "-q").Run(); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgresql not ready after 30s")
		}
		logf("Waiting for PostgreSQL to accept connections…")
		time.Sleep(2 * time.Second)
	}
}

// Provision idempotently creates (or re-passwords) the role and creates the
// database if missing. Mirrors mpd's SQL: the role via a DO block (CREATE
// ROLE has no IF NOT EXISTS), the database probed first because CREATE
// DATABASE cannot run inside a DO block.
func Provision(logf execx.Logf) error {
	if err := WaitReady(logf); err != nil {
		return err
	}
	role := `DO $$ BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + Name + `') THEN
		CREATE ROLE "` + Name + `" LOGIN PASSWORD '` + Name + `';
	ELSE
		ALTER ROLE "` + Name + `" LOGIN PASSWORD '` + Name + `';
	END IF;
END $$;`
	if err := execx.Run(logf, "", "runuser", psql("-c", role)...); err != nil {
		return err
	}
	exists, err := execx.Output("", "runuser", psql("-c",
		`SELECT 1 FROM pg_database WHERE datname = '`+Name+`'`)...)
	if err != nil {
		return err
	}
	if exists == "" {
		if err := execx.Run(logf, "", "runuser", psql("-c",
			`CREATE DATABASE "`+Name+`" OWNER "`+Name+`";`)...); err != nil {
			return err
		}
	}
	return nil
}

// TableCount reports how many tables the demo database holds — nonzero
// means a site is (or was) installed in it.
func TableCount() (int, error) {
	out, err := execx.Output("", "runuser", psql("-d", Name, "-c",
		`SELECT count(*) FROM pg_tables WHERE schemaname = 'public'`)...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// uri connects as the demo role over TCP. Dumps and loads go through it
// rather than peer-auth postgres: the demo role then owns everything a load
// creates (no --no-owner surprises for Moodle), and the process needs no
// runuser, so it can write dump files into root-only staging directories.
// The "password" is the fixed, publicly documented convention — not a secret.
const uri = "postgresql://" + Name + ":" + Name + "@127.0.0.1/" + Name

// DumpTo writes a plain-SQL dump of the demo database to file. Plain format
// on purpose: the backup archive is gzipped as a whole, and plain SQL loads
// with psql alone. The dump goes to a file, never through the line logger —
// it can be gigabytes.
func DumpTo(logf execx.Logf, file string) error {
	if err := WaitReady(logf); err != nil {
		return err
	}
	return execx.Run(logf, "", "pg_dump", "--no-owner", "--no-privileges", "-f", file, uri)
}

// LoadFrom replays a plain-SQL dump into the (freshly provisioned, empty)
// demo database.
func LoadFrom(logf execx.Logf, file string) error {
	return execx.Run(logf, "", "psql", "-v", "ON_ERROR_STOP=1", "-q", "-f", file, uri)
}

// Drop removes the demo database and role. WITH (FORCE) kicks any lingering
// connections (php-fpm workers) instead of failing on them.
func Drop(logf execx.Logf) error {
	if err := execx.Run(logf, "", "runuser", psql("-c",
		`DROP DATABASE IF EXISTS "`+Name+`" WITH (FORCE);`)...); err != nil {
		return err
	}
	return execx.Run(logf, "", "runuser", psql("-c", `DROP ROLE IF EXISTS "`+Name+`";`)...)
}
