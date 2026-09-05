package webui

import (
	"sync"
	"sync/atomic"

	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/site"
)

// job is the one background operation (install or reset) the UI may run at
// a time, with its log kept for the progress section. Single-flight: a demo
// container has exactly one site, so there is never a queue.
type job struct {
	mu       sync.Mutex
	kind     string // "install" | "reset"
	recipe   string // the recipe being installed, so the busy card can name it
	siteName string // the site's name, for the busy card during install
	lines    []string
	dropped  int // lines evicted from the front of lines, so an absolute line number stays a stable cursor
	running  bool
	err      error
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

// logSink is the running server's job: anything noteworthy outside the
// install/reset flow — cron ticks, and whatever comes next — streams into
// the Site log through it (the UI runs in-process under mdl-demo init).
// Atomic because Serve and the writers start as sibling goroutines.
var logSink atomic.Pointer[job]

// SiteLog appends one line to the Site log; callers prefix their lines
// ("cron: …") so sources stay tellable apart. Lines are dropped until a
// job has run — the Site log card only renders once one exists.
func SiteLog(line string) {
	j := logSink.Load()
	if j == nil {
		return
	}
	j.mu.Lock()
	kind := j.kind
	j.mu.Unlock()
	if kind == "" {
		return
	}
	j.logf(line)
}

// start launches fn in a goroutine unless a job is already running. recipe and
// name identify an install (so the busy card can name it), "" otherwise.
func (j *job) start(kind, recipe, name string, fn func(execx.Logf) error) bool {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return false
	}
	j.kind, j.recipe, j.siteName, j.running, j.err = kind, recipe, name, true, nil
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
	// Label is what the running badge says — the activity, not a bare
	// "running": installing, resetting, backing up, restoring.
	Label    string
	Recipe   string // the recipe being installed, for the busy card
	SiteName string // the site's name, for the busy card
	Failed   bool
	Error    string
	// Log is the batch of log lines to render — the recent tail on a full
	// section render, or just the new lines on an incremental /joblog poll.
	Log []string
	// Next is the absolute number of the first not-yet-sent line: the cursor
	// the log tail passes back so the next poll asks only for what came after.
	Next int
}

// jobLabels turns a job kind into the running badge's text (the English
// catalog key, translated in the template).
var jobLabels = map[string]string{
	"install": "installing",
	"reset":   "resetting",
	"backup":  "backing up",
	"restore": "restoring",
	"plugin":  "adding plugin",
}

// logTailLimit bounds how many trailing lines a full render carries; the
// incremental poll streams in everything after that.
const logTailLimit = 400

func (j *job) view() jobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := jobView{Kind: j.kind, Running: j.running, Label: jobLabels[j.kind], Recipe: j.recipe, SiteName: j.siteName}
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
	return j.start("install", o.Recipe, o.Fullname, func(logf execx.Logf) error {
		return site.Install(logf, o)
	})
}

func (j *job) startReset() bool {
	return j.start("reset", "", "", site.Reset)
}

func (j *job) startBackup(version, name string) bool {
	return j.start("backup", "", "", func(logf execx.Logf) error {
		_, err := site.Backup(logf, version, name)
		return err
	})
}

func (j *job) startAddPlugin(url, ref string, backupFirst bool, version string) bool {
	return j.start("plugin", "", "", func(logf execx.Logf) error {
		return site.AddPlugin(logf, url, ref, backupFirst, version)
	})
}

func (j *job) startRestore(o site.RestoreOptions) bool {
	return j.start("restore", o.Recipe, "", func(logf execx.Logf) error {
		return site.Restore(logf, o)
	})
}

func (j *job) idle() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return !j.running
}
