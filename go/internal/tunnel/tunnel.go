// Package tunnel manages the optional Cloudflare Quick Tunnel
// (try.cloudflare.com): one cloudflared process exposing the demo site on a
// random public trycloudflare.com URL, no account needed. While the tunnel
// runs, the Moodle wwwroot is rewritten to the public URL and restored when
// it stops — a demo-only shortcut: a real Moodle site does not survive
// wwwroot changes, a throwaway demo does.
package tunnel

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mutms/mdl-demo/go/internal/apache"
	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/svc"
)

// target is the site as cloudflared reaches it from inside the container.
const target = "http://127.0.0.1:8082"

const startTimeout = 45 * time.Second

var urlRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type tun struct {
	url     string
	proc    *os.Process
	done    chan struct{} // closed when cloudflared has exited
	stopped bool          // deliberate Stop, not a crash (guarded by mu)
}

var (
	mu       sync.Mutex
	cur      *tun
	starting bool
)

// URL returns the public tunnel URL, or "" when no tunnel is active.
func URL() string {
	mu.Lock()
	defer mu.Unlock()
	if cur == nil {
		return ""
	}
	return cur.url
}

// Start launches cloudflared, waits for its public URL, points the Moodle
// site at it and returns it. A second Start while one is active returns the
// existing URL.
func Start(logf execx.Logf) (string, error) {
	mu.Lock()
	if cur != nil {
		u := cur.url
		mu.Unlock()
		return u, nil
	}
	if starting {
		mu.Unlock()
		return "", fmt.Errorf("a tunnel is already starting")
	}
	starting = true
	mu.Unlock()
	defer func() { mu.Lock(); starting = false; mu.Unlock() }()

	logf("Starting Cloudflare Quick Tunnel")
	logf("$ cloudflared tunnel --no-autoupdate --url " + target)
	// setpriv --no-new-privs: same hardening as the supervised services.
	cmd := exec.Command("setpriv", "--no-new-privs", "--",
		"cloudflared", "tunnel", "--no-autoupdate", "--url", target)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout

	wait, err := startChild(cmd)
	if err != nil {
		return "", fmt.Errorf("cloudflared: %w", err)
	}

	t := &tun{proc: cmd.Process, done: make(chan struct{})}
	urlCh := make(chan string, 1)
	connCh := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		found, connected := false, false
		for scanner.Scan() {
			line := scanner.Text()
			logf(line)
			if !found {
				if u := urlRe.FindString(line); u != "" {
					found = true
					urlCh <- u
				}
			}
			if !connected && strings.Contains(line, "Registered tunnel connection") {
				connected = true
				connCh <- struct{}{}
			}
		}
	}()
	go func() {
		_ = wait()
		mu.Lock()
		deliberate := t.stopped
		if cur == t {
			cur = nil
		}
		mu.Unlock()
		close(t.done)
		if !deliberate {
			// Crash mid-demo: put the site back on its installed URL so at
			// least the local address works again.
			fmt.Fprintln(os.Stderr, "tunnel: cloudflared exited unexpectedly, restoring the site URL")
			restore(func(string) {})
		}
	}()

	select {
	case u := <-urlCh:
		// The URL is announced before the edge connections register; a browser
		// (or a QR-scanning audience) hitting it that early gets Cloudflare's
		// error 530. Wait for the first registered connection — but not forever,
		// the tunnel usually recovers on its own.
		select {
		case <-connCh:
		case <-t.done:
			return "", fmt.Errorf("cloudflared exited before connecting to the Cloudflare edge")
		case <-time.After(20 * time.Second):
			logf("warning: no edge connection registered yet, continuing anyway")
		}
		if err := point(logf, u); err != nil {
			stop(logf, t)
			_ = restore(logf) // a half-applied wwwroot must not outlive the tunnel
			return "", err
		}
		t.url = u
		mu.Lock()
		cur = t
		mu.Unlock()
		logf("Tunnel ready: " + u)
		return u, nil
	case <-t.done:
		return "", fmt.Errorf("cloudflared exited before announcing a tunnel URL")
	case <-time.After(startTimeout):
		stop(logf, t)
		return "", fmt.Errorf("no tunnel URL after %s", startTimeout)
	}
}

// Stop tears the tunnel down and points the site back at its installed URL.
// A no-op when no tunnel is active.
func Stop(logf execx.Logf) error {
	mu.Lock()
	t := cur
	if t == nil {
		mu.Unlock()
		return nil
	}
	t.stopped = true
	cur = nil
	mu.Unlock()

	logf("Stopping the tunnel")
	stop(logf, t)
	return restore(logf)
}

// stop terminates cloudflared and waits for its exit (escalating to SIGKILL),
// with t.stopped set so the exit monitor knows it is deliberate.
func stop(logf execx.Logf, t *tun) {
	mu.Lock()
	t.stopped = true
	mu.Unlock()
	_ = t.proc.Signal(syscall.SIGTERM)
	select {
	case <-t.done:
	case <-time.After(10 * time.Second):
		logf("cloudflared ignored SIGTERM, killing it")
		_ = t.proc.Kill()
		<-t.done
	}
}

func restore(logf execx.Logf) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	return point(logf, st.Wwwroot)
}

// point rewrites the site's wwwroot in config.php AND the Apache vhost's
// canonical ServerName (Apache builds its own redirects from it), then purges
// Moodle caches, which hold absolute URLs that would keep steering browsers
// at the old address. A no-op without an installed site.
func point(logf execx.Logf, wwwroot string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if wwwroot == "" || !st.Installed() || !moodle.Detected() {
		return nil
	}
	logf("Pointing the site at " + wwwroot)
	if err := moodle.WriteConfig(wwwroot); err != nil {
		return err
	}
	if err := apache.WriteVhost(wwwroot, moodle.Docroot(), moodle.HasRouter()); err != nil {
		return err
	}
	if err := svc.Current().ReloadApache(logf); err != nil {
		return err
	}
	return moodle.RunCLI(logf, "admin/cli/purge_caches.php")
}

// startChild mirrors execx: inside PID 1 the reaper-aware hook must start the
// child (a competing cmd.Wait loses the wait race), standalone uses os/exec.
func startChild(cmd *exec.Cmd) (func() error, error) {
	if execx.StartChild != nil {
		return execx.StartChild(cmd)
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Wait, nil
}
