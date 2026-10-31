package webui

// The CLI console's exchange folder, on the Settings page: files go in for use in
// the terminal, and what Claude Code made comes out before the demo is deleted.
//
// This console runs as root and www-data controls the folder, so nothing here may
// trust what is in it. os.Root refuses paths that escape the folder, including
// through symlinks; only regular files are served, as attachments, because a page
// opened on this origin could drive the console.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// extraFilesDir is a variable so tests can point it elsewhere.
var extraFilesDir = "/srv/projects/extrafiles"

type extraFile struct {
	Name string
	Size string
}

// checkExtraFileName accepts a plain file name. Dot files stay out of the console:
// they are the shell's own business, and uploads use them while in progress.
func checkExtraFileName(name string) error {
	if name == "" || len(name) > 255 || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\`) ||
		strings.ContainsFunc(name, func(c rune) bool { return c < 32 || c == 127 }) {
		return fmt.Errorf("not a valid file name")
	}
	return nil
}

// listExtraFiles lists the regular files in the folder, for the Settings page.
func listExtraFiles() []extraFile {
	entries, err := os.ReadDir(extraFilesDir)
	if err != nil {
		return nil
	}
	files := []extraFile{}
	for _, e := range entries {
		if checkExtraFileName(e.Name()) != nil || !e.Type().IsRegular() {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, extraFile{Name: e.Name(), Size: fmt.Sprintf("%.1f MB", float64(info.Size())/1e6)})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files
}

// chownExtraFile hands an uploaded file to www-data, so the terminal can change or
// delete it. A variable because tests do not run as root.
var chownExtraFile = func(f *os.File) error {
	u, err := user.Lookup("www-data")
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	return f.Chown(uid, gid)
}

// receiveExtraFile stores one upload. It lands under a temporary name and is renamed
// when complete, so the terminal never sees half a file.
func receiveExtraFile(root *os.Root, name string, src io.Reader) error {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	tmp := ".upload-" + hex.EncodeToString(suffix)
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	if err == nil {
		err = chownExtraFile(f)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}

// handleFileUpload streams the file instead of using s.csrf, for the same reason as
// handleBackupUpload: FormValue would spool the whole body first.
func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	if !sameOriginOK(r) {
		http.Error(w, "cross-site request rejected", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "expected a multipart upload", http.StatusBadRequest)
		return
	}
	csrfOK := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			http.Error(w, "no file in the upload", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch part.FormName() {
		case "csrf":
			val, _ := io.ReadAll(io.LimitReader(part, 1024))
			csrfOK = csrfMatches(r, string(val))
		case "file":
			if !csrfOK {
				http.Error(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
			name := part.FileName()
			if err := checkExtraFileName(name); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			root, err := os.OpenRoot(extraFilesDir)
			if err != nil {
				http.Error(w, "upload failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer root.Close()
			if _, err := root.Lstat(name); err == nil {
				http.Error(w, "a file named "+name+" already exists", http.StatusConflict)
				return
			}
			if err := receiveExtraFile(root, name, part); err != nil {
				http.Error(w, "upload failed: "+err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
	}
}

// handleFileDownload serves a regular file as an attachment. O_NONBLOCK so a FIFO
// planted in the folder can't hang the request before the check below.
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	name := r.URL.Query().Get("name")
	if err := checkExtraFileName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	root, err := os.OpenRoot(extraFilesDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	// ServeContent keeps the Content-Type set above and adds Range support, so a large
	// download over a flaky link can resume.
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if !cliEnabled() {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	if err := checkExtraFileName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	root, err := os.OpenRoot(extraFilesDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}
	if err := root.Remove(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
