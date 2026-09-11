package webui

// Live updates. Every page holds one GET /events stream (server-sent events)
// and nothing in the console runs on a timer: when something changes, the
// server pushes a hint naming what changed, and the sections that show it
// refetch themselves (hx-trigger="mdl:<topic> from:body" in the templates,
// app.js re-dispatching each event on <body>).
//
// Events are hints, never payload — every fetch they cause is idempotent, so
// coalescing (one pending slot per topic) and dropping on a slow client are
// both safe: the client ends up fetching the current state either way.
//
// One emitter per fact. What only this process knows (a job starting or
// ending, log lines, a service going up or down) is emitted at the point it
// happens. Anything another process can change too — state.json (`mdl-demo
// url`, a CLI install), busy.lock (a CLI job), the tunnel URL (cloudflared
// dying), an SSO token file (PHP claims it) — is emitted only by the one
// watcher below, which compares those every second. Handlers never emit what
// the watcher can see, so there is exactly one place deciding each event.

import (
	"os"
	"sync"
	"time"

	"github.com/mutms/mdl-demo/go/internal/sso"
	"github.com/mutms/mdl-demo/go/internal/state"
	"github.com/mutms/mdl-demo/go/internal/tunnel"
)

// Topics. Each names the fact that changed; see the table in AGENTS.md.
const (
	evReload   = "reload"   // the page is stale: new process, site identity or code changed
	evJob      = "job"      // the single-flight job started or ended (also a CLI job's busy.lock)
	evLog      = "log"      // new Site log lines
	evState    = "state"    // state.json changed without the site identity changing (users, URL)
	evTunnel   = "tunnel"   // tunnel URL or starting state changed
	evServices = "services" // a supervised service went up or down
	evSSO      = "sso"      // a single-use login token was claimed or expired (data: its id)
)

// topicOrder is the order events leave one flush: reload last, so the
// sections refresh first — a reload may be held back while a dialog is open.
var topicOrder = []string{evJob, evLog, evState, evTunnel, evServices, evSSO, evReload}

type event struct{ Topic, Data string }

func ev(topic string) event { return event{Topic: topic} }

const (
	// maxSubscribers bounds open streams: a local console, so a few tabs.
	maxSubscribers = 64
	// maxSSOWatched bounds the pending login tokens the watcher checks.
	maxSSOWatched = 32
)

// logGap throttles log events: a fast install writes hundreds of lines a
// second, and each event costs the page one /joblog fetch. A trailing event
// after the last burst guarantees nothing is left unfetched.
var logGap = 250 * time.Millisecond

// subscriber is one open /events response. pending is the coalescing buffer —
// one slot per topic, latest data wins — and wake has capacity one, so notify
// never blocks: a slow client simply drains once with everything folded in.
type subscriber struct {
	mu      sync.Mutex
	pending map[string]string
	wake    chan struct{}
}

func (s *subscriber) add(e event) {
	s.mu.Lock()
	s.pending[e.Topic] = e.Data
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// drain returns what is pending, in topicOrder, and clears it.
func (s *subscriber) drain() []event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []event
	for _, t := range topicOrder {
		if d, ok := s.pending[t]; ok {
			out = append(out, event{t, d})
			delete(s.pending, t)
		}
	}
	return out
}

type hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
	// watch is the watcher, once running, so a job ending can have the files
	// it changed looked at right away (settle) rather than at the next tick.
	watch *watcher
	// sso holds the login token ids minted by this process that the watcher
	// checks each tick (id → expiry), so a claimed QR closes its dialog.
	sso map[string]time.Time

	logMu    sync.Mutex
	logLast  time.Time
	logTimer *time.Timer
}

func newHub() *hub {
	return &hub{subs: map[*subscriber]struct{}{}, sso: map[string]time.Time{}}
}

// events is the process-wide hub: Serve wires it into the Server and the job;
// initd reaches it through Notify, as cron reaches the log through SiteLog.
var events = newHub()

// Notify tells every open console page that topic changed. Exported for
// initd (service up/down); everything in this package calls the hub directly.
func Notify(topic string) { events.notify(ev(topic)) }

func (h *hub) subscribe() (*subscriber, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) >= maxSubscribers {
		return nil, false
	}
	s := &subscriber{pending: map[string]string{}, wake: make(chan struct{}, 1)}
	h.subs[s] = struct{}{}
	return s, true
}

func (h *hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// notify queues evs for every subscriber. Nil-safe: a job built without a hub
// (tests) just has nobody to tell.
func (h *hub) notify(evs ...event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		for _, e := range evs {
			s.add(e)
		}
	}
}

// notifyLog emits a log event at most once per logGap, always followed by a
// trailing one when lines kept arriving inside the gap.
func (h *hub) notifyLog() {
	if h == nil {
		return
	}
	h.logMu.Lock()
	defer h.logMu.Unlock()
	now := time.Now()
	if since := now.Sub(h.logLast); since >= logGap {
		h.logLast = now
		h.notify(ev(evLog))
		return
	} else if h.logTimer == nil {
		h.logTimer = time.AfterFunc(logGap-since, func() {
			h.logMu.Lock()
			h.logTimer = nil
			h.logLast = time.Now()
			h.logMu.Unlock()
			h.notify(ev(evLog))
		})
	}
}

// watchSSO registers a freshly minted login token for the watcher. Beyond
// maxSSOWatched the oldest is dropped — its dialog then just stays open until
// closed by hand, which is harmless.
func (h *hub) watchSSO(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sso) >= maxSSOWatched {
		var oldest string
		var when time.Time
		for k, t := range h.sso {
			if oldest == "" || t.Before(when) {
				oldest, when = k, t
			}
		}
		delete(h.sso, oldest)
	}
	h.sso[id] = time.Now().Add(sso.TTL)
}

// settledSSO removes and returns the watched ids that are no longer pending
// (claimed by the site, or expired).
func (h *hub) settledSSO(pending func(string) bool, now time.Time) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var done []string
	for id, exp := range h.sso {
		if now.After(exp) || !pending(id) {
			delete(h.sso, id)
			done = append(done, id)
		}
	}
	return done
}

// settle runs one watcher pass now. A job that just ended wrote state.json and
// released busy.lock; looking at once (instead of within the next second) makes
// the resulting state/reload events part of the job's own ending, and keeps
// the watcher the only one deciding them — no second reload a tick later.
func (h *hub) settle() {
	if h == nil || h.watch == nil {
		return
	}
	h.watch.tick()
}

// treeOnly reports whether a finished job of this kind changed the code tree
// without touching state.json — a plugin added, branches pulled — so nothing
// the watcher looks at moved, and the job itself must say the page is stale.
// install, reset and restore change the site identity in state.json; the
// watcher sees that and emits the reload (see settle). A backup only reads.
func treeOnly(kind string) bool {
	return kind == "plugin" || kind == "update"
}

// siteFacts is the part of state.json that decides what the dashboard is: an
// installed site, and which recipe. A change means reload, not refresh.
type siteFacts struct {
	installed bool
	recipe    string
}

func reloadOn(prev, cur siteFacts) bool { return prev != cur }

// fileStamp identifies a version of state.json; Save writes a new file
// (rename), so mtime moves on every change — size guards a same-instant write.
type fileStamp struct {
	mod  time.Time
	size int64
}

// watcher is the one poller: it compares the facts other processes can change
// once a second and turns a difference into events. Probes are fields so a
// test substitutes fakes and drives tick() by hand.
type watcher struct {
	mu         sync.Mutex // one pass at a time: the ticker's and settle's
	hub        *hub
	stateStamp func() fileStamp // a stat, every tick
	stateFacts func() siteFacts // a read, only when the stamp moved
	busy       func() bool
	tunnel     func() (url string, starting bool)
	ssoPending func(id string) bool
	// last seen
	stamp       fileStamp
	facts       siteFacts
	busyNow     bool
	tunURL      string
	tunStarting bool
}

func newWatcher(h *hub) *watcher {
	w := &watcher{
		hub:        h,
		stateStamp: stateStamp,
		stateFacts: stateFacts,
		busy:       state.Busy,
		tunnel:     func() (string, bool) { return tunnel.URL(), tunnel.Starting() },
		ssoPending: sso.Pending,
	}
	w.seed()
	h.watch = w
	return w
}

// stateStamp identifies the state.json on disk; a missing file (never
// installed, or a CLI reset removed it) is the zero stamp.
func stateStamp() fileStamp {
	fi, err := os.Stat(state.Path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: fi.ModTime(), size: fi.Size()}
}

func stateFacts() siteFacts {
	if st, err := state.Load(); err == nil && st.Installed() {
		return siteFacts{installed: true, recipe: st.Recipe}
	}
	return siteFacts{}
}

// seed records the current facts without emitting anything: a page rendered
// after this already shows them.
func (w *watcher) seed() {
	w.stamp, w.facts = w.stateStamp(), w.stateFacts()
	w.busyNow = w.busy()
	w.tunURL, w.tunStarting = w.tunnel()
}

// tick is one comparison pass.
func (w *watcher) tick() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if stamp := w.stateStamp(); stamp != w.stamp {
		w.stamp = stamp
		facts := w.stateFacts()
		if reloadOn(w.facts, facts) {
			w.hub.notify(ev(evState), event{evReload, "state"})
		} else {
			w.hub.notify(ev(evState))
		}
		w.facts = facts
	}
	if b := w.busy(); b != w.busyNow {
		w.busyNow = b
		w.hub.notify(ev(evJob))
	}
	if u, st := w.tunnel(); u != w.tunURL || st != w.tunStarting {
		w.tunURL, w.tunStarting = u, st
		w.hub.notify(ev(evTunnel))
	}
	for _, id := range w.hub.settledSSO(w.ssoPending, time.Now()) {
		w.hub.notify(event{evSSO, id})
	}
}

func (w *watcher) run() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for range tick.C {
		w.tick()
	}
}
