// Package initd is mdl-demo's own init: PID 1 for container runtimes that
// cannot boot systemd (the WSL containers preview mounts cgroup2 read-only
// and grants no CAP_SYS_ADMIN, so systemd dies before its manager starts).
//
// Started with `mdl-demo init` as the container entrypoint, typically with
// the runtime pre-mounting a tmpfs on /run (e.g. wslc's --tmpfs /run). It
// starts postgres, php-fpm and apache in the foreground, supervises them
// with restart backoff, runs the management web UI in-process, replaces the
// systemd cron timer with a Go ticker, reaps orphans, and turns the stop
// signal into an ordered shutdown. It deliberately does NOT self-heal
// beyond restarting: misbehavior is surfaced on the diagnostics page for
// users to copy into bug reports.
package initd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/svc"
	"github.com/mutms/mdl-demo/go/internal/webui"
)

// sigStop is what the image's STOPSIGNAL (SIGRTMIN+3, glibc numbering)
// resolves to; runtimes send the number, not the name.
const sigStop = syscall.Signal(37)

const logTailLines = 100

type proc struct {
	name   string
	newCmd func() (*exec.Cmd, error)
	// stop is the signal for ordered shutdown (postgres wants SIGINT —
	// its documented "fast shutdown" — the rest plain SIGTERM).
	stop syscall.Signal

	mu       sync.Mutex
	pid      int
	running  bool
	restarts int
	lastExit string
	tail     []string
	exitCh   chan syscall.WaitStatus
}

func (p *proc) log(line string) {
	p.mu.Lock()
	p.tail = append(p.tail, time.Now().UTC().Format("15:04:05")+" "+line)
	if len(p.tail) > logTailLines {
		p.tail = p.tail[len(p.tail)-logTailLines:]
	}
	p.mu.Unlock()
}

// Supervisor implements svc.Manager for the no-systemd boot.
type Supervisor struct {
	version  string
	started  time.Time
	mu       sync.Mutex
	byPID    map[int]*proc
	procs    []*proc
	stopping atomic.Bool

	// execWaiters routes the exit status of a child started through StartChild
	// (execx, in-process) back to its waiter, keyed by pid — the same job
	// byPID does for a supervised service. Both are read under mu by reap.
	execWaiters map[int]chan syscall.WaitStatus
}

// Run boots the world and blocks until shutdown. Never returns in normal
// operation; an error means boot itself failed.
func Run(version string) error {
	if os.Getpid() != 1 {
		fmt.Fprintln(os.Stderr, "warning: mdl-demo init is not PID 1 — zombie reaping is not guaranteed")
	}

	s := &Supervisor{
		version:     version,
		started:     time.Now(),
		byPID:       map[int]*proc{},
		execWaiters: map[int]chan syscall.WaitStatus{},
	}
	svc.Use(s)

	if err := prepareRunDirs(); err != nil {
		return err
	}

	s.procs = []*proc{
		{name: "postgresql", stop: syscall.SIGINT, newCmd: postgresCmd},
		{name: "php8.3-fpm", stop: syscall.SIGTERM, newCmd: func() (*exec.Cmd, error) {
			return nnp("/usr/sbin/php-fpm8.3", "--nodaemonize"), nil
		}},
		{name: "apache2", stop: syscall.SIGTERM, newCmd: func() (*exec.Cmd, error) {
			return nnp("apache2ctl", "-DFOREGROUND"), nil
		}},
		// Mailpit catches all the site's outgoing mail; in-memory on purpose
		// (throwaway messages, cleared by a container restart). The console
		// proxies its UI under /mail behind the console session.
		{name: "mailpit", stop: syscall.SIGTERM, newCmd: func() (*exec.Cmd, error) {
			return nnp("mailpit",
				"--smtp", "127.0.0.1:1025",
				"--listen", "127.0.0.1:8025",
				"--webroot", "/mail"), nil
		}},
	}
	for _, p := range s.procs {
		p.exitCh = make(chan syscall.WaitStatus, 1)
	}

	// Signal plumbing must be in place before the first child starts.
	sigchld := make(chan os.Signal, 16)
	signal.Notify(sigchld, syscall.SIGCHLD)
	stop := make(chan os.Signal, 2)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, sigStop)
	go s.reap(sigchld)

	// Only now, with the reaper running, may in-process execx route its waits
	// through it. Everything before this point (prepareRunDirs) ran before the
	// reaper and used a plain cmd.Wait(), which is safe with no reaper to race.
	// Everything after — the supervised services below still use their own
	// exitCh path; the web UI's install/reset jobs and the cron ticker use
	// execx — waits through the reaper instead of losing the race with ECHILD.
	execx.StartChild = s.StartChild

	for _, p := range s.procs {
		go s.supervise(p)
	}
	// Clear a busy lock left by an operation that crashed in a previous process,
	// so the cron ticker isn't wedged off after a restart.
	state.ClearBusy()
	go s.cronLoop()
	go func() {
		if err := webui.Serve(os.Stdout, version); err != nil {
			fmt.Fprintf(os.Stderr, "web UI died: %v\n", err)
		}
	}()

	fmt.Println("mdl-demo init: supervising postgresql, php8.3-fpm, apache2 (mode mdl-demo-init)")
	sig := <-stop
	fmt.Printf("mdl-demo init: received %v, shutting down\n", sig)
	s.shutdown()
	return nil
}

// reap is the single wait()er: it collects every exited child — supervised
// service, in-process execx command, or reparented orphan — and routes each
// exit to whoever is waiting for it. Using one central Wait4 avoids the classic
// Go-PID-1 race between a global reaper and per-child cmd.Wait(): nothing else
// calls Wait, so the status is never stolen out from under its waiter.
func (s *Supervisor) reap(sigchld <-chan os.Signal) {
	for range sigchld {
		for {
			var ws syscall.WaitStatus
			pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
			if pid <= 0 || err != nil {
				break
			}
			s.mu.Lock()
			p := s.byPID[pid]
			delete(s.byPID, pid)
			ch := s.execWaiters[pid]
			delete(s.execWaiters, pid)
			s.mu.Unlock()
			// A pid is in at most one map; both channels are buffered, so the
			// send never blocks the reaper. An unmatched pid is a reparented
			// orphan — reaped here, with nothing to route.
			switch {
			case p != nil:
				p.exitCh <- ws
			case ch != nil:
				ch <- ws
			}
		}
	}
}

// StartChild starts cmd and returns a wait function yielding its exit status.
// It is execx.StartChild inside PID 1: code in this process must not call
// cmd.Wait(), because reap already collects every child with Wait4(-1) and a
// second waiter loses the race with ECHILD ("waitid: no child processes").
//
// Start() and the registration run under mu, the lock reap takes before it
// routes a reaped pid. The child cannot be reaped before it exists (after
// Start), so reap's routing lookup blocks until the pid is registered here —
// which closes the start-vs-reap window without holding mu across the Wait4
// syscall itself.
func (s *Supervisor) StartChild(cmd *exec.Cmd) (func() error, error) {
	s.mu.Lock()
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	ch := make(chan syscall.WaitStatus, 1)
	s.execWaiters[cmd.Process.Pid] = ch
	s.mu.Unlock()

	wait := func() error {
		ws := <-ch
		// reap already collected the pid; Release just drops os/exec's handle
		// so nothing later tries (and fails) to Wait on it.
		_ = cmd.Process.Release()
		return waitStatusError(ws)
	}
	return wait, nil
}

// waitStatusError renders a WaitStatus the way exec.Cmd.Wait would: nil for a
// clean exit, otherwise a message matching os/exec's own wording, since the
// routed wait path cannot hand back an *exec.ExitError.
func waitStatusError(ws syscall.WaitStatus) error {
	switch {
	case ws.Exited() && ws.ExitStatus() == 0:
		return nil
	case ws.Exited():
		return fmt.Errorf("exit status %d", ws.ExitStatus())
	case ws.Signaled():
		return fmt.Errorf("signal: %s", ws.Signal())
	default:
		return fmt.Errorf("unexpected wait status %#x", ws)
	}
}

func (s *Supervisor) supervise(p *proc) {
	backoff := time.Second
	for !s.stopping.Load() {
		cmd, err := p.newCmd()
		if err != nil {
			p.log("cannot build command: " + err.Error())
			return
		}
		pr, pw, err := os.Pipe()
		if err == nil {
			cmd.Stdout, cmd.Stderr = pw, pw
		}
		if err := cmd.Start(); err != nil {
			p.log("start failed: " + err.Error())
		} else {
			if pw != nil {
				pw.Close()
				go func() {
					buf := make([]byte, 4096)
					carry := ""
					for {
						n, err := pr.Read(buf)
						if n > 0 {
							lines := strings.Split(carry+string(buf[:n]), "\n")
							carry = lines[len(lines)-1]
							for _, l := range lines[:len(lines)-1] {
								p.log(l)
							}
						}
						if err != nil {
							if carry != "" {
								p.log(carry)
							}
							return
						}
					}
				}()
			}
			started := time.Now()
			s.mu.Lock()
			s.byPID[cmd.Process.Pid] = p
			s.mu.Unlock()
			p.mu.Lock()
			p.pid, p.running = cmd.Process.Pid, true
			p.mu.Unlock()

			ws := <-p.exitCh
			_ = cmd.Process.Release()
			p.mu.Lock()
			p.running = false
			p.lastExit = exitString(ws)
			p.restarts++
			p.mu.Unlock()
			if s.stopping.Load() {
				return
			}
			// A service that ran for a minute earned a fresh backoff; a
			// crash loop backs off up to 30s but never gives up.
			if time.Since(started) > time.Minute {
				backoff = time.Second
			}
			p.log(fmt.Sprintf("exited (%s), restarting in %s", exitString(ws), backoff))
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// cronLoop replaces moodle-cron.timer: one tick a minute whenever
// state.json says a site is installed — keyed off the file, not an
// in-memory flag, so installs done via `exec mdl-demo install` (a separate
// process) are picked up too. Runs cron inline so slow runs cannot pile up.
func (s *Supervisor) cronLoop() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for range tick.C {
		if s.stopping.Load() {
			return
		}
		st, err := state.Load()
		if err != nil || !st.Installed() || !moodle.Detected() {
			continue
		}
		// Skip while a destructive op runs (install/reset/plugin add/restore/
		// backup): a cron run mid-upgrade hits a half-changed schema and errors.
		if state.Busy() {
			continue
		}
		// Output streams into the web UI's Site log ("cron:"-prefixed);
		// errors also land on PID 1's stderr for podman logs.
		logf := func(line string) { webui.SiteLog("cron: " + line) }
		if err := moodle.Cron(logf); err != nil {
			logf("failed: " + err.Error())
			fmt.Fprintf(os.Stderr, "moodle cron: %v\n", err)
		}
	}
}

func (s *Supervisor) shutdown() {
	s.stopping.Store(true)
	// Reverse dependency order: web tier first, database last.
	for i := len(s.procs) - 1; i >= 0; i-- {
		p := s.procs[i]
		p.mu.Lock()
		pid, running := p.pid, p.running
		p.mu.Unlock()
		if running {
			_ = syscall.Kill(pid, p.stop)
		}
	}
	// Stay under container runtimes' default 10s stop timeout: PID 1 must
	// exit before the runtime escalates to SIGKILL, even if a service
	// ignores its stop signal (namespace teardown reaps it anyway).
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		alive := false
		for _, p := range s.procs {
			p.mu.Lock()
			alive = alive || p.running
			p.mu.Unlock()
		}
		if !alive {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// --- svc.Manager ---

func (s *Supervisor) Mode() string { return "mdl-demo-init" }

// ReloadApache goes through apache2ctl (pidfile-based) rather than
// signalling the supervised PID directly, so the exact same path works from
// exec'd CLI processes (svc.NewStandalone) and stays consistent here.
func (s *Supervisor) ReloadApache(logf execx.Logf) error {
	return execx.Run(logf, "", "apache2ctl", "graceful")
}

// EnableCron/DisableCron only narrate: the ticker keys off state.json,
// which the install/reset flows maintain.
func (s *Supervisor) EnableCron(logf execx.Logf) error {
	logf("Moodle cron ticker armed (runs every minute while a site is installed)")
	return nil
}

func (s *Supervisor) DisableCron(logf execx.Logf) error {
	logf("Moodle cron ticker disarmed (no site installed)")
	return nil
}

func (s *Supervisor) Statuses() []svc.Status {
	var out []svc.Status
	for _, p := range s.procs {
		out = append(out, s.statusOf(p))
	}
	// The cron ticker only runs while a site is installed — that is by design,
	// not a fault. So list it only when installed (and then it is active),
	// exactly like the on-demand cloudflared is simply absent otherwise; this
	// keeps it out of the "services not running" banner on an empty demo.
	if st, err := state.Load(); err == nil && st.Installed() {
		out = append(out, svc.Status{Name: "moodle-cron (ticker)", State: "active", Running: true})
	}
	return out
}

func (s *Supervisor) Diagnostics() []svc.Diag {
	var out []svc.Diag
	for _, p := range s.procs {
		// statusOf locks p.mu itself, so it must run before we take the
		// lock for the remaining fields (sync.Mutex is not reentrant).
		d := svc.Diag{Status: s.statusOf(p)}
		p.mu.Lock()
		d.PID, d.Restarts, d.LastExit = p.pid, p.restarts, p.lastExit
		d.LogTail = append([]string(nil), p.tail...)
		p.mu.Unlock()
		out = append(out, d)
	}
	return out
}

func (s *Supervisor) statusOf(p *proc) svc.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := svc.Status{Name: p.name}
	switch {
	case p.running && p.restarts == 0:
		st.State, st.Running = "active", true
	case p.running:
		st.State, st.Running = fmt.Sprintf("active (restarted %d×)", p.restarts), true
	default:
		st.State = "restarting"
	}
	return st
}

func (s *Supervisor) find(name string) *proc {
	for _, p := range s.procs {
		if p.name == name {
			return p
		}
	}
	panic("unknown proc " + name) // static list; a miss is a programming error
}

// --- helpers ---

// nnp wraps a service in `setpriv --no-new-privs`: nothing in that process
// tree can ever gain privileges again — setuid binaries go inert for a
// compromised www-data — the closest portable stand-in for an LSM profile
// (containers cannot load AppArmor policy, and two of the three target
// runtimes have no AppArmor at all). Voluntary privilege drops (the
// apache/php-fpm masters setuid()ing workers to www-data) are unaffected.
// setpriv execs the service, so the supervised child IS the service.
func nnp(name string, args ...string) *exec.Cmd {
	return exec.Command("setpriv", append([]string{"--no-new-privs", "--", name}, args...)...)
}

// prepareRunDirs recreates the service socket/pid directories. /run may be
// a plain overlay directory rather than a fresh tmpfs (no runtime flags
// required), so stale pidfiles/sockets from a previous boot are wiped —
// a stale apache pid matching a reused PID would otherwise block startup.
func prepareRunDirs() error {
	for _, d := range []string{"/run/php", "/run/apache2", "/run/lock", "/run/postgresql"} {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	// postgres refuses a socket dir it cannot write.
	if err := execx.Run(func(string) {}, "", "chown", "postgres:postgres", "/run/postgresql"); err != nil {
		return err
	}
	return os.Chmod("/run/postgresql", 02775)
}

// postgresCmd runs the cluster's postmaster in the foreground as the
// postgres user, against the Debian cluster layout (version discovered by
// glob so the image can move to postgres 18 without touching this).
// Credentials are switched directly rather than via runuser: the supervised
// child must BE the postmaster, so stop/reload signals reach it instead of
// a wrapper that may not forward them.
func postgresCmd() (*exec.Cmd, error) {
	bins, _ := filepath.Glob("/usr/lib/postgresql/*/bin/postgres")
	confs, _ := filepath.Glob("/etc/postgresql/*/main/postgresql.conf")
	datas, _ := filepath.Glob("/var/lib/postgresql/*/main")
	if len(bins) != 1 || len(confs) != 1 || len(datas) != 1 {
		return nil, fmt.Errorf("cannot locate exactly one postgres cluster (bins=%v confs=%v)", bins, confs)
	}
	u, err := user.Lookup("postgres")
	if err != nil {
		return nil, err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	// Supplementary groups matter: Debian's postgres user is in ssl-cert,
	// without which the postmaster cannot read the snakeoil key and dies at
	// startup ("could not access private key file").
	var groups []uint32
	if ids, err := u.GroupIds(); err == nil {
		for _, id := range ids {
			if n, err := strconv.Atoi(id); err == nil {
				groups = append(groups, uint32(n))
			}
		}
	}
	// nnp execs the postmaster, so the supervised child still IS it (the
	// credential switch happens at fork, before setpriv runs).
	cmd := nnp(bins[0], "-D", datas[0], "-c", "config_file="+confs[0])
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups},
	}
	cmd.Env = append(os.Environ(), "HOME="+u.HomeDir, "USER=postgres")
	return cmd, nil
}

func exitString(ws syscall.WaitStatus) string {
	switch {
	case ws.Exited():
		return fmt.Sprintf("exit status %d", ws.ExitStatus())
	case ws.Signaled():
		return "killed by " + ws.Signal().String()
	default:
		return fmt.Sprintf("wait status %#x", uint32(ws))
	}
}
