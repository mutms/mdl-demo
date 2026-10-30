// The CLI console page: xterm.js on one WebSocket. Binary frames are pty bytes,
// both ways; text frames are JSON control (the window size). See internal/term.
(function () {
  'use strict';

  const el = document.getElementById('term');
  const term = new Terminal({
    allowProposedApi: true, // the unicode addon needs it
    cursorBlink: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, "DejaVu Sans Mono", "Liberation Mono", monospace',
    fontSize: 14,
    lineHeight: 1.2, // at 1.0 rows clip what hangs below the baseline, such as _
    theme: { background: '#171717', foreground: '#cacaca', cursor: '#bbbbbb' },
  });

  // Some full-width rows end their line anyway: a table or a drawing, ending in | or
  // a box-drawing character or made only of rule characters like +-----+, and a
  // shell command continued with a backslash. A lone - = _ + at the end proves
  // nothing: URLs have those at any row break.
  const endsLine = function (row) {
    return /[|\\\u2500-\u259f]$/.test(row) || /^[-=_+|*#~.:\s\u2500-\u259f]+$/.test(row);
  };

  // Claude Code's login screen breaks its long URL into rows itself, so to the
  // terminal they are separate lines, and a click would open only the first. A URL
  // that fills its row to the last column continues on the next rows, up to one
  // with a space or that stops short.
  const wholeUrl = function (uri) {
    const buffer = term.buffer.active;
    for (let y = buffer.length - 1; y >= 0; y--) {
      const row = buffer.getLine(y).translateToString(true);
      const start = row.indexOf(uri);
      if (start < 0) {
        continue;
      }
      let url = row.slice(start);
      if (row.length < term.cols || /\s/.test(url) || endsLine(row)) {
        return uri;
      }
      for (let n = y + 1; n < buffer.length; n++) {
        const next = buffer.getLine(n).translateToString(true);
        if (!/^\S+$/.test(next) || endsLine(next)) {
          break;
        }
        url += next;
        if (next.length < term.cols) {
          break;
        }
      }
      return url;
    }
    return uri;
  };

  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.loadAddon(new WebLinksAddon.WebLinksAddon(function (ev, uri) {
    window.open(wholeUrl(uri), '_blank', 'noopener');
  }));
  term.loadAddon(new Unicode11Addon.Unicode11Addon());
  term.unicode.activeVersion = '11';
  // OSC 52: lets tmux and Claude Code's "copy the login URL" reach the clipboard.
  term.loadAddon(new ClipboardAddon.ClipboardAddon());
  term.open(el);
  fit.fit();

  // Copy and paste like any web page. With text selected, Ctrl+C copies; with
  // nothing selected it stays the interrupt it is in a shell. Returning false
  // hands the key to the browser, whose copy and paste events xterm.js handles,
  // which also works on plain http, where navigator.clipboard does not exist.
  term.attachCustomKeyEventHandler(function (ev) {
    if (ev.type !== 'keydown' || !ev.ctrlKey || ev.shiftKey || ev.altKey) {
      return true;
    }
    if (ev.key === 'c' && term.hasSelection()) {
      setTimeout(function () { term.clearSelection(); });
      return false;
    }
    return ev.key !== 'v';
  });

  // The copying half of wholeUrl: a selected row that fills the terminal's width
  // continues on the next one, unless endsLine says otherwise; a shorter row ends
  // its line. Rows the terminal already joined arrive as one line of several widths.
  // Double-width characters make a row's length miss the width, which only keeps a
  // line break.
  el.addEventListener('copy', function (ev) {
    const selection = term.getSelectionPosition();
    if (!selection) {
      return;
    }
    const text = term.getSelection();
    const lines = text.split(/\r?\n/);
    const joined = lines.reduce(function (out, line, i) {
      if (i === lines.length - 1) {
        return out + line;
      }
      const cells = line.length + (i === 0 ? selection.start.x : 0);
      return out + line + (cells > 0 && cells % term.cols === 0 && !endsLine(line) ? '' : '\n');
    }, '');
    if (joined !== text) {
      ev.clipboardData.setData('text/plain', joined);
      ev.preventDefault();
      ev.stopPropagation();
    }
  }, true);

  const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(scheme + '//' + location.host + '/cli/ws?csrf=' + encodeURIComponent(el.dataset.csrf));
  ws.binaryType = 'arraybuffer';

  const sendSize = function () {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
    }
  };
  const encoder = new TextEncoder();
  const send = function (data) {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(data);
    }
  };

  ws.addEventListener('open', function () {
    sendSize();
    term.focus();
  });
  ws.addEventListener('message', function (ev) {
    term.write(new Uint8Array(ev.data));
  });
  // A normal close means the shell client exited, usually because the session
  // ended. Anything else is the connection or the console going away.
  ws.addEventListener('close', function (ev) {
    document.getElementById(ev.code === 1000 ? 'cli-ended' : 'cli-lost').hidden = false;
    document.getElementById('cli-closed').hidden = false;
    fit.fit();
  });
  term.onData(function (data) {
    send(encoder.encode(data));
  });
  term.onBinary(function (data) {
    send(Uint8Array.from(data, function (c) { return c.charCodeAt(0); }));
  });
  term.onResize(sendSize);
  new ResizeObserver(function () { fit.fit(); }).observe(el);

  // A reload rather than a reconnect: it also picks up a restarted console, and
  // starts a fresh session if the old one ended.
  document.getElementById('cli-reconnect').addEventListener('click', function () {
    location.reload();
  });
})();
