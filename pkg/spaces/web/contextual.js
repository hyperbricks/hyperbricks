(() => {
  'use strict';

  // The development-only response supplies the Space and its authorized fields.
  // Markers describe ownership; they never authorize a field on their own.
  const source = document.getElementById('hb-spaces-context');
  if (!source || document.getElementById('hb-spaces-tools')) return;
  const scriptURL = document.currentScript?.src;
  let context;
  try {
    context = JSON.parse(source.textContent);
  } catch {
    return;
  }
  if (!context || typeof context !== 'object') return;

  const allowed = new Map();
  if (Array.isArray(context.fields)) {
    for (const field of context.fields) {
      if (field && typeof field.id === 'string' && field.id.startsWith('/')) {
        allowed.set(field.id, {
          id: field.id,
          label: typeof field.label === 'string' && field.label ? field.label : field.id,
        });
      }
    }
  }

  if (typeof context.editor !== 'string' || !context.editor) return;
  let editor;
  try {
    editor = new URL(context.editor, location.href);
    if (editor.origin !== location.origin || editor.username || editor.password) return;
  } catch {
    return;
  }

  function element(tag, attributes = {}, text = '') {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(attributes)) node.setAttribute(key, value);
    if (text) node.textContent = text;
    return node;
  }

  const host = element('div', {id: 'hb-spaces-tools'});
  // Inline host rules keep page-wide selectors from repositioning the overlay.
  for (const [property, value] of Object.entries({
    all: 'initial', position: 'fixed', inset: '0', 'z-index': '2147483000',
    'pointer-events': 'none', display: 'block', margin: '0', padding: '0',
    'font-family': 'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    'font-size': '14px', 'line-height': '1.4', 'color-scheme': 'light',
  })) host.style.setProperty(property, value, 'important');
  const shadow = host.attachShadow({mode: 'open'});
  const stylesheet = element('link', {
    rel: 'stylesheet',
    href: scriptURL ? new URL('contextual.css', scriptURL).href : `${editor.pathname}/contextual.css`,
  });
  shadow.append(stylesheet);

  const panel = element('section', {class: 'panel', 'aria-label': 'Spaces contextual editing'});
  const heading = element('div', {class: 'heading'});
  const readonly = context.write === false;
  const title = element('strong', {}, readonly ? 'Spaces · Read-only view' : 'Spaces · Edit view');
  const space = element('span', {class: 'space'}, typeof context.name === 'string' ? context.name : '');
  const exit = element('a', {class: 'exit', title: 'Exit edit view'}, 'Exit');
  function updateExitURL() {
    const url = new URL(location.href);
    url.searchParams.delete('edit');
    exit.href = url.href;
  }
  updateExitURL();
  exit.addEventListener('click', updateExitURL);
  window.addEventListener('hashchange', updateExitURL);
  window.addEventListener('popstate', updateExitURL);
  heading.append(title, space, exit);

  const controls = element('div', {class: 'controls'});
  const label = element('label', {for: 'field', class: 'sr-only'}, 'Editable page field');
  const select = element('select', {id: 'field', 'aria-describedby': 'status'});
  const locate = element('button', {type: 'button', class: 'locate'}, 'Locate');
  const edit = element('a', {class: 'edit'}, readonly ? 'View field' : 'Edit field');
  controls.append(label, select, locate, edit);
  const status = element('p', {id: 'status', class: 'status', role: 'status', 'aria-live': 'polite'});
  panel.append(heading, controls, status);

  const outline = element('div', {class: 'outline', hidden: '', 'aria-hidden': 'true'});
  const floating = element('a', {class: 'floating', hidden: '', tabindex: '-1'}, readonly ? 'View' : 'Edit');
  shadow.append(outline, floating, panel);
  document.body.append(host);

  let mapped = new Map();
  let ownership = new WeakMap();
  let active = null;
  let selectedID = '';
  let hideTimer = 0;
  let positionFrame = 0;
  let refreshFrame = 0;

  function destination(id) {
    const url = new URL(editor);
    url.searchParams.set('name', context.name);
    url.searchParams.set('field', id);
    return url.href;
  }

  function selection(id) {
    selectedID = mapped.has(id) ? id : '';
    select.value = selectedID;
    locate.disabled = !selectedID;
    edit.hidden = !selectedID;
    if (selectedID) {
      edit.href = destination(selectedID);
      edit.setAttribute('aria-label', `${readonly ? 'View' : 'Edit'} ${allowed.get(selectedID).label} in Spaces`);
    } else {
      edit.removeAttribute('href');
    }
  }

  function defaultStatus() {
    if (context.error) return String(context.error);
    if (!mapped.size) return 'No editable field markers on this page.';
    if (readonly) return 'Source writes are disabled. Choose a field to inspect it in Spaces.';
    return 'Hover over content or choose a field to edit its source.';
  }

  function clearActive() {
    active = null;
    outline.hidden = true;
    floating.hidden = true;
  }

  function position() {
    positionFrame = 0;
    if (!active?.isConnected || !ownership.has(active)) {
      clearActive();
      return;
    }
    const rect = active.getBoundingClientRect();
    const width = document.documentElement.clientWidth;
    const height = window.innerHeight;
    if (!active.getClientRects().length || rect.bottom <= 0 || rect.top >= height || rect.right <= 0 || rect.left >= width) {
      outline.hidden = true;
      floating.hidden = true;
      return;
    }
    // Clip oversized regions to the viewport without changing the page layout.
    const left = Math.max(2, rect.left - 3);
    const top = Math.max(2, rect.top - 3);
    const right = Math.min(width - 2, rect.right + 3);
    const bottom = Math.min(height - 2, rect.bottom + 3);
    Object.assign(outline.style, {left: `${left}px`, top: `${top}px`, width: `${Math.max(0, right - left)}px`, height: `${Math.max(0, bottom - top)}px`});
    outline.hidden = false;
    floating.hidden = false;
    const control = floating.getBoundingClientRect();
    const x = Math.max(6, Math.min(width - control.width - 6, right - control.width));
    const y = top > control.height + 10 ? top - control.height - 5 : Math.min(height - control.height - 6, top + 5);
    Object.assign(floating.style, {left: `${x}px`, top: `${Math.max(6, y)}px`});
  }

  function schedulePosition() {
    if (!positionFrame) positionFrame = requestAnimationFrame(position);
  }

  function activate(node, updateSelection = true) {
    const id = ownership.get(node);
    if (!id) return;
    clearTimeout(hideTimer);
    active = node;
    if (updateSelection) selection(id);
    floating.href = destination(id);
    floating.setAttribute('aria-label', `${readonly ? 'View' : 'Edit'} ${allowed.get(id).label} in Spaces`);
    schedulePosition();
  }

  function markedOwner(node) {
    if (!(node instanceof Element)) return null;
    let candidate = node.closest('[data-hb-space-field]');
    while (candidate) {
      if (ownership.has(candidate)) return candidate;
      candidate = candidate.parentElement?.closest('[data-hb-space-field]');
    }
    return null;
  }

  function refresh() {
    refreshFrame = 0;
    const next = new Map();
    ownership = new WeakMap();
    if (!context.error && typeof context.name === 'string' && context.name) {
      for (const node of document.querySelectorAll('[data-hb-space-field]')) {
        const id = node.getAttribute('data-hb-space-field');
        if (!allowed.has(id)) continue;
        if (!next.has(id)) next.set(id, []);
        next.get(id).push(node);
        ownership.set(node, id);
      }
    }
    const previousID = selectedID;
    const choicesChanged = next.size !== mapped.size || [...next].some(([id, nodes]) => nodes.length !== mapped.get(id)?.length);
    mapped = next;
    if (choicesChanged || !select.options.length) {
      const options = [...mapped].map(([id, nodes]) => {
        const label = allowed.get(id).label;
        return element('option', {value: id}, nodes.length > 1 ? `${label} (${nodes.length} places)` : label);
      });
      if (!options.length) options.push(element('option', {value: ''}, 'No mapped fields'));
      select.replaceChildren(...options);
    }
    select.disabled = !mapped.size;
    selection(mapped.has(previousID) ? previousID : mapped.keys().next().value || '');
    status.textContent = defaultStatus();
    if (active && !ownership.has(active)) clearActive();
    else if (active) schedulePosition();
  }

  function locateSelected() {
    const nodes = mapped.get(selectedID);
    if (!nodes?.length) return;
    // Open native disclosures when the mapped value lives in a closed details.
    let target = nodes.find(node => node.getClientRects().length) || nodes[0];
    for (let parent = target.parentElement; parent; parent = parent.parentElement) {
      if (parent instanceof HTMLDetailsElement) parent.open = true;
    }
    if (!target.getClientRects().length) {
      clearActive();
      status.textContent = `This field is hidden in the current page view. You can still ${readonly ? 'view' : 'edit'} its source.`;
      return;
    }
    activate(target, false);
    target.scrollIntoView({behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth', block: 'center', inline: 'nearest'});
    status.textContent = `Located ${allowed.get(selectedID).label}. Use ${readonly ? 'View' : 'Edit'} field to open its source.`;
    // Keep keyboard focus in the toolbar. The page's normal tab order is intact.
    schedulePosition();
  }

  select.addEventListener('change', () => {
    selection(select.value);
    locateSelected();
  });
  locate.addEventListener('click', locateSelected);
  document.addEventListener('pointerover', event => {
    const owner = markedOwner(event.target);
    if (owner) activate(owner);
  }, {passive: true});
  document.addEventListener('pointerout', event => {
    if (!active || !(event.target instanceof Node) || !active.contains(event.target)) return;
    const next = event.relatedTarget;
    if (next instanceof Node && (active.contains(next) || next === host)) return;
    hideTimer = window.setTimeout(() => {
      if (!shadow.activeElement && !active?.contains(document.activeElement)) clearActive();
    }, 180);
  }, {passive: true});
  document.addEventListener('focusin', event => {
    const owner = markedOwner(event.target);
    if (owner) activate(owner);
    else if (event.target !== host) clearActive();
  });
  floating.addEventListener('pointerenter', () => clearTimeout(hideTimer));
  floating.addEventListener('pointerleave', () => {
    hideTimer = window.setTimeout(() => { if (shadow.activeElement !== floating) clearActive(); }, 180);
  });
  document.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || !active) return;
    if (shadow.activeElement === floating) select.focus();
    clearActive();
  });
  window.addEventListener('scroll', schedulePosition, {passive: true, capture: true});
  window.addEventListener('resize', schedulePosition, {passive: true});
  window.visualViewport?.addEventListener('resize', schedulePosition, {passive: true});

  // HTMX/fragment replacement may add or remove mapped content after page load.
  // Only field markers and DOM structure matter; typing and style changes do not.
  const observer = new MutationObserver(records => {
    if (records.every(record => record.target === host)) return;
    if (!refreshFrame) refreshFrame = requestAnimationFrame(refresh);
  });
  observer.observe(document.body, {subtree: true, childList: true, attributes: true, attributeFilter: ['data-hb-space-field']});
  refresh();
})();
