// Runs synchronously before the body parses, so the stored theme is on
// <html> before first paint.
try {
  var _t = localStorage.getItem('mdl-demo-theme');
  if (_t === 'light' || _t === 'dark') document.documentElement.dataset.theme = _t;
} catch (e) {}
