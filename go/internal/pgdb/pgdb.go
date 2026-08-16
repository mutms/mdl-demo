// Package pgdb provisions the demo site's PostgreSQL role and database on
// the image's local cluster, via `runuser -u postgres -- psql` (peer auth).
//
// Convention (same dev-only choice mpd documents): dbname = dbuser =
// dbpassword = "demo". The cluster only listens on localhost inside the
// container.
package pgdb

import (
	"strconv"

	"github.com/mutms/mdl-demo/internal/execx"
)

// Name is the database, role and password of the demo site.
const Name = "demo"

func psql(args ...string) []string {
	return append([]string{"-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-qAt"}, args...)
}

// Provision idempotently creates (or re-passwords) the role and creates the
// database if missing. Mirrors mpd's SQL: the role via a DO block (CREATE
// ROLE has no IF NOT EXISTS), the database probed first because CREATE
// DATABASE cannot run inside a DO block.
func Provision(logf execx.Logf) error {
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

// Drop removes the demo database and role. WITH (FORCE) kicks any lingering
// connections (php-fpm workers) instead of failing on them.
func Drop(logf execx.Logf) error {
	if err := execx.Run(logf, "", "runuser", psql("-c",
		`DROP DATABASE IF EXISTS "`+Name+`" WITH (FORCE);`)...); err != nil {
		return err
	}
	return execx.Run(logf, "", "runuser", psql("-c", `DROP ROLE IF EXISTS "`+Name+`";`)...)
}
