// Applies the stored or system theme before React renders, to avoid a light
// flash in dark mode. Kept in sync with src/lib/theme.ts (key "theme").
(function () {
  var stored = null;
  try {
    stored = window.localStorage.getItem('theme');
  } catch (_) {
    // Storage can be unavailable (private mode, blocked site data).
  }
  var dark = stored ? stored === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (dark) document.documentElement.classList.add('dark');
})();
