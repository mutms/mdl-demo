package webui

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	pbkdf2Iterations = 600_000
	sessionCookie    = "mdl_demo_session"
	sessionTTL       = 24 * time.Hour
	maxLoginFails    = 10
	failWindow       = 15 * time.Minute
)

// hashPassword returns "pbkdf2-sha256:<iter>:<salt-b64>:<key-b64>".
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256:%d:%s:%s", pbkdf2Iterations,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(key)), nil
}

func verifyPassword(stored, password string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > 10_000_000 {
		return false
	}
	salt, err1 := base64.StdEncoding.DecodeString(parts[2])
	want, err2 := base64.StdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}

// sessions is an in-memory session + CSRF-token store; a serve restart
// logs everyone out, which is fine for a single-user localhost UI.
type sessions struct {
	mu    sync.Mutex
	live  map[string]session
	fails map[string][]time.Time
}

type session struct {
	expires time.Time
	csrf    string
}

func newSessions() *sessions {
	return &sessions{live: map[string]session{}, fails: map[string][]time.Time{}}
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the kernel CSPRNG failing is not recoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *sessions) start(w http.ResponseWriter) {
	token := randomToken()
	s.mu.Lock()
	s.live[token] = session{expires: time.Now().Add(sessionTTL), csrf: randomToken()}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// get returns the live session for the request, if any.
func (s *sessions) get(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.live[c.Value]
	if !ok {
		return session{}, false
	}
	if time.Now().After(sess.expires) {
		delete(s.live, c.Value)
		return session{}, false
	}
	return sess, true
}

func (s *sessions) end(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.live, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

// throttled reports whether the client IP has burned its login attempts.
func (s *sessions) throttled(r *http.Request) bool {
	ip := clientIP(r)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := s.fails[ip][:0]
	for _, t := range s.fails[ip] {
		if now.Sub(t) < failWindow {
			recent = append(recent, t)
		}
	}
	s.fails[ip] = recent
	return len(recent) >= maxLoginFails
}

func (s *sessions) recordFail(r *http.Request) {
	ip := clientIP(r)
	s.mu.Lock()
	s.fails[ip] = append(s.fails[ip], time.Now())
	s.mu.Unlock()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sameOriginOK rejects state-changing requests whose Origin (or, failing
// that, Referer) names a different host than the one the request reached —
// a cheap cross-origin check on top of the per-session CSRF token.
func sameOriginOK(r *http.Request) bool {
	check := func(raw string) bool {
		u, err := url.Parse(raw)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	}
	if o := r.Header.Get("Origin"); o != "" && o != "null" {
		return check(o)
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		return check(ref)
	}
	// Same-origin form posts always carry at least one of the two in every
	// current browser; a bare request is a non-browser client that already
	// has the session cookie, so there is nothing left to protect.
	return true
}
