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
	dropped int // lines evicted from the front of lines, so an absolute line number stays a stable cursor
	running bool
	err     error
	// adminPass is shown once on the success card and never persisted.
	adminPass string
	wwwroot   string
}

func (j *job) logf(line string) {
	j.mu.Lock()
	j.lines = append(j.lines, line)
	// Bound memory: keep the newest few thousand lines. dropped tracks what
	// fell off the front so the log tail's cursor keeps counting through it.
	if len(j.lines) > 4000 {
		drop := len(j.lines) - 4000
		j.lines = j.lines[drop:]
		j.dropped += drop
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
	j.lines, j.dropped = nil, 0
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
	// Log is the batch of log lines to render — the recent tail on a full
	// section render, or just the new lines on an incremental /joblog poll.
	Log []string
	// Next is the absolute number of the first not-yet-sent line: the cursor
	// the log tail passes back so the next poll asks only for what came after.
	Next int
	// Success-card fields, only set after a finished install.
	Wwwroot   string
	AdminPass string
}

// logTailLimit bounds how many trailing lines a full render carries; the
// incremental poll streams in everything after that.
const logTailLimit = 400

func (j *job) view() jobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := jobView{Kind: j.kind, Running: j.running}
	if j.err != nil {
		v.Failed = true
		v.Error = j.err.Error()
	}
	tail := j.lines
	if len(tail) > logTailLimit {
		tail = tail[len(tail)-logTailLimit:]
	}
	v.Log = append([]string(nil), tail...)
	v.Next = j.dropped + len(j.lines)
	if j.kind == "install" && !j.running && j.err == nil {
		v.Wwwroot, v.AdminPass = j.wwwroot, j.adminPass
	}
	return v
}

// logSince returns only the log lines at or after absolute line number from,
// plus the new cursor — what the incremental log tail polls for. A from that
// predates the retained window (evicted lines) simply starts at the oldest
// line still held.
func (j *job) logSince(from int) jobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	start := from - j.dropped
	if start < 0 {
		start = 0
	}
	if start > len(j.lines) {
		start = len(j.lines)
	}
	return jobView{
		Kind:    j.kind,
		Running: j.running,
		Log:     append([]string(nil), j.lines[start:]...),
		Next:    j.dropped + len(j.lines),
	}
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
