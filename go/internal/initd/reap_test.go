package initd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"testing"
)

// TestStartChildRoutesStatus is the regression guard for the PID-1 reaper race.
//
// With a central Wait4(-1) reaper running, a child started through StartChild
// must get its real exit status back through the reaper — not the ECHILD
// ("waitid: no child processes") that a competing cmd.Wait() would return once
// the reaper reaped the child first. Many children run concurrently, half of
// them failing, so a stolen or misrouted status shows up as the wrong error
// rather than an occasional flake.
func TestStartChildRoutesStatus(t *testing.T) {
	s := &Supervisor{
		byPID:       map[int]*proc{},
		execWaiters: map[int]chan syscall.WaitStatus{},
	}

	sigchld := make(chan os.Signal, 16)
	signal.Notify(sigchld, syscall.SIGCHLD)
	go s.reap(sigchld)

	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			code := 0
			if i%2 == 1 {
				code = 3
			}

			wait, err := s.StartChild(exec.Command("sh", "-c", fmt.Sprintf("exit %d", code)))
			if err != nil {
				t.Errorf("child %d: StartChild: %v", i, err)
				return
			}

			err = wait()
			switch {
			case code == 0 && err != nil:
				t.Errorf("child %d (exit 0): got %v, want nil", i, err)
			case code != 0 && (err == nil || err.Error() != "exit status 3"):
				t.Errorf("child %d (exit 3): got %v, want \"exit status 3\"", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Let the reaper goroutine finish so it cannot outlive the test and steal
	// a later test's subprocess status.
	signal.Stop(sigchld)
	close(sigchld)
}
