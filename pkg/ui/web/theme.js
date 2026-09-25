(() => {
  'use strict';
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const normalize = value => value === 'light' ? 'lofi' : value === 'dark' ? 'night' : ['lofi', 'night'].includes(value) ? value : null;
  const read = () => { try { return normalize(localStorage.getItem('hb-theme')); } catch { return null; } };
  let preference = read();
  function apply() {
    const theme = preference || (media.matches ? 'night' : 'lofi');
    document.documentElement.dataset.theme = theme;
    document.querySelectorAll('[data-theme-toggle]').forEach(button => {
      const label = theme === 'night'
        ? button.dataset.themeLightLabel || 'Switch to light theme'
        : button.dataset.themeDarkLabel || 'Switch to dark theme';
      button.setAttribute('aria-label', label);
      button.setAttribute('title', label);
      button.setAttribute('aria-pressed', String(theme === 'night'));
    });
  }
  apply();
  document.addEventListener('DOMContentLoaded', () => {
    apply();
    window.lucide?.createIcons();
    document.querySelectorAll('[data-theme-toggle]').forEach(button => button.addEventListener('click', () => {
      preference = document.documentElement.dataset.theme === 'night' ? 'lofi' : 'night';
      try { localStorage.setItem('hb-theme', preference); } catch { /* The choice still applies for this page. */ }
      apply();
    }));
  });
  media.addEventListener('change', apply);
  window.addEventListener('storage', event => {
    if (event.key === 'hb-theme' || event.key === null) { preference = read(); apply(); }
  });
})();
