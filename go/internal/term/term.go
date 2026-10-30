// Package term is the CLI console: a shell as www-data, in a browser window,
// over one WebSocket. The shell lives in a single shared tmux session, so work
// (a running Claude Code, a data generator) outlives the window and a reconnect
// redraws the screen with no replay buffer here.
//
// Wire protocol: binary frames are pty bytes, both ways; text frames carry JSON
// control from the browser, today only the window size. What guards the socket
// is in webui; this package only moves bytes.
package term

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

const (
	// userName runs the shell: the same account as PHP, so Moodle CLI scripts
	// write caches and dataroot files the site can still use. No sudo: helpers
	// reach root-level work through the console on loopback (container/cli/).
	userName = "www-data"
	home     = "/home/www-data"
	workDir  = "/srv/projects"
	session  = "main"

	// maxInput bounds one frame from the browser: a large paste arrives as one.
	maxInput = 8 << 20
)

// newCommand builds the process a connection attaches to; tests swap in a
// plain shell, since they do not run as root.
var newCommand = tmuxCommand

// tmuxCommand attaches to the shared session, creating it if needed. Like every
// other child, it runs with no-new-privs: nothing in this shell can gain
// privileges, which is why the image carries no sudo.
func tmuxCommand() (*exec.Cmd, error) {
	u, err := user.Lookup(userName)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("setpriv", "--no-new-privs", "--",
		"tmux", "-u", "new-session", "-A", "-s", session, "-c", workDir)
	cmd.Dir = workDir
	// An explicit environment: the shell must not inherit PID 1's, which holds
	// whatever the container was started with.
	cmd.Env = []string{
		"HOME=" + home,
		"USER=" + userName,
		"LOGNAME=" + userName,
		"SHELL=/bin/bash",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{uint32(gid)}},
	}
	return cmd, nil
}

// resize is the one control message the browser sends.
type resize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Attach runs a shell client for this connection until either side ends. The
// tmux server stays behind when it returns: closing the pty only ends the client.
func Attach(ctx context.Context, conn *websocket.Conn) error {
	conn.SetReadLimit(maxInput)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd, err := newCommand()
	if err != nil {
		return err
	}
	master, tty, err := pty.Open()
	if err != nil {
		return fmt.Errorf("opening a pty: %w", err)
	}
	var closeMaster sync.Once
	stop := func() { closeMaster.Do(func() { master.Close() }) }
	defer stop()

	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Ctty = 0 // the child's stdin, above
	wait, err := startChild(cmd)
	tty.Close() // the child holds its own copy; EOF on master needs this one gone
	if err != nil {
		return fmt.Errorf("starting the shell: %w", err)
	}

	// Browser to pty. A read error is the browser going away.
	go func() {
		defer stop()
		defer cancel()
		for {
			kind, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if kind == websocket.MessageText {
				var r resize
				if json.Unmarshal(data, &r) == nil && r.Cols > 0 && r.Rows > 0 {
					_ = pty.Setsize(master, &pty.Winsize{Cols: r.Cols, Rows: r.Rows})
				}
				continue
			}
			if _, err := master.Write(data); err != nil {
				return
			}
		}
	}()

	// Pty to browser, until the client exits (Linux reports that as EIO) or the
	// browser goes away and stop closes the master.
	buf := make([]byte, 32*1024)
	for {
		n, err := master.Read(buf)
		if n > 0 {
			if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) && !errors.Is(err, os.ErrClosed) {
				fmt.Fprintln(os.Stderr, "term: reading the pty:", err)
			}
			break
		}
	}
	stop()
	_ = wait()
	return conn.Close(websocket.StatusNormalClosure, "")
}

// KillAll ends the session and everything in it. Reset and restore call it, so
// a running generator cannot write into a site being wiped; the home directory
// and extrarepos/ stay. Failure is ignored: it mostly means no session exists.
func KillAll(logf execx.Logf) {
	logf("Ending the CLI console session")
	_, _ = execx.Output("", "runuser", "-u", userName, "--", "tmux", "kill-server")
}

// InstallTools adds what screenshots, browser checks and diagnosing bugs need, at
// run time so the image stays small for everyone who never uses them. A fixed
// package list with no arguments: this route is reachable by anything on the
// console port. Tools that only work as root, like tcpdump, are left out.
func InstallTools(logf execx.Logf) error {
	apt := []string{"DEBIAN_FRONTEND=noninteractive", "apt-get"}
	if err := execx.Run(logf, "", "env", append(apt, "update")...); err != nil {
		return err
	}
	if err := execx.Run(logf, "", "env", append(apt, "install", "-y", "--no-install-recommends",
		"chromium", "nodejs", "fonts-liberation",
		"python3", "ripgrep", "jq", "less", "vim", "nano", "tree", "file", "patch", "unzip", "zip", "rsync",
		"strace", "lsof", "htop", "psmisc", "iproute2", "bind9-dnsutils", "netcat-openbsd")...); err != nil {
		return err
	}
	// Drop the package indexes apt-get update fetched, as the Containerfile does.
	lists, err := filepath.Glob("/var/lib/apt/lists/*")
	if err != nil {
		return err
	}
	for _, path := range lists {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
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
