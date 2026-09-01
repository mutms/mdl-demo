package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostAllowed(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost:8081", true},
		{"localhost", true},
		{"LocalHost:8081", true},
		{"localhost.:8081", true}, // fully qualified, trailing dot
		{"demo.localhost:8081", true},
		{"127.0.0.1:8081", true},
		{"[::1]:8081", true},
		{"10.163.222.1:6381", true}, // the mpd VM, reached by address
		{"192.168.64.5:8081", true}, // an Apple container / WSL container IP
		// A name is the one thing a rebinding page can bring, so no name
		// beyond localhost is answered — not even one that looks internal.
		{"mdl-demo.222.mpd.test", false},
		{"evil.example:8081", false},
		{"notlocalhost:8081", false},
		{"localhost.evil.example:8081", false},
		{"", false},
	}
	for _, c := range cases {
		if got := hostAllowed(c.host); got != c.want {
			t.Errorf("hostAllowed(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// guard must answer a rebinding attempt with 403 and none of our content.
func TestGuardRejectsForeignHost(t *testing.T) {
	s := &Server{}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler ran for a host the console does not answer to")
	}))
	r := httptest.NewRequest("GET", "http://evil.example:8081/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// guard mints a token for a browser that has no cookie yet, and leaves an
// existing one alone so an open page keeps working across a console restart.
func TestGuardIssuesAndKeepsToken(t *testing.T) {
	s := &Server{}
	var seen string
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = csrfToken(r)
	}))

	r := httptest.NewRequest("GET", "http://localhost:8081/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !validToken(seen) {
		t.Fatalf("no usable token issued: %q", seen)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != csrfCookie || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie = %+v, want %s HttpOnly SameSite=Strict", cookie, csrfCookie)
	}

	r2 := httptest.NewRequest("GET", "http://localhost:8081/", nil)
	r2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if seen != cookie.Value {
		t.Fatalf("token changed: %q, want %q", seen, cookie.Value)
	}
	if len(w2.Result().Cookies()) != 0 {
		t.Fatal("a browser that already holds a token should not be re-issued one")
	}
}

func TestCSRF(t *testing.T) {
	token := randomToken()
	post := func(sent, cookie, origin string) int {
		r := httptest.NewRequest("POST", "http://localhost:8081/reset",
			strings.NewReader("csrf="+sent))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: csrfCookie, Value: cookie})
		}
		w := httptest.NewRecorder()
		s := &Server{}
		s.csrf(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})(w, r)
		return w.Code
	}

	if got := post(token, token, "http://localhost:8081"); got != http.StatusNoContent {
		t.Errorf("matching token and origin: %d, want 204", got)
	}
	if got := post(token, token, ""); got != http.StatusNoContent {
		t.Errorf("matching token, no Origin header: %d, want 204", got)
	}
	// The cross-site cases: SameSite=Strict keeps the cookie at home, so an
	// attacker holds neither half — and guessing one alone is no help.
	if got := post(token, "", "http://localhost:8081"); got != http.StatusForbidden {
		t.Errorf("no cookie: %d, want 403", got)
	}
	if got := post("", token, "http://localhost:8081"); got != http.StatusForbidden {
		t.Errorf("no form token: %d, want 403", got)
	}
	if got := post(randomToken(), token, "http://localhost:8081"); got != http.StatusForbidden {
		t.Errorf("wrong token: %d, want 403", got)
	}
	if got := post(token, token, "http://evil.example"); got != http.StatusForbidden {
		t.Errorf("foreign origin: %d, want 403", got)
	}
}

// Fetch Metadata is the second cross-site guard: a browser that labels the
// request as coming from another site is turned away whatever else it sends.
func TestFetchMetadata(t *testing.T) {
	post := func(site string) bool {
		r := httptest.NewRequest("POST", "http://localhost:8081/reset", nil)
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		return sameOriginOK(r)
	}
	for _, ok := range []string{"", "same-origin", "none"} {
		if !post(ok) {
			t.Errorf("Sec-Fetch-Site %q rejected", ok)
		}
	}
	for _, bad := range []string{"cross-site", "same-site"} {
		if post(bad) {
			t.Errorf("Sec-Fetch-Site %q accepted", bad)
		}
	}
}

// Every console response carries the policy; the proxied Mailpit page keeps
// its own.
func TestSecureHeaders(t *testing.T) {
	h := secureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	get := func(path string) http.Header {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost:8081"+path, nil))
		return w.Result().Header
	}
	own := get("/")
	if own.Get("Content-Security-Policy") != csp {
		t.Errorf("CSP = %q", own.Get("Content-Security-Policy"))
	}
	if strings.Contains(csp, "unsafe") {
		t.Errorf("policy allows something unsafe: %s", csp)
	}
	for _, k := range []string{"X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options", "Cross-Origin-Opener-Policy"} {
		if own.Get(k) == "" {
			t.Errorf("%s missing", k)
		}
	}
	// COOP is meaningful only in a secure context: on the VM address over
	// plain HTTP the browser would ignore it and log a warning.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://10.163.222.1:6381/", nil))
	if w.Result().Header.Get("Cross-Origin-Opener-Policy") != "" {
		t.Error("COOP sent to a non-loopback plain-HTTP origin")
	}
	if mail := get("/mail/"); mail.Get("Content-Security-Policy") != "" {
		t.Error("CSP set on the Mailpit proxy")
	}
}
