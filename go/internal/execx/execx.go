// Package execx runs external commands with their output streamed line by
// line to a logger, so long operations (mudev clone, Moodle install) surface
// progress in the CLI and the web UI's install log alike.
package execx

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// Logf receives one line of command output or progress narration.
type Logf func(line string)

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

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		logf(scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// Output runs the command quietly and returns its trimmed stdout.
func Output(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}
