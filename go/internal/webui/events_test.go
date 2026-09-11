package webui

import (
	"testing"
	"time"

	"github.com/mutms/mdl-demo/go/internal/execx"
)

func TestHubCoalescesAndOrders(t *testing.T) {
	h := newHub()
	sub, ok := h.subscribe()
	if !ok {
		t.Fatal("subscribe refused")
	}
	// reload first, then three job hints: one job event, reload drained last.
	h.notify(event{evReload, "job"})
	h.notify(ev(evJob))
	h.notify(ev(evJob))
	h.notify(ev(evJob), event{evSSO, "abc"})
	<-sub.wake
	got := sub.drain()
	want := []event{{evJob, ""}, {evSSO, "abc"}, {evReload, "job"}}
	if len(got) != len(want) {
		t.Fatalf("drained %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %v, want %v", i, got[i], want[i])
		}
	}
	if len(sub.drain()) != 0 {
		t.Error("drain did not clear pending")
	}
}

// A subscriber that never reads must not stall the emitters.
func TestHubNeverBlocks(t *testing.T) {
	h := newHub()
	if _, ok := h.subscribe(); !ok {
		t.Fatal("subscribe refused")
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10000; i++ {
			h.notify(ev(evJob), ev(evLog))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("notify blocked on an undrained subscriber")
	}
}

func TestHubCap(t *testing.T) {
	h := newHub()
	for i := 0; i < maxSubscribers; i++ {
		if _, ok := h.subscribe(); !ok {
			t.Fatalf("subscribe %d refused below the cap", i)
		}
	}
	if _, ok := h.subscribe(); ok {
		t.Error("subscribe beyond the cap accepted")
	}
	var nilHub *hub
	nilHub.notify(ev(evJob)) // a job without a hub has nobody to tell
	nilHub.notifyLog()
}

// A burst of lines yields one immediate log event and one trailing one.
func TestLogThrottle(t *testing.T) {
	old := logGap
	logGap = 50 * time.Millisecond
	defer func() { logGap = old }()
	h := newHub()
	sub, _ := h.subscribe()
	count := func(wait time.Duration) int {
		n := 0
		deadline := time.After(wait)
		for {
			select {
			case <-sub.wake:
				for _, e := range sub.drain() {
					if e.Topic == evLog {
						n++
					}
				}
			case <-deadline:
				return n
			}
		}
	}
	for i := 0; i < 100; i++ {
		h.notifyLog()
	}
	if n := count(10 * time.Millisecond); n != 1 {
		t.Fatalf("immediate log events = %d, want 1", n)
	}
	if n := count(200 * time.Millisecond); n != 1 {
		t.Fatalf("trailing log events = %d, want 1", n)
	}
}

func TestTreeOnly(t *testing.T) {
	for kind, want := range map[string]bool{
		"plugin": true, "update": true,
		"install": false, "reset": false, "restore": false, "backup": false, "": false,
	} {
		if got := treeOnly(kind); got != want {
			t.Errorf("treeOnly(%q) = %t, want %t", kind, got, want)
		}
	}
}

// A job ending settles the watcher at once: the state change it caused is
// reported with the job, and the next tick has nothing left to say.
func TestJobEndSettles(t *testing.T) {
	h := newHub()
	sub, _ := h.subscribe()
	f := &fakeWorld{stamp: fileStamp{mod: time.Unix(1, 0)}, facts: siteFacts{installed: true, recipe: "x"}, pending: map[string]bool{}}
	w := f.watcher(h)
	h.watch = w
	j := &job{hub: h}
	done := make(chan struct{})
	j.start("reset", "", "", func(execx.Logf) error {
		f.stamp.mod, f.facts = time.Unix(2, 0), siteFacts{} // the site is gone
		return nil
	})
	go func() {
		for !j.idle() {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond) // let the goroutine's notifies land
		close(done)
	}()
	<-done
	if got := topics(drained(t, sub)); got != "job: state: reload:state " {
		t.Fatalf("job end: %q", got)
	}
	w.tick()
	if got := drained(t, sub); got != nil {
		t.Fatalf("next tick re-emitted %s", topics(got))
	}
}

// fakeWorld is the watcher's probes under test control.
type fakeWorld struct {
	stamp   fileStamp
	facts   siteFacts
	busy    bool
	tunURL  string
	tunStrt bool
	pending map[string]bool
}

func (f *fakeWorld) watcher(h *hub) *watcher {
	w := &watcher{
		hub:        h,
		stateStamp: func() fileStamp { return f.stamp },
		stateFacts: func() siteFacts { return f.facts },
		busy:       func() bool { return f.busy },
		tunnel:     func() (string, bool) { return f.tunURL, f.tunStrt },
		ssoPending: func(id string) bool { return f.pending[id] },
	}
	w.seed()
	return w
}

func drained(t *testing.T, sub *subscriber) []event {
	t.Helper()
	select {
	case <-sub.wake:
		return sub.drain()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func topics(evs []event) string {
	s := ""
	for _, e := range evs {
		s += e.Topic + ":" + e.Data + " "
	}
	return s
}

func TestWatcherTick(t *testing.T) {
	h := newHub()
	sub, _ := h.subscribe()
	f := &fakeWorld{stamp: fileStamp{mod: time.Unix(1, 0), size: 10}, pending: map[string]bool{}}
	w := f.watcher(h)

	w.tick()
	if got := drained(t, sub); got != nil {
		t.Fatalf("idle tick emitted %s", topics(got))
	}

	// state.json rewritten without the site changing: state only.
	f.stamp.mod = time.Unix(2, 0)
	w.tick()
	if got := topics(drained(t, sub)); got != "state: " {
		t.Fatalf("mtime-only change: %q", got)
	}

	// A site appears: state + reload.
	f.stamp.mod, f.facts = time.Unix(3, 0), siteFacts{installed: true, recipe: "moodle/release/5.2"}
	w.tick()
	if got := topics(drained(t, sub)); got != "state: reload:state " {
		t.Fatalf("install: %q", got)
	}

	// A CLI job holds busy.lock, then releases it.
	f.busy = true
	w.tick()
	if got := topics(drained(t, sub)); got != "job: " {
		t.Fatalf("busy: %q", got)
	}
	f.busy = false
	w.tick()
	if got := topics(drained(t, sub)); got != "job: " {
		t.Fatalf("idle again: %q", got)
	}

	// Tunnel comes up.
	f.tunURL = "https://x.trycloudflare.com"
	w.tick()
	if got := topics(drained(t, sub)); got != "tunnel: " {
		t.Fatalf("tunnel: %q", got)
	}

	// A watched login token is claimed: sso with its id, once.
	f.pending["tok1"] = true
	h.watchSSO("tok1")
	w.tick()
	if got := drained(t, sub); got != nil {
		t.Fatalf("pending token emitted %s", topics(got))
	}
	f.pending["tok1"] = false
	w.tick()
	if got := topics(drained(t, sub)); got != "sso:tok1 " {
		t.Fatalf("claimed token: %q", got)
	}
	w.tick()
	if got := drained(t, sub); got != nil {
		t.Fatalf("claimed token emitted again: %s", topics(got))
	}
}
