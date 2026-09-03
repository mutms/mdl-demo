// The console's only script. Every handler is delegated on document, so it
// survives htmx swaps; the markup carries data-* attributes, never on*
// handlers — the Content-Security-Policy allows nothing inline.

document.addEventListener('click', function (e) {
  var b = e.target.closest('[data-copy]');
  if (!b) return;
  var text = b.dataset.copy;
  // Flash the check on the copy icon, never on the masked value — swapping the
  // dots' content would make them jump. Clicking the dots or the icon both land
  // here; either way the feedback is on the icon and the ••••• stays put.
  var wrap = b.closest('.secret');
  var fb = (wrap && wrap.querySelector('button.copy')) || b;
  var done = function () {
    // Skip if the check is already showing (a double-click) — re-entering here
    // would capture the ✓ as "old" and restore it to ✓, leaving it stuck. The
    // copy above still happened; only the redundant flash is dropped.
    if (fb.classList.contains('ok')) return;
    var old = fb.innerHTML;
    fb.classList.add('ok');
    fb.innerHTML = '✓';
    setTimeout(function () { fb.classList.remove('ok'); fb.innerHTML = old; }, 1200);
  };
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(done);
  } else {
    var t = document.createElement('textarea');
    t.value = text;
    t.style.position = 'fixed';
    t.style.opacity = '0';
    document.body.appendChild(t);
    t.select();
    try { document.execCommand('copy'); done(); } catch (err) {}
    t.remove();
  }
});
document.addEventListener('click', function (e) {
  var c = e.target.closest('button.dlgclose');
  if (c) c.closest('dialog').close();
});
// Theme toggle: auto → light → dark. Auto = no data-theme, no stored value.
document.addEventListener('click', function (e) {
  var b = e.target.closest('#themebtn');
  if (!b) return;
  var cur = document.documentElement.dataset.theme || 'auto';
  var next = cur === 'auto' ? 'light' : cur === 'light' ? 'dark' : 'auto';
  try {
    if (next === 'auto') localStorage.removeItem('mdl-demo-theme');
    else localStorage.setItem('mdl-demo-theme', next);
  } catch (err) {}
  if (next === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = next;
  b.textContent = next === 'auto' ? '◐' : next === 'light' ? '☀' : '☾';
});
document.addEventListener('DOMContentLoaded', function () {
  // Enable tab hiding only now that JS runs (see .tabs.js in the stylesheet).
  document.querySelectorAll('.tabs').forEach(function (t) { t.classList.add('js'); });
  var b = document.getElementById('themebtn');
  if (!b) return;
  var t = document.documentElement.dataset.theme || 'auto';
  b.textContent = t === 'light' ? '☀' : t === 'dark' ? '☾' : '◐';
});
// The QR dialog lives outside the polled #site section, so a 5s refresh swap
// cannot close it mid-presentation; any click (or Esc) dismisses it.
document.addEventListener('click', function (e) {
  var q = e.target.closest('button.qr');
  if (!q) return;
  var d = document.getElementById('qrdialog');
  if (!d) return;
  d.querySelector('img').src = '/tunnel/qr.png?' + Date.now();
  d.showModal();
});
// Chooser: the shared name fields apply to whichever recipe gets installed —
// copied into the submitting row's form. Delegated; runs whether or not the
// confirm passes (harmless hidden inputs on a cancelled submit).
document.addEventListener('submit', function (e) {
  var f = e.target;
  if (f.getAttribute('action') !== '/install') return;
  ['fullname', 'shortname'].forEach(function (n) {
    var src = document.getElementById(n);
    if (!src) return;
    f.querySelectorAll('input[name="' + n + '"]').forEach(function (o) { o.remove(); });
    var h = document.createElement('input');
    h.type = 'hidden'; h.name = n; h.value = src.value;
    f.appendChild(h);
  });
});
// Backups page: "Restore" fills the dialog with the row's file name and
// resets the recipe choice to the bundled default. Delegated, so it survives
// the list fragment's htmx swaps.
document.addEventListener('click', function (e) {
  var b = e.target.closest('[data-restore]');
  if (!b) return;
  var f = document.getElementById('restorefile');
  if (!f) return;
  f.value = b.dataset.restore;
  document.getElementById('restorename').textContent = b.dataset.restore;
  document.querySelector('#restoredialog select').selectedIndex = 0;
  document.getElementById('restoredialog').showModal();
});
document.addEventListener('click', function (e) {
  var r = e.target.closest('button.reveal');
  if (!r) return;
  var val = r.parentNode.querySelector('.secret-val');
  if (!val) return;
  var shown = r.classList.toggle('on');
  val.textContent = shown ? val.dataset.secret : '••••••••';
  r.title = shown ? 'Hide' : 'Reveal';
});

// data-tab: switch the active vendor tab in the install chooser. The chooser
// never polls, so the chosen tab sticks; with JS off every panel just shows.
document.addEventListener('click', function (e) {
  var t = e.target.closest('[data-tab]');
  if (!t) return;
  var box = t.closest('.tabs');
  if (!box) return;
  var v = t.dataset.tab;
  box.querySelectorAll('[data-tab]').forEach(function (b) {
    b.classList.toggle('on', b.dataset.tab === v);
  });
  box.querySelectorAll('[data-tabpanel]').forEach(function (p) {
    p.classList.toggle('on', p.dataset.tabpanel === v);
  });
});
// data-confirm: ask before a destructive form submits.
// data-close-dialog: close that dialog once the form goes off (a new tab).
document.addEventListener('submit', function (e) {
  var f = e.target;
  if (f.dataset.confirm && !confirm(f.dataset.confirm)) {
    e.preventDefault();
    return;
  }
  if (f.dataset.closeDialog) {
    var d = document.getElementById(f.dataset.closeDialog);
    if (d) d.close();
  }
});
// data-open: open that dialog; data-clear empties an element first (the SSO
// dialog body, so a stale stage never shows while htmx fetches the new one).
document.addEventListener('click', function (e) {
  var b = e.target.closest('[data-open]');
  if (!b) return;
  var d = document.getElementById(b.dataset.open);
  if (!d) return;
  if (b.dataset.clear) {
    var c = document.getElementById(b.dataset.clear);
    if (c) c.innerHTML = '';
  }
  d.showModal();
});
// data-close on a <dialog>: "backdrop" closes on a click outside the box,
// "any" on any click.
document.addEventListener('click', function (e) {
  var d = e.target.closest('dialog');
  if (!d || !d.dataset.close) return;
  if (d.dataset.close === 'any' || e.target === d) d.close();
});
// The SSO poll answers with an HX-Trigger header once the code is claimed.
document.addEventListener('sso-done', function () {
  var d = document.getElementById('ssodialog');
  if (d && d.open) d.close();
});
