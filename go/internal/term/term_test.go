package term

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// dial attaches a client to a server running Attach on a plain shell: tests do
// not run as root, so they cannot drop to www-data, and tmux need not exist.
func dial(t *testing.T) (*websocket.Conn, context.Context) {
	t.Helper()
	orig := newCommand
	newCommand = func() (*exec.Cmd, error) { return exec.Command("/bin/sh"), nil }
	t.Cleanup(func() { newCommand = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_ = Attach(r.Context(), conn)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn, ctx
}

// readUntil collects binary output until it contains want.
func readUntil(t *testing.T, ctx context.Context, conn *websocket.Conn, want string) string {
	t.Helper()
	var got strings.Builder
	for !strings.Contains(got.String(), want) {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("reading: %v (got %q, want %q)", err, got.String(), want)
		}
		if kind != websocket.MessageBinary {
			t.Fatalf("output must be binary frames, got %v", kind)
		}
		got.Write(data)
	}
	return got.String()
}

func send(t *testing.T, ctx context.Context, conn *websocket.Conn, line string) {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageBinary, []byte(line+"\n")); err != nil {
		t.Fatal(err)
	}
}

func TestAttachRoundTrip(t *testing.T) {
	conn, ctx := dial(t)
	// The marker is split in the command so the echoed input cannot satisfy the read.
	send(t, ctx, conn, "echo rou''nd-trip")
	readUntil(t, ctx, conn, "round-trip")
}

func TestAttachResize(t *testing.T) {
	conn, ctx := dial(t)
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"cols":123,"rows":45}`)); err != nil {
		t.Fatal(err)
	}
	// Bad control messages are ignored, not fatal.
	for _, bad := range []string{`not json`, `{"cols":0,"rows":0}`} {
		if err := conn.Write(ctx, websocket.MessageText, []byte(bad)); err != nil {
			t.Fatal(err)
		}
	}
	send(t, ctx, conn, "stty size; echo si''ze-done")
	if got := readUntil(t, ctx, conn, "size-done"); !strings.Contains(got, "45 123") {
		t.Fatalf("the pty did not take the size: %q", got)
	}
}

// When the shell exits, the client must hear a normal close: that is what shows
// the page's Reconnect button.
func TestAttachClosesWhenShellExits(t *testing.T) {
	conn, ctx := dial(t)
	send(t, ctx, conn, "exit")
	for {
		_, _, err := conn.Read(ctx)
		if err == nil {
			continue
		}
		if s := websocket.CloseStatus(err); s != websocket.StatusNormalClosure {
			t.Fatalf("close status = %v (%v)", s, err)
		}
		return
	}
}
