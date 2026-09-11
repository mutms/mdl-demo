package webui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSeries(t *testing.T) {
	cases := map[string]string{
		"5.2.2":    "5.2",
		"5.2.1":    "5.2",
		"5.2.2.01": "5.2", // a MuTMS build folds with its Moodle branch
		"5.2.1.01": "5.2",
		"5.2":      "5.2", // a dev stream is its own branch
		"4.5":      "4.5",
	}
	for v, want := range cases {
		if got := series(v); got != want {
			t.Errorf("series(%q) = %q, want %q", v, got, want)
		}
	}
}

// The event stream: the right headers, a reload up front when the page's epoch
// is not this process's, heartbeats, pushed events, and the subscriber gone
// once the client hangs up.
func TestEventsHandler(t *testing.T) {
	old := heartbeat
	heartbeat = 10 * time.Millisecond
	defer func() { heartbeat = old }()

	s := &Server{hub: newHub(), job: &job{}, epoch: "e1"}
	h, err := s.routes()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	open := func(epoch string) (*bufio.Reader, context.CancelFunc, *http.Response) {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/events?e="+epoch, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		return bufio.NewReader(resp.Body), cancel, resp
	}
	readUntil := func(r *bufio.Reader, want string) string {
		var got strings.Builder
		for {
			line, err := r.ReadString('\n')
			got.WriteString(line)
			if strings.Contains(got.String(), want) {
				return got.String()
			}
			if err != nil {
				t.Fatalf("stream ended before %q: %q", want, got.String())
			}
		}
	}

	r, cancel, resp := open("stale")
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	got := readUntil(r, "data: epoch\n")
	if !strings.HasPrefix(got, "retry: 2000\n\nevent: reload\n") {
		t.Errorf("stale epoch: got %q", got)
	}
	readUntil(r, ": ping\n")
	s.hub.notify(ev(evJob), event{evReload, "job"})
	if got := readUntil(r, "data: job\n"); !strings.Contains(got, "event: job\ndata: \n\nevent: reload\ndata: job\n") {
		t.Errorf("pushed events: got %q", got)
	}
	cancel()
	resp.Body.Close()

	r, cancel, resp = open("e1")
	defer cancel()
	defer resp.Body.Close()
	if got := readUntil(r, ": ping\n"); strings.Contains(got, "reload") {
		t.Errorf("current epoch got a reload: %q", got)
	}
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.hub.mu.Lock()
		n := len(s.hub.subs)
		s.hub.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d subscribers still registered after the clients left", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Every page and fragment renders against an empty and a busy/installed view:
// a template naming a field the view no longer has fails here, not in a
// running container.
func TestTemplatesRender(t *testing.T) {
	names := []string{"page", "plugins", "backups", "camp", "settings", "recommends",
		"site", "install", "users", "tools", "progress", "jobstatus", "statuspill", "svcproblems",
		"backuplist", "logtail", "ssodialog", "ssoqr", "ssopoll"}
	views := map[string]view{
		"empty": {Lang: "en"},
		"busy": {Lang: "en", Epoch: "e1", Installed: true, Busy: true, Recipe: "moodle/release/5.2",
			Job:             jobView{Kind: "install", Running: true, Label: "installing", Log: []string{"a", "b"}, Next: 2},
			ServiceProblems: []serviceRow{{Name: "mailpit", Status: "restarting"}},
			Users:           []userRow{{Username: "admin", Password: "x", Role: "admin"}},
			TunnelURL:       "https://x.trycloudflare.com", Snapshot: true},
	}
	for label, v := range views {
		for _, name := range names {
			w := httptest.NewRecorder()
			s := &Server{}
			s.render(w, name, v)
			if body := w.Body.String(); strings.Contains(body, "render error") {
				t.Errorf("%s/%s: %s", name, label, body[strings.Index(body, "render error"):])
			}
		}
	}
}
