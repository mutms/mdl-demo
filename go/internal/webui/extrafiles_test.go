package webui

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

const fileTestToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 zero bytes, base64

// extraFilesTest points the handlers at a temporary folder, with a secret beside it
// that nothing may reach.
func extraFilesTest(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "extrafiles")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0644); err != nil {
		t.Fatal(err)
	}
	origDir, origChown := extraFilesDir, chownExtraFile
	extraFilesDir, chownExtraFile = dir, func(*os.File) error { return nil }
	t.Cleanup(func() { extraFilesDir, chownExtraFile = origDir, origChown })
	return cliTestServer(t), dir, secret
}

func fileRequest(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	r.Host = "127.0.0.1:8081"
	r.AddCookie(&http.Cookie{Name: csrfCookie, Value: fileTestToken})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func upload(h http.Handler, name, body, token string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("csrf", token)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write([]byte(body))
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/settings/files/upload", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return fileRequest(h, r)
}

func download(h http.Handler, name string) *httptest.ResponseRecorder {
	return fileRequest(h, httptest.NewRequest("GET", "/settings/files/download?name="+url.QueryEscape(name), nil))
}

func TestExtraFilesRoundTrip(t *testing.T) {
	h, dir, _ := extraFilesTest(t)
	if w := upload(h, "plugin.zip", "zipped", fileTestToken); w.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("the temporary file stayed behind: %v", entries)
	}
	w := download(h, "plugin.zip")
	if w.Code != http.StatusOK || w.Body.String() != "zipped" {
		t.Fatalf("download: %d %q", w.Code, w.Body)
	}
	if got, _ := listExtraFiles(); len(got) != 1 || got[0].Name != "plugin.zip" {
		t.Fatalf("listing: %v", got)
	}
	if w := upload(h, "plugin.zip", "other", fileTestToken); w.Code != http.StatusConflict {
		t.Errorf("a second upload must not replace the file: %d", w.Code)
	}
	form := url.Values{"csrf": {fileTestToken}, "name": {"plugin.zip"}}
	r := httptest.NewRequest("POST", "/settings/files/delete", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := fileRequest(h, r); w.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", w.Code)
	}
	if got, _ := listExtraFiles(); len(got) != 0 {
		t.Errorf("still listed after delete: %v", got)
	}
}

// A page in the folder must never open on the console's origin.
func TestExtraFilesDownloadIsAnAttachment(t *testing.T) {
	h, dir, _ := extraFilesTest(t)
	if err := os.WriteFile(filepath.Join(dir, `x".html`), []byte("<script>"), 0644); err != nil {
		t.Fatal(err)
	}
	w := download(h, `x".html`)
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("Content-Disposition = %q", w.Header().Get("Content-Disposition"))
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q", got)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
}

// www-data controls the folder and this runs as root: planted links must lead nowhere.
func TestExtraFilesRefusesPlantedFiles(t *testing.T) {
	h, dir, secret := extraFilesTest(t)
	if err := os.Symlink(secret, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../secret", filepath.Join(dir, "rel")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link", "rel", "../secret"} {
		if w := download(h, name); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "TOP-SECRET") {
			t.Errorf("download %q: %d %q", name, w.Code, w.Body)
		}
	}
	// An upload onto a link must not write through it.
	if w := upload(h, "link", "OVERWRITTEN", fileTestToken); w.Code == http.StatusSeeOther {
		t.Error("an upload replaced a link")
	}
	if data, _ := os.ReadFile(secret); string(data) != "TOP-SECRET" {
		t.Errorf("the secret was overwritten: %q", data)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0644); err != nil {
		t.Fatal(err)
	}
	if w := download(h, "fifo"); w.Code != http.StatusNotFound {
		t.Errorf("fifo: %d", w.Code)
	}
	if got, _ := listExtraFiles(); len(got) != 0 {
		t.Errorf("non-files must not be listed: %v", got)
	}
}

func TestExtraFilesNames(t *testing.T) {
	for name, ok := range map[string]bool{
		"plugin.zip": true, "my plugin (1).zip": true, "žluťoučký.zip": true,
		"": false, ".": false, "..": false, ".hidden": false, "a/b": false, `a\b`: false, "a\nb": false,
	} {
		if err := checkExtraFileName(name); (err == nil) != ok {
			t.Errorf("checkExtraFileName(%q) = %v", name, err)
		}
	}
}

func TestExtraFilesGuards(t *testing.T) {
	h, _, _ := extraFilesTest(t)
	if w := upload(h, "x.zip", "x", "wrong"); w.Code != http.StatusForbidden {
		t.Errorf("wrong token: %d", w.Code)
	}
	r := httptest.NewRequest("POST", "/settings/files/delete?name=x", nil)
	if w := fileRequest(h, r); w.Code != http.StatusForbidden {
		t.Errorf("delete without a token: %d", w.Code)
	}
	orig := cliEnabled
	cliEnabled = func() bool { return false }
	t.Cleanup(func() { cliEnabled = orig })
	if w := upload(h, "x.zip", "x", fileTestToken); w.Code != http.StatusNotFound {
		t.Errorf("upload with the CLI off: %d", w.Code)
	}
	if w := download(h, "x.zip"); w.Code != http.StatusNotFound {
		t.Errorf("download with the CLI off: %d", w.Code)
	}
}

func TestExtraFilesTree(t *testing.T) {
	h, dir, secret := extraFilesTest(t)
	for _, f := range []string{"top.zip", "plugin/version.php", "plugin/lang/en/x.php", "plugin/.git/config", ".hidden"} {
		full := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Links stay unlisted and unfollowed, wherever they point and however deep.
	if err := os.Symlink(filepath.Dir(secret), filepath.Join(dir, "plugin", "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "plugin", "stolen")); err != nil {
		t.Fatal(err)
	}
	rows, truncated := listExtraFiles()
	if truncated {
		t.Error("a small tree must not be cut short")
	}
	var got []string
	for _, r := range rows {
		got = append(got, strings.Repeat(">", r.Depth)+r.Path)
	}
	want := []string{"plugin", ">plugin/lang", ">>plugin/lang/en", ">>>plugin/lang/en/x.php", ">plugin/version.php", "top.zip"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("tree = %v, want %v", got, want)
	}
	w := download(h, "plugin/lang/en/x.php")
	if w.Code != http.StatusOK || w.Body.String() != "data" {
		t.Fatalf("nested download: %d %q", w.Code, w.Body)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "filename=x.php") {
		t.Errorf("Content-Disposition = %q", got)
	}
	for _, name := range []string{"plugin/stolen", "plugin/escape/secret", "plugin/../../secret", "plugin/.git/config", "plugin", "/etc/passwd", "plugin//version.php"} {
		if w := download(h, name); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "TOP-SECRET") {
			t.Errorf("download %q: %d %q", name, w.Code, w.Body)
		}
	}
	remove := func(name string) int {
		form := url.Values{"csrf": {fileTestToken}, "name": {name}}
		r := httptest.NewRequest("POST", "/settings/files/delete", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return fileRequest(h, r).Code
	}
	if c := remove("plugin/version.php"); c != http.StatusSeeOther {
		t.Errorf("nested delete: %d", c)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugin", "version.php")); err == nil {
		t.Error("the nested file is still there")
	}
	if c := remove("plugin/lang"); c == http.StatusSeeOther {
		t.Error("a directory must not be deleted from the console")
	}
	if c := remove("plugin/stolen"); c == http.StatusSeeOther {
		t.Error("a link must not be deleted from the console")
	}
	if data, _ := os.ReadFile(secret); string(data) != "TOP-SECRET" {
		t.Errorf("the secret changed: %q", data)
	}
}

func TestExtraFilesListingIsCapped(t *testing.T) {
	_, dir, _ := extraFilesTest(t)
	for i := range maxExtraFileRows + 5 {
		if err := os.WriteFile(filepath.Join(dir, "f"+strconv.Itoa(i)), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	rows, truncated := listExtraFiles()
	if len(rows) != maxExtraFileRows || !truncated {
		t.Errorf("rows = %d, truncated = %v", len(rows), truncated)
	}
}
