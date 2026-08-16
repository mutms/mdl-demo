package webui

import (
	"sync"

	"github.com/mutms/mdl-demo/internal/execx"
	"github.com/mutms/mdl-demo/internal/site"
)

// job is the one background operation (install or reset) the UI may run at
// a time, with its log kept for the progress section. Single-flight: a demo
// container has exactly one site, so there is never a queue.
type job struct {
	mu      sync.Mutex
	kind    string // "install" | "reset"
	lines   []string
	running bool
	err     error
	// adminPass is shown once on the success card and never persisted.
	adminPass string
	wwwroot   string
}

func (j *job) logf(line string) {
	j.mu.Lock()
	j.lines = append(j.lines, line)
	// Bound memory: keep the newest few thousand lines.
	if len(j.lines) > 4000 {
		j.lines = j.lines[len(j.lines)-4000:]
	}
	j.mu.Unlock()
}

// start launches fn in a goroutine unless a job is already running.
func (j *job) start(kind string, fn func(execx.Logf) error) bool {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return false
	}
	j.kind, j.running, j.err = kind, true, nil
	j.lines = nil
	j.mu.Unlock()

	go func() {
		err := fn(j.logf)
		j.mu.Lock()
		j.running, j.err = false, err
		j.mu.Unlock()
	}()
	return true
}

type jobView struct {
	Kind    string
	Running bool
	Failed  bool
	Error   string
	Tail    []string
	// Success-card fields, only set after a finished install.
	Wwwroot   string
	AdminPass string
}

func (j *job) view() jobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := jobView{Kind: j.kind, Running: j.running}
	if j.err != nil {
		v.Failed = true
		v.Error = j.err.Error()
	}
	tail := j.lines
	if len(tail) > 30 {
		tail = tail[len(tail)-30:]
	}
	v.Tail = append([]string(nil), tail...)
	if j.kind == "install" && !j.running && j.err == nil {
		v.Wwwroot, v.AdminPass = j.wwwroot, j.adminPass
	}
	return v
}

func (j *job) startInstall(o site.Options) bool {
	ok := j.start("install", func(logf execx.Logf) error {
		return site.Install(logf, o)
	})
	if ok {
		j.mu.Lock()
		j.adminPass, j.wwwroot = o.AdminPass, o.Wwwroot
		j.mu.Unlock()
	}
	return ok
}

func (j *job) startReset() bool {
	return j.start("reset", site.Reset)
}

func (j *job) idle() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return !j.running
}
