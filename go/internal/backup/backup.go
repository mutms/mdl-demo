// Package backup knows the .mdb backup file format — one gzipped tar holding
// the whole demo site: meta.json (always the FIRST entry, so listings read it
// without scanning the archive), recipe.yaml (the tree's live recipe from
// `mudev recipe export`, catalogue-independent), db.sql (plain pg_dump) and
// the dataroot as demo/. Orchestration (what to dump, when to wipe) lives in
// internal/site; this package only reads, validates and extracts archives.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Dir holds the backups inside the container. Deliberately outside the paths
// "Reset site" wipes, so restore-from-empty works after a reset; root-only —
// dumps hold everything the site knows, www-data must not read them.
const Dir = "/srv/backups"

// Archive member names. MetaName must be the first tar entry.
const (
	MetaName   = "meta.json"
	RecipeName = "recipe.yaml"
	DBName     = "db.sql"
	DataPrefix = "demo" // the dataroot, stored as demo/…
)

// Revision is the format revision this build writes and the newest it
// restores.
const Revision = 1

// Meta is the archive's first entry: everything the console needs to list a
// backup and rebuild the site around the restored data. Users carries names
// and role labels only — passwords are regenerated on restore, never stored.
type Meta struct {
	Revision int        `json:"revision"`
	Recipe   string     `json:"recipe,omitempty"`   // catalogue ID, display only
	Fullname string     `json:"fullname,omitempty"` // Moodle site full name
	Created  time.Time  `json:"created"`
	Version  string     `json:"version,omitempty"` // mdl-demo version that wrote it
	Users    []MetaUser `json:"users,omitempty"`
}

// MetaUser is one console-created account recorded in a backup.
type MetaUser struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// EnsureDir creates the backups directory (idempotent; also covers containers
// built from images that predate it).
func EnsureDir() error {
	return os.MkdirAll(Dir, 0700)
}

// nameRe rejects path separators and leading dots: a valid name can never
// collide with the .partial/.staging working files in the same directory.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.mdb$`)

// CheckName validates a backup file name from the outside world (a form value,
// an uploaded file name) before it is used to address anything in Dir.
func CheckName(name string) error {
	if !nameRe.MatchString(name) || filepath.Base(name) != name {
		return fmt.Errorf("not a valid backup file name (letters, digits, . _ -, ending in .mdb)")
	}
	return nil
}

// Path returns the full path of a named backup after validating the name.
func Path(name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	return filepath.Join(Dir, name), nil
}

// SuggestName builds a file name from the site's full name: diacritics folded,
// spaces to dashes, everything else stripped, a too-short remainder replaced
// by "demo", then a timestamp.
func SuggestName(fullname string, now time.Time) string {
	base := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(fullname))
	base = strings.ReplaceAll(base, " ", "-")
	base = regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(base, "")
	base = strings.Trim(regexp.MustCompile(`-+`).ReplaceAllString(base, "-"), "-")
	if len(base) < 3 {
		base = "demo"
	}
	return base + "-" + now.Format("20060102-150405") + ".mdb"
}

// Info is one row of the backups listing. Meta is nil for a file that is not
// a readable known-revision archive — such a file can be downloaded or deleted but
// not restored.
type Info struct {
	Name    string
	Size    int64
	ModTime time.Time
	Meta    *Meta
}

// List returns the backups in Dir, newest first. Working files (leading dot,
// wrong extension) are skipped entirely.
func List() ([]Info, error) {
	if err := EnsureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(Dir)
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if !e.Type().IsRegular() || CheckName(e.Name()) != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		info := Info{Name: e.Name(), Size: fi.Size(), ModTime: fi.ModTime()}
		if m, err := ReadMeta(filepath.Join(Dir, e.Name())); err == nil {
			info.Meta = &m
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// Delete removes a named backup.
func Delete(name string) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// open starts reading an archive: file → gzip → tar.
func open(path string) (*os.File, *tar.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("not a gzip archive: %w", err)
	}
	return f, tar.NewReader(gz), nil
}

// ReadMeta reads the archive's metadata. It only looks at the first tar entry
// — the format puts meta.json there so this stays cheap however large the
// archive is.
func ReadMeta(path string) (Meta, error) {
	f, tr, err := open(path)
	if err != nil {
		return Meta{}, err
	}
	defer f.Close()
	hdr, err := tr.Next()
	if err != nil {
		return Meta{}, fmt.Errorf("reading archive: %w", err)
	}
	if filepath.Clean(hdr.Name) != MetaName {
		return Meta{}, fmt.Errorf("%s is not the first archive entry", MetaName)
	}
	return decodeMeta(tr)
}

func decodeMeta(r io.Reader) (Meta, error) {
	var m Meta
	if err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&m); err != nil {
		return Meta{}, fmt.Errorf("reading %s: %w", MetaName, err)
	}
	if m.Revision < 1 || m.Revision > Revision {
		return Meta{}, fmt.Errorf("unsupported backup format revision %d (this build reads up to %d)", m.Revision, Revision)
	}
	return m, nil
}

// Validate scans the whole archive before anything destructive happens:
// meta.json first and readable, recipe.yaml and db.sql present, and every
// entry a regular file or directory with a safe local name — either one of
// the top-level members or inside demo/. Returns the metadata so callers
// need not read it twice.
func Validate(path string) (Meta, error) {
	f, tr, err := open(path)
	if err != nil {
		return Meta{}, err
	}
	defer f.Close()

	var meta Meta
	seen := map[string]bool{}
	for i := 0; ; i++ {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Meta{}, fmt.Errorf("reading archive: %w", err)
		}
		name := filepath.Clean(hdr.Name)
		if !filepath.IsLocal(name) {
			return Meta{}, fmt.Errorf("unsafe path %q in archive", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		default:
			return Meta{}, fmt.Errorf("unsupported entry type for %q in archive (only files and directories)", hdr.Name)
		}
		if i == 0 {
			if name != MetaName {
				return Meta{}, fmt.Errorf("%s is not the first archive entry", MetaName)
			}
			if meta, err = decodeMeta(tr); err != nil {
				return Meta{}, err
			}
		}
		switch {
		case name == MetaName || name == RecipeName || name == DBName:
			seen[name] = true
		case name == DataPrefix || strings.HasPrefix(name, DataPrefix+"/"):
		default:
			return Meta{}, fmt.Errorf("unexpected entry %q in archive", hdr.Name)
		}
	}
	for _, want := range []string{MetaName, RecipeName, DBName} {
		if !seen[want] {
			return Meta{}, fmt.Errorf("%s missing from archive", want)
		}
	}
	return meta, nil
}

// ExtractFile writes one top-level archive member (recipe.yaml, db.sql) to
// dest.
func ExtractFile(path, member, dest string) error {
	f, tr, err := open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s missing from archive", member)
		}
		if err != nil {
			return err
		}
		if filepath.Clean(hdr.Name) != member {
			continue
		}
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}

// RecipeHash returns the SHA-256 (hex) of the archive's recipe.yaml member —
// the tree's `mudev recipe export --sort` output, which is deterministic (no
// timestamps, sorted), so equal hashes mean the same code recipe. Used to
// decide whether a restore can keep the code already on disk. Reads at most a
// megabyte: a recipe is small, and this caps a malformed archive.
func RecipeHash(path string) (string, error) {
	f, tr, err := open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("%s missing from archive", RecipeName)
		}
		if err != nil {
			return "", err
		}
		if filepath.Clean(hdr.Name) != RecipeName {
			continue
		}
		h := sha256.New()
		if _, err := io.Copy(h, io.LimitReader(tr, 1<<20)); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
}

// ExtractData unpacks the demo/ dataroot entries into dataParent (the
// directory that will contain demo/). Extraction is done in Go inside an
// os.Root so no archive entry can escape dataParent, whatever its name says;
// Validate has already rejected non-file/dir entry types.
func ExtractData(path, dataParent string) error {
	f, tr, err := open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	root, err := os.OpenRoot(dataParent)
	if err != nil {
		return err
	}
	defer root.Close()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe path %q in archive", hdr.Name)
		}
		if name != DataPrefix && !strings.HasPrefix(name, DataPrefix+"/") {
			continue
		}
		mode := os.FileMode(hdr.Mode).Perm()
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
				return err
			}
			out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported entry type for %q in archive", hdr.Name)
		}
	}
}
