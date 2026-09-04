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
// Progressive enhancement for the install chooser. Runs on first load AND
// after every htmx swap: the empty-state chooser reappears via the site card's
// poll when a reset/restore job ends, without a full page load — so this must
// re-init the swapped-in markup, or the tabs and Install button stay inert
// until a manual reload. Idempotent, so re-running over old nodes is harmless.
function initChooser() {
  // Enable tab hiding only now that JS runs (see .tabs.js in the stylesheet).
  document.querySelectorAll('.tabs').forEach(function (t) { t.classList.add('js'); });
  // Same idea for the Install button: disable it until a package is picked
  // only when JS can re-enable it (the change handler above).
  document.querySelectorAll('.installform button.install').forEach(function (b) {
    if (!b.dataset.base) b.dataset.base = b.textContent.trim(); // the plain "Install" verb
    b.disabled = !b.closest('.installform').querySelector('input[name="recipe"]:checked');
  });
}
document.addEventListener('DOMContentLoaded', initChooser);
document.addEventListener('htmx:afterSwap', initChooser); // htmx events bubble to document
document.addEventListener('DOMContentLoaded', function () {
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
// Install chooser: the button stays disabled until a version package is
// picked, so "pick one, then Install" needs no confirm dialog.
document.addEventListener('change', function (e) {
  var r = e.target.closest('.installform input[name="recipe"]');
  if (!r) return;
  var f = r.closest('.installform');
  var btn = f && f.querySelector('button.install');
  if (!btn) return;
  btn.disabled = false;
  // Name the choice on the button ("Install MuTMS 5.2.2.01"), so it is clear
  // what is selected even after switching to another vendor's tab.
  if (btn.dataset.base && r.dataset.pick) btn.textContent = btn.dataset.base + ' ' + r.dataset.pick;
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
// Tunnel switch: starting cloudflared takes a few seconds and the request is a
// full navigation, so show the spinner and disable the switch immediately —
// otherwise the wait looks frozen. Also stop the tools card's poll so it can't
// swap the spinner away mid-wait; the redirect reloads the page regardless.
document.addEventListener('submit', function (e) {
  var f = e.target;
  if (!f.classList || !f.classList.contains('tunnelform')) return;
  f.classList.add('busy');
  var b = f.querySelector('button');
  if (b) b.disabled = true;
  var card = f.closest('#tools');
  if (card) card.removeAttribute('hx-trigger');
});
// data-confirm: ask before a destructive form submits, using the console's own
// confirm dialog (never the browser's native confirm popup). The submit is held
// and the form kept in pendingConfirm; clicking the dialog's confirm button
// re-submits it with a flag set so this handler lets it straight through.
// data-close-dialog: close that dialog once the form goes off (a new tab).
var pendingConfirm = null;
document.addEventListener('submit', function (e) {
  var f = e.target;
  if (f.dataset.confirm && !f.dataset.confirmed) {
    e.preventDefault();
    var d = document.getElementById('confirmdialog');
    if (!d) { if (confirm(f.dataset.confirm)) f.submit(); return; } // no dialog on page: fall back
    d.querySelector('.msg').textContent = f.dataset.confirm;
    pendingConfirm = f;
    d.showModal();
    return;
  }
  if (f.dataset.closeDialog) {
    var cd = document.getElementById(f.dataset.closeDialog);
    if (cd) cd.close();
  }
});
// Confirm dialog buttons: OK re-submits the held form; anything else drops it.
document.addEventListener('click', function (e) {
  if (e.target.closest('[data-confirm-ok]')) {
    var f = pendingConfirm;
    pendingConfirm = null;
    document.getElementById('confirmdialog').close();
    if (f) { f.dataset.confirmed = '1'; f.requestSubmit ? f.requestSubmit() : f.submit(); }
    return;
  }
  if (e.target.closest('[data-confirm-cancel]')) {
    pendingConfirm = null;
    document.getElementById('confirmdialog').close();
  }
});
// Dropping the held form whenever the confirm dialog closes any other way
// (backdrop click, Esc, ×) keeps a stale form from firing on the next confirm.
document.addEventListener('close', function (e) {
  if (e.target.id === 'confirmdialog') pendingConfirm = null;
}, true);
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
