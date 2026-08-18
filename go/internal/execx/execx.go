// Package execx runs external commands with their output streamed line by
// line to a logger, so long operations (mudev clone, Moodle install) surface
// progress in the CLI and the web UI's install log alike.
package execx

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Logf receives one line of command output or progress narration.
type Logf func(line string)

// StartChild, when set, starts cmd and returns a function that blocks for its
// exit status — replacing cmd.Start()/cmd.Wait().
//
// It exists for code running inside PID 1 (the web UI and cron). There the
// container's init reaps every child with a central Wait4(-1); a competing
// cmd.Wait() loses that race and returns ECHILD, surfaced as the baffling
// "waitid: no child processes" even when the command itself succeeded. initd
// installs a hook here that routes the status through the reaper instead. In a
// standalone CLI process (no such reaper) it stays nil and the plain
// Start/Wait path below is used.
var StartChild func(cmd *exec.Cmd) (wait func() error, err error)

// startChild starts cmd and returns its wait function, via the reaper-aware
// hook when one is installed, otherwise via os/exec directly.
func startChild(cmd *exec.Cmd) (func() error, error) {
	if StartChild != nil {
		return StartChild(cmd)
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd.Wait, nil
}

// Run executes name args... in dir (empty = inherited cwd), streaming
// combined stdout+stderr lines to logf.
func Run(logf Logf, dir string, name string, args ...string) error {
	logf("$ " + name + " " + strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	wait, err := startChild(cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	// The routed wait path does not run cmd.Wait(), so nobody else closes the
	// read end — do it here. In the plain path cmd.Wait() closes it too, and
	// the second Close is a harmless no-op.
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		logf(scanner.Text())
	}

	if err := wait(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// Output runs the command quietly and returns its trimmed stdout.
func Output(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	wait, err := startChild(cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}

	defer outPipe.Close()
	defer errPipe.Close()

	// Read both pipes to EOF in this process rather than through cmd.Wait()'s
	// copier goroutines, which the routed (PID 1) wait path does not run.
	// Concurrently, so a command that fills one pipe's buffer cannot deadlock
	// against us draining the other. stdout stays separate from stderr, so a
	// NOTICE on stderr never pollutes the value callers parse.
	var stderr []byte
	done := make(chan struct{})
	go func() {
		stderr, _ = io.ReadAll(errPipe)
		close(done)
	}()
	stdout, _ := io.ReadAll(outPipe)
	<-done

	if err := wait(); err != nil {
		if msg := strings.TrimSpace(string(stderr)); msg != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(stdout)), nil
}
