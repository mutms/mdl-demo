package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func cliTestServer(t *testing.T) http.Handler {
	t.Helper()
	s := &Server{job: &job{}}
	h, err := s.routes()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestCLISocketGuards(t *testing.T) {
	h := cliTestServer(t)
	good := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 zero bytes, base64
	cases := []struct {
		name   string
		host   string
		origin string
		fetch  string
		cookie string
		token  string
	}{
		{"foreign origin", "127.0.0.1:8081", "https://evil.example", "", good, good},
		{"cross-site fetch metadata", "127.0.0.1:8081", "", "cross-site", good, good},
		{"rebound host", "evil.example:8081", "http://evil.example:8081", "", good, good},
		{"no token", "127.0.0.1:8081", "http://127.0.0.1:8081", "same-origin", good, ""},
		{"wrong token", "127.0.0.1:8081", "http://127.0.0.1:8081", "same-origin", good, strings.Repeat("B", 43)},
		{"no cookie", "127.0.0.1:8081", "http://127.0.0.1:8081", "same-origin", "", good},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/cli/ws?csrf="+c.token, nil)
			r.Host = c.host
			r.Header.Set("Connection", "Upgrade")
			r.Header.Set("Upgrade", "websocket")
			r.Header.Set("Sec-WebSocket-Version", "13")
			r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			if c.fetch != "" {
				r.Header.Set("Sec-Fetch-Site", c.fetch)
			}
			if c.cookie != "" {
				r.AddCookie(&http.Cookie{Name: csrfCookie, Value: c.cookie})
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

func TestCLIPage(t *testing.T) {
	r := httptest.NewRequest("GET", "/cli", nil)
	r.Host = "127.0.0.1:8081"
	w := httptest.NewRecorder()
	cliTestServer(t).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `data-csrf="`) {
		t.Fatal("the page carries no token for the socket")
	}
	if !strings.Contains(w.Body.String(), "<footer>") {
		t.Fatal("the page dropped the legal notice footer (invariant 8)")
	}
}

func TestCLIKillSwitch(t *testing.T) {
	orig := cliEnabled
	cliEnabled = func() bool { return false }
	t.Cleanup(func() { cliEnabled = orig })
	h := cliTestServer(t)

	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 32 zero bytes, base64
	for _, route := range []struct{ method, path string }{
		{"GET", "/cli"},
		{"GET", "/cli/ws?csrf=" + token},
		{"POST", "/cli/install-tools?csrf=" + token},
	} {
		r := httptest.NewRequest(route.method, route.path, nil)
		r.Host = "127.0.0.1:8081"
		r.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", route.method, route.path, w.Code)
		}
	}
}

// Only the terminal page may allow inline style.
func TestCLIContentSecurityPolicy(t *testing.T) {
	h := cliTestServer(t)
	policy := func(path string) string {
		r := httptest.NewRequest("GET", path, nil)
		r.Host = "127.0.0.1:8081"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Header().Get("Content-Security-Policy")
	}
	for _, want := range []string{"style-src-elem 'self' 'unsafe-inline'", "style-src-attr 'unsafe-inline'"} {
		if got := policy("/cli"); !strings.Contains(got, want) {
			t.Errorf("/cli policy = %q, want %q", got, want)
		}
	}
	if got := policy("/cli"); strings.Contains(got, "script-src 'self' 'unsafe") {
		t.Errorf("/cli must keep scripts strict: %q", got)
	}
	for _, path := range []string{"/", "/settings", "/cli/ws", "/static/cli.js"} {
		if got := policy(path); strings.Contains(got, "unsafe-inline") {
			t.Errorf("%s policy allows inline: %q", path, got)
		}
	}
}

func TestJobLogText(t *testing.T) {
	j := &job{kind: "tools", lines: []string{"one", "two"}, running: true}
	s := &Server{job: j}
	h, err := s.routes()
	if err != nil {
		t.Fatal(err)
	}
	get := func(url string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url, nil)
		r.Host = "127.0.0.1:8081"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	w := get("/joblog?format=text&from=1")
	if got := w.Body.String(); got != "two\n" {
		t.Errorf("body = %q", got)
	}
	if w.Header().Get("X-Job-Next") != "2" || w.Header().Get("X-Job-Running") != "true" || w.Header().Get("X-Job-Failed") != "false" {
		t.Errorf("headers = %v", w.Header())
	}

	j.running, j.err = false, os.ErrNotExist
	w = get("/joblog?format=text&from=2")
	if !strings.Contains(w.Body.String(), os.ErrNotExist.Error()) {
		t.Errorf("a failed job must say why: %q", w.Body.String())
	}
	if w.Header().Get("X-Job-Running") != "false" || w.Header().Get("X-Job-Failed") != "true" {
		t.Errorf("headers = %v", w.Header())
	}
}
