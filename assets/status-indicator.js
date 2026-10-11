(function () {
  'use strict';
  function tracker(initialKey, initialCount, initialID, changed) {
    const failures = new Map();
    const latest = new Map();
    let sequence = 0;
    function publish() { changed(Array.from(failures.values()).at(-1) || ''); }
    function begin(key) { const n = ++sequence; latest.set(key, n); return {key, n}; }
    function cancel(ticket) { if (latest.get(ticket.key) === ticket.n) latest.delete(ticket.key); }
    function finish(ticket, count, id) {
      if (latest.get(ticket.key) !== ticket.n) return;
      if (!/^\d+$/.test(String(count)) || !/^hb-\d+$/.test(id || '')) { cancel(ticket); return; }
      if (Number(count) > 0) failures.set(ticket.key, id);
      else failures.delete(ticket.key);
      // Discard completed successful request bookkeeping.
      if (!failures.has(ticket.key)) latest.delete(ticket.key);
      publish();
    }
    finish(begin(initialKey), initialCount, initialID);
    return {begin, finish, cancel};
  }
  if (typeof window === 'undefined' && typeof module !== 'undefined' && module.exports) { module.exports = tracker; return; }
  const script = document.currentScript;
  if (!script || window.__hyperbricksStatusIndicator) return;
  window.__hyperbricksStatusIndicator = true;
  let failedID = '', button, tooltip, host, hideTimer;
  function paint(id) {
    failedID = id;
    if (!button) return;
    const wasOK = button.dataset.status === 'ok';
    button.className = 'badge badge-soft ' + (id ? 'badge-error' : 'badge-success');
    button.textContent = id ? '● errors' : '● ok';
    button.setAttribute('aria-label', id ? 'Render error: open Errors in developer tab' : 'No render errors: open Dashboard in developer tab');
    button.dataset.status = id ? 'error' : 'ok';
    tooltip.setAttribute('data-tip', id ? 'Open HyperBricks Diagnostics' : 'Open HyperBricks Dashboard');
    if (id || !wasOK) {
      window.clearTimeout(hideTimer);
      host.style.display = 'block';
      if (!id) hideTimer = window.setTimeout(() => { host.style.display = 'none'; }, 5000);
    }
  }
  const state = tracker(key('GET', location.href), script.dataset.errors, script.dataset.request, paint);
  function key(method, value) {
    try {
      const url = new URL(value, location.href);
      url.hash = '';
      if (url.origin !== location.origin || url.pathname.startsWith('/__hyperbricks/')) return null;
      return String(method || 'GET').toUpperCase() + ' ' + url.href;
    } catch (_) { return null; }
  }
  // Observation must never change the application's response or exception.
  function observe(action) { try { return action(); } catch (_) { return undefined; } }
  const nativeFetch = window.fetch;
  if (nativeFetch) window.fetch = function(input, options) {
    const ticket = observe(() => {
      const k = key(options?.method || input?.method, typeof input === 'string' || input instanceof URL ? input : input?.url);
      return k && state.begin(k);
    });
    let pending;
    try { pending = nativeFetch.apply(this, arguments); }
    catch (error) { observe(() => ticket && state.cancel(ticket)); throw error; }
    return pending.then(response => {
      observe(() => {
        if (ticket && new URL(response.url || location.href).origin === location.origin) state.finish(ticket, response.headers.get('X-Hyperbricks-Render-Error-Count'), response.headers.get('X-Hyperbricks-Request-ID'));
      });
      observe(() => ticket && state.cancel(ticket));
      return response;
    }, error => { observe(() => ticket && state.cancel(ticket)); throw error; });
  };
  if (typeof XMLHttpRequest !== 'undefined') {
    const open = XMLHttpRequest.prototype.open, send = XMLHttpRequest.prototype.send;
    const requests = new WeakMap(), pending = new WeakMap();
    XMLHttpRequest.prototype.open = function(method, url) {
      const result = open.apply(this, arguments);
      observe(() => pending.get(this)?.());
      observe(() => requests.set(this, key(method, url)));
      return result;
    };
    XMLHttpRequest.prototype.send = function() {
      let cleanup;
      observe(() => {
        const k = requests.get(this);
        // A second send on an active XHR must leave its existing observer intact.
        if (!k || pending.has(this)) return;
        const ticket = state.begin(k);
        let disposed = false;
        const onLoad = () => observe(() => {
          if (!disposed && this.responseURL && new URL(this.responseURL).origin === location.origin) state.finish(ticket, this.getResponseHeader('X-Hyperbricks-Render-Error-Count'), this.getResponseHeader('X-Hyperbricks-Request-ID'));
        });
        cleanup = () => {
          if (disposed) return;
          disposed = true;
          observe(() => this.removeEventListener('load', onLoad));
          observe(() => this.removeEventListener('loadend', cleanup));
          observe(() => state.cancel(ticket));
          if (pending.get(this) === cleanup) pending.delete(this);
        };
        pending.set(this, cleanup);
        try {
          this.addEventListener('load', onLoad);
          this.addEventListener('loadend', cleanup);
        } catch (_) { cleanup(); }
      });
      try { return send.apply(this, arguments); }
      catch (error) { observe(() => cleanup?.()); throw error; }
    };
  }
  function mount() {
    host = document.createElement('hyperbricks-status');
    // Isolate the control from application CSS; no application framework is required.
    host.style.cssText = 'all:initial;position:fixed;right:16px;bottom:16px;z-index:2147483647;display:block';
    const root = host.attachShadow({mode:'open'});
    const style = document.createElement('link');
    style.rel = 'stylesheet';
    style.href = '/__hyperbricks/status-indicator.css';
    button = document.createElement('button');
    button.type = 'button';
    button.setAttribute('aria-live','polite');
    button.addEventListener('click', () => {
      const url = failedID ? '/__hyperbricks/errors?request_id=' + encodeURIComponent(failedID) : '/__hyperbricks/dashboard';
      const target = window.open(url, 'hyperbricks-devtools');
      if (target) target.focus();
    });
    tooltip = document.createElement('span');
    tooltip.className = 'tooltip tooltip-top';
    tooltip.setAttribute('data-theme', 'lofi');
    observe(() => {
      const media = window.matchMedia('(prefers-color-scheme: dark)');
      const applyTheme = () => observe(() => {
        const saved = observe(() => window.localStorage.getItem('hb-theme'));
        const preference = saved === 'light' ? 'lofi' : saved === 'dark' ? 'night' : saved;
        tooltip.setAttribute('data-theme', ['lofi', 'night'].includes(preference) ? preference : media.matches ? 'night' : 'lofi');
      });
      applyTheme();
      media.addEventListener('change', applyTheme);
      window.addEventListener('storage', event => {
        if (event.key === 'hb-theme' || event.key === null) applyTheme();
      });
    });
    tooltip.append(button);
    root.append(style, tooltip);
    document.body.append(host);
    paint(failedID);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => observe(mount), {once:true});
  else observe(mount);
})();
