package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSuggestName(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 30, 0, 0, time.UTC)
	cases := []struct{ in, want string }{
		{"Moje škola", "Moje-skola-20260830-153000.mdb"},
		{"mpd test", "mpd-test-20260830-153000.mdb"},
		{"Über Müller & Söhne!", "Uber-Muller-Sohne-20260830-153000.mdb"},
		{"", "demo-20260830-153000.mdb"},
		{"ab", "demo-20260830-153000.mdb"},
		{"日本語", "demo-20260830-153000.mdb"},
		{"a - b", "a-b-20260830-153000.mdb"},
	}
	for _, c := range cases {
		if got := SuggestName(c.in, now); got != c.want {
			t.Errorf("SuggestName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCheckName(t *testing.T) {
	good := []string{"demo-20260830-153000.mdb", "a.mdb", "My_Site.v2.mdb"}
	for _, n := range good {
		if err := CheckName(n); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", n, err)
		}
	}
	bad := []string{
		"", "x.tgz", ".mdb", "..mdb", ".hidden.mdb", ".upload-1.partial",
		"a/b.mdb", "../x.mdb", "sub/../a.mdb", "a b.mdb", "příliš.mdb",
	}
	for _, n := range bad {
		if err := CheckName(n); err == nil {
			t.Errorf("CheckName(%q) = nil, want error", n)
		}
	}
}

// entry is a test archive member.
type entry struct {
	name string
	typ  byte
	body string
}

func writeArchive(t *testing.T, path string, entries []entry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0644}
		if e.typ == tar.TypeDir {
			hdr.Mode = 0755
		}
		if e.typ == tar.TypeSymlink {
			hdr.Linkname = "/etc/passwd"
		}
		if e.typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

const goodMeta = `{"revision":2,"recipe":"moodle/release/5.2.2","created":"2026-08-30T15:30:00Z","db":"postgres:17"}`

func goodEntries() []entry {
	return []entry{
		{MetaName, tar.TypeReg, goodMeta},
		{RecipeName, tar.TypeReg, "name: test\n"},
		{DBName, tar.TypeReg, "SELECT 1;\n"},
		{DataPrefix, tar.TypeDir, ""},
		{DataPrefix + "/filedir", tar.TypeDir, ""},
		{DataPrefix + "/filedir/x.bin", tar.TypeReg, "data"},
	}
}

// rev1Entries is a revision-1 archive: its dataroot lives under demo/, the
// name mdl-demo used before revision 2. Such backups exist in the wild.
func rev1Entries() []entry {
	return []entry{
		{MetaName, tar.TypeReg, `{"revision":1,"recipe":"moodle/release/5.2.2","created":"2026-08-30T15:30:00Z"}`},
		{RecipeName, tar.TypeReg, "name: test\n"},
		{DBName, tar.TypeReg, "SELECT 1;\n"},
		{DataPrefixV1, tar.TypeDir, ""},
		{DataPrefixV1 + "/filedir", tar.TypeDir, ""},
		{DataPrefixV1 + "/filedir/x.bin", tar.TypeReg, "data"},
	}
}

func TestValidateGood(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.mdb")
	writeArchive(t, p, goodEntries())
	meta, err := Validate(p)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if meta.Recipe != "moodle/release/5.2.2" {
		t.Errorf("meta.Recipe = %q", meta.Recipe)
	}
	if m, err := ReadMeta(p); err != nil || m.Revision != 2 {
		t.Errorf("ReadMeta = %+v, %v", m, err)
	}
	// A revision-1 archive (demo/ prefix) still validates.
	p1 := filepath.Join(t.TempDir(), "rev1.mdb")
	writeArchive(t, p1, rev1Entries())
	if _, err := Validate(p1); err != nil {
		t.Fatalf("Validate(rev1): %v", err)
	}
}

func TestValidateHostile(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]entry{
		"meta not first":   append([]entry{{RecipeName, tar.TypeReg, "x"}}, goodEntries()...),
		"missing db.sql":   {{MetaName, tar.TypeReg, goodMeta}, {RecipeName, tar.TypeReg, "x"}},
		"traversal":        append(goodEntries(), entry{"../evil", tar.TypeReg, "x"}),
		"nested traversal": append(goodEntries(), entry{DataPrefix + "/../../evil", tar.TypeReg, "x"}),
		"absolute":         append(goodEntries(), entry{"/etc/evil", tar.TypeReg, "x"}),
		"symlink":          append(goodEntries(), entry{DataPrefix + "/link", tar.TypeSymlink, ""}),
		"stray entry":      append(goodEntries(), entry{"other/file", tar.TypeReg, "x"}),
		"bad revision":     {{MetaName, tar.TypeReg, `{"revision":99}`}, {RecipeName, tar.TypeReg, "x"}, {DBName, tar.TypeReg, "x"}},
	}
	for name, entries := range cases {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".mdb")
		writeArchive(t, p, entries)
		if _, err := Validate(p); err == nil {
			t.Errorf("Validate(%s): no error, want one", name)
		}
	}
	if _, err := Validate(filepath.Join(dir, "absent.mdb")); err == nil {
		t.Error("Validate(absent): no error")
	}
	notGz := filepath.Join(dir, "notgz.mdb")
	os.WriteFile(notGz, []byte("plain"), 0600)
	if _, err := Validate(notGz); err == nil {
		t.Error("Validate(not gzip): no error")
	}
}

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.mdb")
	writeArchive(t, p, goodEntries())

	dest := filepath.Join(dir, RecipeName)
	if err := ExtractFile(p, RecipeName, dest); err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "name: test\n" {
		t.Errorf("recipe.yaml content = %q", b)
	}

	// ExtractData writes into the destination dataroot, stripping the archive
	// prefix — so dataroot/filedir/x.bin lands at <droot>/filedir/x.bin.
	droot := filepath.Join(dir, "myroot")
	if err := os.Mkdir(droot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ExtractData(p, droot); err != nil {
		t.Fatalf("ExtractData: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(droot, "filedir", "x.bin")); string(b) != "data" {
		t.Errorf("extracted file content = %q", b)
	}
	// Top-level members must not leak into the data directory.
	if _, err := os.Stat(filepath.Join(droot, DBName)); err == nil {
		t.Error("db.sql extracted into the data directory")
	}

	// A revision-1 archive (demo/ prefix) extracts into the same destination,
	// with the prefix stripped — a backup restores whatever the dataroot is named.
	p1 := filepath.Join(dir, "rev1.mdb")
	writeArchive(t, p1, rev1Entries())
	dest1 := filepath.Join(dir, "myroot1")
	if err := os.Mkdir(dest1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ExtractData(p1, dest1); err != nil {
		t.Fatalf("ExtractData(rev1): %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest1, "filedir", "x.bin")); string(b) != "data" {
		t.Errorf("rev1 extracted file content = %q", b)
	}
}

func TestSourceDB(t *testing.T) {
	cases := []struct {
		db         string
		wantEngine string
		wantMajor  int
	}{
		{"", "postgres", 17}, // revision-1 default
		{"postgres:17", "postgres", 17},
		{"postgres:18", "postgres", 18},
		{"mariadb:11.4", "mariadb", 11},
		{"mysql:8", "mysql", 8},
		{"postgres", "postgres", 0}, // no version
	}
	for _, c := range cases {
		engine, major := Meta{DB: c.db}.SourceDB()
		if engine != c.wantEngine || major != c.wantMajor {
			t.Errorf("SourceDB(%q) = %q,%d; want %q,%d", c.db, engine, major, c.wantEngine, c.wantMajor)
		}
	}
}

func TestRelinkPublic(t *testing.T) {
	in := `base:
  source:
    git:
      remotes:
        origin: git@github.com:mutms/patches.git
        forge: git@forge.mpd.test:mutms/patches.git
        mirror: file:///srv/extra/repos/github.com/moodle/moodle
plugins:
  - source: {git: {remotes: {origin: ssh://git@gitlab.com/acme/mod_thing.git}}}
  - source: {git: {remotes: {origin: file:///srv/extra/repos/gitlab.com/acme/mod_two}}}
  - source: {git: {remotes: {origin: https://github.com/moodle/moodle.git}}}
`
	out := string(RelinkPublic([]byte(in)))
	wantContains := []string{
		"origin: https://github.com/mutms/patches.git",  // scp github
		"mirror: https://github.com/moodle/moodle",      // file:// github mirror
		"origin: https://gitlab.com/acme/mod_thing.git", // ssh:// gitlab
		"origin: https://gitlab.com/acme/mod_two",       // file:// gitlab mirror
		"origin: https://github.com/moodle/moodle.git",  // already https, unchanged
		"forge: git@forge.mpd.test:mutms/patches.git",   // non-public host untouched
	}
	for _, w := range wantContains {
		if !strings.Contains(out, w) {
			t.Errorf("RelinkPublic output missing %q\ngot:\n%s", w, out)
		}
	}
	if strings.Contains(out, "git@github.com") || strings.Contains(out, "ssh://") ||
		strings.Contains(out, "file:///srv/extra/repos/github.com") ||
		strings.Contains(out, "file:///srv/extra/repos/gitlab.com") {
		t.Errorf("RelinkPublic left a private URL:\n%s", out)
	}
}
