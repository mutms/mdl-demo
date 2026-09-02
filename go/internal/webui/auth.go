package webui

// The console is a local port. It is addressed by IP address (127.0.0.1 in
// every documented command) or as localhost, and three checks keep a web page
// the user happens to be visiting from driving it:
//
//   - a CSRF token in a SameSite=Strict, HttpOnly cookie, echoed by every
//     state-changing form (double submit — nothing is kept server-side, so a
//     console restart does not invalidate a page someone has open);
//   - an Origin/Referer check on those same requests;
//   - a Host allow-list, which is what stops DNS rebinding — the one attack
//     the first two cannot see, because it makes a hostile page genuinely
//     same-origin with this console.
//
// Note the division of labour: cookies and SameSite both ignore ports, so
// several demos on one host share a token and neither of those two checks
// isolates the console from another service on a neighbouring port. The
// Origin check is what does that, and it is load-bearing rather than
// belt-and-braces.
//
// Reaching the console at all means reaching its port, and every documented
// run command publishes it on 127.0.0.1 only. That port binding — not
// anything in this file — is what decides who can talk to it.
//
// secureHeaders adds the browser-side layer: a Content-Security-Policy that
// allows nothing inline and no origin but this one, so an injected script
// or style could not run even if a template escape were ever wrong.

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	csrfCookie = "mdl_demo_csrf"
	// tokenBytes of entropy; base64 makes that a 43-character cookie.
	tokenBytes = 32
)

type ctxKey int

const csrfCtxKey ctxKey = iota

func randomToken() string {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the kernel CSPRNG failing is not recoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// validToken screens a cookie value before it is trusted as this browser's
// token: only something this console could have minted is echoed into a page.
func validToken(v string) bool {
	b, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(b) == tokenBytes
}

// guard wraps the whole mux. It turns away requests addressed to a host this
// console does not answer to, and makes sure the browser carries a CSRF
// token — minting one when it arrives without a usable cookie — for the
// templates to render into forms and for csrf to check against.
// csp is the whole policy: every script, style and image is a same-origin
// file, except the SSO QR code, which is a data: image. form-action is left
// out on purpose: the SSO login form ends in a redirect to the site's own
// origin, and Chrome applies form-action to redirects too.
const csp = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; frame-ancestors 'none'; base-uri 'none'"

// secureHeaders sets the browser hardening headers on every response. The
// Mailpit proxy under /mail/ is skipped: that is Mailpit's own page, with
// its own inline scripts, and it sets its own headers.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		// Only a secure context honours COOP; elsewhere the browser logs a
		// warning and ignores it, so send it where it can take effect.
		if r.TLS != nil || loopbackHost(hostOnly(r.Host)) {
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
		}
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if !strings.HasPrefix(r.URL.Path, "/mail/") {
			h.Set("Content-Security-Policy", csp)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host) {
			http.Error(w, "this console does not answer to the host "+r.Host+
				" — reach it at 127.0.0.1 or at another IP address", http.StatusForbidden)
			return
		}
		token := ""
		if c, err := r.Cookie(csrfCookie); err == nil && validToken(c.Value) {
			token = c.Value
		} else {
			token = randomToken()
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true, // nothing in the page reads it; the server renders it in
				SameSite: http.SameSiteStrictMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfCtxKey, token)))
	})
}

// csrfToken is the token guard settled on for this request — what the
// templates put in their hidden csrf field.
func csrfToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfCtxKey).(string)
	return token
}

// csrf verifies a state-changing request before it runs.
func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOriginOK(r) || !csrfMatches(r, r.FormValue("csrf")) {
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// csrfMatches reports whether sent is the token in this browser's cookie —
// which a cross-site request never carries (SameSite=Strict) and can never
// read (HttpOnly, and the response is unreadable cross-origin anyway). A
// browser arriving without a valid cookie has nothing to prove it ever loaded
// a page of ours, so it fails here whatever it sends.
func csrfMatches(r *http.Request, sent string) bool {
	c, err := r.Cookie(csrfCookie)
	if err != nil || !validToken(c.Value) {
		return false
	}
	return hmac.Equal([]byte(sent), []byte(c.Value))
}

// hostAllowed reports whether this console answers to the host a request was
// addressed to. It is the defence against DNS rebinding: a hostile page can
// point a name it owns at 127.0.0.1, and from then on the browser treats it
// as same-origin with this console — same-site cookie attached, CSRF token
// readable straight out of the DOM, Origin matching — but it cannot change
// the name it wrote into the Host header.
//
// Allowed:
//   - any IP literal: no resolver is involved, so there is nothing to rebind
//     (this is how the container's own address and a VM address keep working);
//   - localhost, and *.localhost, which browsers resolve to loopback
//     themselves without asking DNS. Nothing mdl-demo prints uses the name —
//     URLs it builds itself say 127.0.0.1, because localhost resolves to ::1
//     first on some machines and the ports are published on IPv4 only — but
//     people type it, so it is answered.
//
// Nothing widens this list. The console is a local port and has no setting
// that would let it answer to a name — one would only invite pointing it at
// something public. A site published under a real hostname is the job of the
// Moodle vhost on 8082, which this never touches.
// loopbackHost reports whether host names the browser's own machine, which
// browsers treat as a secure context even over plain HTTP.
func loopbackHost(host string) bool {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	return h == "localhost" || strings.HasSuffix(h, ".localhost")
}

func hostAllowed(hostport string) bool {
	host := hostOnly(hostport)
	if host == "" {
		return false
	}
	if net.ParseIP(strings.Trim(host, "[]")) != nil {
		return true
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	return h == "localhost" || strings.HasSuffix(h, ".localhost")
}

// sameOriginOK rejects state-changing requests whose Origin (or, failing
// that, Referer) names a different host than the one the request reached —
// a cheap cross-origin check on top of the CSRF token.
func sameOriginOK(r *http.Request) bool {
	// Fetch Metadata: a browser labels every request with where it came
	// from, and unlike Origin a form post cannot leave it out.
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
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
	// holds the cookie, so there is nothing left to protect.
	return true
}
